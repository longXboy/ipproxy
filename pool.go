package ipproxy

import (
	"context"
	"sync"
	"time"

	"github.com/longXboy/ipproxy/api"
	"github.com/longXboy/ipproxy/log"
	"github.com/longXboy/ipproxy/source"
)

type Pool struct {
	filter  chan api.IP
	valid   chan api.IP
	invalid chan api.IP
	setter  chan api.IP
	getter  chan request

	pools  map[string]api.IP
	rented map[string]api.IP

	c      context.Context
	cancel context.CancelFunc
}

func NewPool() (p *Pool) {
	c, cancel := context.WithCancel(context.Background())
	p = &Pool{
		filter:  make(chan api.IP, 4096),
		valid:   make(chan api.IP, 1024),
		invalid: make(chan api.IP, 1024),
		setter:  make(chan api.IP, 1024),
		getter:  make(chan request, 1024),
		c:       c,
		cancel:  cancel,
		pools:   make(map[string]api.IP),
		rented:  make(map[string]api.IP),
	}
	go p.produce()
	for i := 0; i < 20; i++ {
		go p.clean()
	}
	go p.process()
	return
}

func (p *Pool) produce() {
	p.refresh()
	ticker := time.NewTicker(time.Minute * 10)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if len(p.filter) < 128 {
				p.refresh()
			}
		case <-p.c.Done():
			return
		}
	}
}

func (p *Pool) refresh() {
	var wg sync.WaitGroup
	funs := []func() []api.IP{
		source.Feiyi,
		source.IP66, //need to remove it
		source.KDL,
		source.PLP, //need to remove it
		source.IP89,
	}
	for _, f := range funs {
		wg.Add(1)
		go func(f func() []api.IP) {
			temp := f()
			for _, v := range temp {
				select {
				case p.filter <- v:
				case <-p.c.Done():
					return
				}
			}
			wg.Done()
		}(f)
	}
	wg.Wait()
	log.S.Infof("All getters finished.")
}

func (p *Pool) clean() {
	for {
		select {
		case ip := <-p.filter:
			speed, ok := CheckIP(ip)
			if ok && speed < 2500 {
				ip.Speed = speed
				select {
				case p.valid <- ip:
				case <-p.c.Done():
					return
				}
			} else {
				select {
				case p.invalid <- ip:
				case <-p.c.Done():
					return
				}
			}
		case <-p.c.Done():
			return
		}

	}
}

func (p *Pool) process() {
	ticker := time.NewTicker(time.Second * 30)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			log.S.Infof("pools:%v", p.pools)
			log.S.Infof("rented:%v", p.rented)
		case ip := <-p.setter:
			delete(p.rented, ip.Addr)
			p.pools[ip.Addr] = ip
		case ip := <-p.valid:
			if _, ok := p.rented[ip.Addr]; !ok {
				p.pools[ip.Addr] = ip
			}
		case ip := <-p.invalid:
			delete(p.pools, ip.Addr)
		case r := <-p.getter:
			var n int
			var ips []api.IP
			for _, ip := range p.pools {
				if n >= r.n {
					break
				}
				ips = append(ips, ip)
				delete(p.pools, ip.Addr)
				p.rented[ip.Addr] = ip
				n++
			}
			*r.ip = ips
			select {
			case r.ok <- struct{}{}:
			case <-p.c.Done():
				return
			}
		case <-p.c.Done():
			return
		}
	}
}

type request struct {
	ip *[]api.IP
	n  int
	ok chan struct{}
}

func (p *Pool) Get(c context.Context, n int) (ips []api.IP) {
	r := request{
		ip: &[]api.IP{},
		n:  n,
		ok: make(chan struct{}),
	}
	select {
	case p.getter <- r:
	case <-c.Done():
		return
	}
	select {
	case <-r.ok:
		ips = *r.ip
		return
	case <-p.c.Done():
		return
	case <-c.Done():
		return
	}
}

func (p *Pool) Put(ips []api.IP) {
	go func() {
		for _, ip := range ips {
			select {
			case p.setter <- ip:
			case <-p.c.Done():
				return
			}
		}
	}()
}

func (p *Pool) Close() {
	p.cancel()
}
