package ipproxy

import (
	"net/http"
	"net/url"
	"time"

	"github.com/longXboy/ipproxy/api"

	sj "github.com/bitly/go-simplejson"
	"github.com/longXboy/ipproxy/log"
)

// CheckIP is to check the ip work or not
func (p *Pool) CheckIP(ip *api.IP) (ok bool) {
	var pollURL string
	var testIP string
	if ip.Url != "" {
		testIP = ip.Url
		pollURL = p.httpsCheck
	} else if ip.Type2 == "https" {
		testIP = "https://" + ip.Addr
		pollURL = p.httpsCheck
	} else {
		testIP = "http://" + ip.Addr
		pollURL = p.httpCheck
	}
	//log.S.Info(testIP)
	begin := time.Now()
	cli := http.Client{
		Timeout: time.Second * 2,
	}
	proxyUrl, err := url.Parse(testIP)
	if err != nil {
		log.S.Warnf("url.Parse testIP = %s: Error = %v", testIP, err)
	}
	cli.Transport = &http.Transport{Proxy: http.ProxyURL(proxyUrl)}
	resp, err := cli.Get(pollURL)
	if err != nil {
		/*	if ip.Source == "local" {
			log.S.Warnf("CheckIP testIP(%s) pollUrl(%s) failed;Error=%v", testIP, pollURL, err)
		}*/
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
		ip.Speed = time.Now().Sub(begin).Nanoseconds() / int64(time.Millisecond)
		ok = true
		return
	} else if resp.StatusCode == http.StatusForbidden {
		ip.Forbidden = true
	}
	return
}
