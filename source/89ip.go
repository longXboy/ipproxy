package source

import (
	"io/ioutil"
	"net/http"
	//"fmt"
	"github.com/longXboy/ipproxy/log"

	"regexp"
	"strings"

	"github.com/longXboy/ipproxy/api"
)

//IP89 get ip from www.89ip.cn
func IP89() (result []api.IP) {
	log.S.Infof("89IP] start test")
	var ExprIP = regexp.MustCompile(`((25[0-5]|2[0-4]\d|((1\d{2})|([1-9]?\d)))\.){3}(25[0-5]|2[0-4]\d|((1\d{2})|([1-9]?\d)))\:([0-9]+)`)
	pollURL := "http://www.89ip.cn/tqdl.html?api=1&num=100&port=&address=%E7%BE%8E%E5%9B%BD&isp="

	resp, err := http.Get(pollURL)
	if err != nil {
		log.S.Warnf(err.Error())
		return
	}

	if resp.StatusCode != 200 {
		log.S.Warnf(err.Error())
		return
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	bodyIPs := string(body)
	ips := ExprIP.FindAllString(bodyIPs, 100)

	for index := 0; index < len(ips); index++ {
		ip := api.NewIP()
		ip.Addr = strings.TrimSpace(ips[index])
		ip.Type1 = "http"
		log.S.Infof("[89IP] ip = %s, type = %s", ip.Addr, ip.Type1)
		result = append(result, ip)
	}

	log.S.Infof("89IP done.")
	return
}
