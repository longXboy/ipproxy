package ipproxy

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/longXboy/ipproxy/api"

	"github.com/longXboy/ipproxy/log"
)

// CheckIP is to check the ip work or not
func (p *Pool) CheckIP(ip *api.IP) {
	var pollURL string
	var testIP string
	if ip.Url != "" {
		testIP = ip.Url
		pollURL = p.conf.HttpsCheckUrl
	} else if ip.Type2 == "https" {
		testIP = "https://" + ip.Addr
		pollURL = p.conf.HttpsCheckUrl
	} else {
		testIP = "http://" + ip.Addr
		pollURL = p.conf.HttpCheckUrl
	}
	//log.S.Info(testIP)
	begin := time.Now()
	cli := http.Client{
		Timeout: time.Second * 6,
	}
	proxyUrl, err := url.Parse(testIP)
	if err != nil {
		log.S.Warnf("url.Parse testIP = %s: Error = %v", testIP, err)
	}
	cli.Transport = &http.Transport{Proxy: http.ProxyURL(proxyUrl)}
	resp, err := cli.Get(pollURL)
	if err != nil {
		ip.Status = api.IPConnErr
		ip.Retry++
		return
	}
	defer resp.Body.Close()

	// still alive
	ip.Retry = 0
	if resp.StatusCode == 200 {
		_, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.S.Warnf("[CheckIP] testIP = %s, pollURL = %s: Error = %v", testIP, pollURL, err)
			return
		}
		//harrybi 计算该代理的速度，单位毫秒
		ip.Speed = time.Now().Sub(begin)
		ip.Status = api.IPNormal
		ip.LastForbiddenTs = 0
	} else if resp.StatusCode == http.StatusForbidden {
		ip.Status = api.IPForbidden
		ip.LastForbiddenTs = time.Now().UnixNano()
	} else {
		ip.LastForbiddenTs = 0
		ip.Status = api.IPOtherErr
	}
	return
}
