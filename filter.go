package ipproxy

import (
	"time"

	"github.com/longXboy/ipproxy/api"

	sj "github.com/bitly/go-simplejson"
	"github.com/longXboy/ipproxy/log"
	"github.com/parnurzeal/gorequest"
)

// CheckIP is to check the ip work or not
func (p *Pool) CheckIP(ip api.IP) (speed int64, ok bool) {
	var pollURL string
	var testIP string
	if ip.Type2 == "https" {
		testIP = "https://" + ip.Addr
		pollURL = p.httpsCheck
	} else {
		testIP = "http://" + ip.Addr
		pollURL = p.httpCheck
	}
	//log.S.Info(testIP)
	begin := time.Now()
	resp, _, errs := gorequest.New().Proxy(testIP).Get(pollURL).End()
	if errs != nil {
		//log.S.Warnf("[CheckIP] testIP = %s, pollURL = %s: Error = %v", testIP, pollURL, errs)
		return
	}

	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		//harrybi 20180815 判断返回的数据格式合法性
		_, err := sj.NewFromReader(resp.Body)
		if err != nil {
			log.S.Warnf("[CheckIP] testIP = %s, pollURL = %s: Error = %v", testIP, pollURL, err)
			return
		}
		//harrybi 计算该代理的速度，单位毫秒
		speed = time.Now().Sub(begin).Nanoseconds() / int64(time.Millisecond)
		ok = true
		return
	}
	return
}
