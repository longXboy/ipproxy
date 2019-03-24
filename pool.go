package ipproxy

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/longXboy/ipproxy/api"
	"github.com/longXboy/ipproxy/log"
	"github.com/longXboy/ipproxy/source"
)

type Config struct {
	//if the real speed is bigger than Config.Speed the ip will be dropped
	Speed         int64
	Sources       []func() []api.IP
	HttpCheckUrl  string
	HttpsCheckUrl string
	FilterWorker  int32
}

type Stat struct {
	Rented    int32
	Valid     int32
	Forbidden int32
	Invalid   int32
}

type Pool struct {
	c          context.Context
	cancel     context.CancelFunc
	httpCheck  string
	httpsCheck string
	speed      int64
	sources    []func() []api.IP

	filter chan api.IP
	valid  chan api.IP
	drop   chan api.IP
	setter chan api.IP
	getter chan request

	pools     map[string]api.IP
	rented    map[string]api.IP
	invalid   map[string]api.IP
	forbidden map[string]api.IP
	stat      *Stat
}

func NewPool(conf *Config) (p *Pool) {
	if conf.HttpCheckUrl == "" {
		conf.HttpCheckUrl = "http://httpbin.org/get"
	}
	if conf.HttpsCheckUrl == "" {
		conf.HttpsCheckUrl = "https://httpbin.org/get"
	}
	if conf.FilterWorker == 0 {
		conf.FilterWorker = 20
	}

	c, cancel := context.WithCancel(context.Background())
	p = &Pool{
		filter:     make(chan api.IP, 4096),
		valid:      make(chan api.IP, 1024),
		drop:       make(chan api.IP, 1024),
		setter:     make(chan api.IP, 1024),
		getter:     make(chan request, 1024),
		c:          c,
		cancel:     cancel,
		pools:      make(map[string]api.IP),
		rented:     make(map[string]api.IP),
		invalid:    make(map[string]api.IP),
		forbidden:  make(map[string]api.IP),
		sources:    conf.Sources,
		httpCheck:  conf.HttpCheckUrl,
		httpsCheck: conf.HttpsCheckUrl,
		speed:      conf.Speed,
		stat:       &Stat{},
	}
	go p.produce()
	for i := 0; i < int(conf.FilterWorker); i++ {
		go p.clean()
	}
	go p.process()
	return
}

func (p *Pool) Reload() {
	if len(p.filter) < 128 {
		p.refresh()
	} else {
		log.S.Warnf("filter is more than 128,don't reload")
	}
}
func (p *Pool) Stats() Stat {
	return Stat{
		Invalid:   atomic.LoadInt32(&p.stat.Invalid),
		Valid:     atomic.LoadInt32(&p.stat.Valid),
		Forbidden: atomic.LoadInt32(&p.stat.Forbidden),
		Rented:    atomic.LoadInt32(&p.stat.Rented),
	}
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
		//source.Feiyi,
		//source.IP66, //need to remove it
		source.KDL,
		//.PLP, //need to remove it
		source.IP89,
	}
	if p.sources != nil {
		funs = append(funs, p.sources...)
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
			ok := p.CheckIP(&ip)
			if ok && ip.Speed < p.speed {
				select {
				case p.valid <- ip:
				case <-p.c.Done():
					return
				}
			} else {
				select {
				case p.drop <- ip:
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
	ticker := time.NewTicker(time.Second * 15)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			atomic.StoreInt32(&p.stat.Valid, int32(len(p.pools)))
			atomic.StoreInt32(&p.stat.Rented, int32(len(p.rented)))
			atomic.StoreInt32(&p.stat.Invalid, int32(len(p.invalid)))
			atomic.StoreInt32(&p.stat.Forbidden, int32(len(p.forbidden)))
			log.S.Debugf("pools:%v", p.pools)
			log.S.Debugf("rented:%v", p.rented)
		case ip := <-p.setter:
			delete(p.rented, ip.Addr)
			select {
			case p.filter <- ip:
			case <-p.c.Done():
				return
			}
		case ip := <-p.valid:
			if _, ok := p.rented[ip.Addr]; !ok {
				p.pools[ip.Addr] = ip
			}
		case ip := <-p.drop:
			delete(p.pools, ip.Addr)
			if ip.Forbidden {
				p.forbidden[ip.Addr] = ip
				log.S.Warnf("ip(%v) is forbidden", ip)
			} else {
				p.invalid[ip.Addr] = ip
			}
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
