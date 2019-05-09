package ipproxy

import (
	"container/list"
	"context"
	"sync"
	"sync/atomic"
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
	Valid     int32
	Forbidden int32
	Bad       int32
	Filter    int32
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
		stat:      &Stat{},
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
		All:       atomic.LoadInt32(&p.stat.All),
		Bad:       atomic.LoadInt32(&p.stat.Bad),
		Valid:     atomic.LoadInt32(&p.stat.Valid),
		Forbidden: atomic.LoadInt32(&p.stat.Forbidden),
		Rented:    atomic.LoadInt32(&p.stat.Rented),
		Filter:    atomic.LoadInt32(&p.stat.Filter),
	}
}

func (p *Pool) produce() {
	p.Refresh()
	ticker := time.NewTicker(time.Minute * 5)
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
	ticker := time.NewTicker(time.Second * 15)
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
			atomic.StoreInt32(&p.stat.All, int32(len(p.all)))
			atomic.StoreInt32(&p.stat.Valid, int32(p.pools.Len()))
			atomic.StoreInt32(&p.stat.Rented, int32(len(p.rented)))
			atomic.StoreInt32(&p.stat.Bad, int32(p.bad.Len()))
			atomic.StoreInt32(&p.stat.Forbidden, int32(p.forbidden.Len()))
			atomic.StoreInt32(&p.stat.Filter, int32(len(p.filter)))
			log.S.Debugf("all:%d pools:%d forbidden:%d bad:%d rented:%d", len(p.all), p.pools.Len(), p.forbidden.Len(), p.bad.Len(), len(p.rented))
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
			front := p.pools.Front()
			if front != nil {
				p.pools.Remove(front)
				ip := front.Value.(*api.IP)
				p.rented[ip.Addr] = ip
				r.ip = ip
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
}

func (p *Pool) Get() (ip *api.IP) {
	r := &request{
		done: make(chan struct{}),
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
