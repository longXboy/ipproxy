package ipproxy

import (
	"container/list"
	"context"
	"sync"
	"time"

	"github.com/longXboy/ipproxy/api"
	"github.com/longXboy/ipproxy/log"
)

type Config struct {
	//if the real speed is bigger than Config.Speed the ip will be dropped
	Speed         time.Duration
	Sources       []func() []api.IP
	HttpCheckUrl  string
	HttpsCheckUrl string
	FilterWorker  int32
	//the interval duration of checking forbidden ip
	CheckInterval time.Duration
}

type Stat struct {
	All       int32
	Rented    int32
	Available int32
	Forbidden int32
	Bad       int32
	Filter    int32
}

type Option struct {
	reserved int
}

func ReserveOpt(count int) func(*Option) {
	return func(o *Option) {
		o.reserved = count
	}
}

type Pool struct {
	conf   *Config
	c      context.Context
	cancel context.CancelFunc

	filter chan *api.IP
	input  chan *api.IP
	valid  chan *api.IP
	drop   chan *api.IP
	putter chan *api.IP
	getter chan *request

	all       map[string]struct{}
	pools     *list.List
	rented    map[string]*api.IP
	bad       *list.List
	forbidden *list.List
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
	if conf.Speed == 0 {
		conf.Speed = time.Second * 4
	}
	if conf.CheckInterval == 0 {
		conf.CheckInterval = time.Second * 65
	}
	c, cancel := context.WithCancel(context.Background())
	p = &Pool{
		conf:      conf,
		input:     make(chan *api.IP, 1024),
		filter:    make(chan *api.IP, 2048),
		putter:    make(chan *api.IP, 1024),
		getter:    make(chan *request, 1024),
		c:         c,
		cancel:    cancel,
		all:       make(map[string]struct{}),
		pools:     list.New(),
		rented:    make(map[string]*api.IP),
		bad:       list.New(),
		forbidden: list.New(),
	}
	go p.produce()
	for i := 0; i < int(conf.FilterWorker); i++ {
		go p.clean()
	}
	go p.process()
	return
}

func (p *Pool) Stats() Stat {
	return Stat{
		All:       int32(len(p.all)),
		Bad:       int32(p.bad.Len()),
		Available: int32(p.pools.Len()),
		Forbidden: int32(p.forbidden.Len()),
		Rented:    int32(len(p.rented)),
		Filter:    int32(len(p.filter)),
	}
}

func (p *Pool) produce() {
	p.Refresh()
	ticker := time.NewTicker(time.Minute * 15)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if len(p.filter) < 256 && len(p.input) < 256 {
				p.Refresh()
			}
		case <-p.c.Done():
			return
		}
	}
}

func (p *Pool) Refresh() {
	var wg sync.WaitGroup
	funs := []func() []api.IP{
		//source.Feiyi,
		//source.IP66, //need to remove it
		//source.KDL,
		//.PLP, //need to remove it
		//source.IP89,
	}
	if p.conf.Sources != nil {
		funs = append(funs, p.conf.Sources...)
	}
	for _, f := range funs {
		wg.Add(1)
		go func(f func() []api.IP) {
			temp := f()
			for i := range temp {
				select {
				case p.input <- &temp[i]:
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
			p.CheckIP(ip)
			select {
			case p.putter <- ip:
			case <-p.c.Done():
				return
			}
		case <-p.c.Done():
			return
		}
	}
}

func (p *Pool) process() {
	checkInter := time.NewTicker(p.conf.CheckInterval)
	defer checkInter.Stop()
	ticker := time.NewTicker(time.Second * 60)
	defer ticker.Stop()
	for {
		select {
		case <-checkInter.C:
			now := p.forbidden.Front()
			for {
				if now == nil {
					break
				}
				ip := now.Value.(*api.IP)
				next := now.Next()
				if ip.LastForbiddenTs == 0 || time.Duration(time.Now().UnixNano()-ip.LastForbiddenTs) > p.conf.CheckInterval {
					p.forbidden.Remove(now)
					select {
					case p.filter <- ip:
					case <-p.c.Done():
						return
					}
				}
				now = next
			}
		case <-ticker.C:
			for i := 0; i < 20; i++ {
				front := p.bad.Front()
				if front == nil {
					break
				}
				p.bad.Remove(front)
				select {
				case p.filter <- front.Value.(*api.IP):
				case <-p.c.Done():
					return
				}
			}
		case ip := <-p.putter:
			delete(p.rented, ip.Addr)
			if ip.Status == api.IPNormal && ip.Speed <= p.conf.Speed {
				p.pools.PushBack(ip)
			} else if ip.Status == api.IPForbidden {
				p.forbidden.PushBack(ip)
			} else if ip.Status == api.IPConnErr && ip.Retry > 100 {
				//drop it
				delete(p.all, ip.Addr)
			} else {
				p.bad.PushBack(ip)
			}
		case r := <-p.getter:
			if r.opt.reserved == 0 || p.pools.Len() > r.opt.reserved {
				front := p.pools.Front()
				if front != nil {
					p.pools.Remove(front)
					ip := front.Value.(*api.IP)
					p.rented[ip.Addr] = ip
					r.ip = ip
				}
			}
			select {
			case r.done <- struct{}{}:
			case <-p.c.Done():
				return
			}
		case ip := <-p.input:
			if _, ok := p.all[ip.Addr]; !ok {
				p.all[ip.Addr] = struct{}{}
				select {
				case p.filter <- ip:
				case <-p.c.Done():
					return
				}
			}
		case <-p.c.Done():
			return
		}
	}
}

type request struct {
	ip   *api.IP
	done chan struct{}
	opt  *Option
}

func (p *Pool) Get(opts ...func(o *Option)) (ip *api.IP) {
	r := &request{
		done: make(chan struct{}),
		opt:  new(Option),
	}
	for _, f := range opts {
		f(r.opt)
	}
	p.getter <- r
	select {
	case <-r.done:
		ip = r.ip
	case <-p.c.Done():
	}
	return
}

func (p *Pool) Put(ip *api.IP) {
	go func() {
		select {
		case p.putter <- ip:
		case <-p.c.Done():
			return
		}
	}()
}

func (p *Pool) Close() {
	p.cancel()
}
