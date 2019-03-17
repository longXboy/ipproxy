package source

import (
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"

	"github.com/longXboy/ipproxy/api"

	"github.com/longXboy/ipproxy/log"
)

// IP66 get ip from 66ip.cn
func IP66() (result []api.IP) {
	var ExprIP = regexp.MustCompile(`((25[0-5]|2[0-4]\d|((1\d{2})|([1-9]?\d)))\.){3}(25[0-5]|2[0-4]\d|((1\d{2})|([1-9]?\d)))\:([0-9]+)`)

	pollURL := "http://www.66ip.cn/mo.php?tqsl=100"
	resp, err := http.Get(pollURL)
	if err != nil {
		log.S.Error(err)
		return
	}

	if resp.StatusCode != 200 {
		log.S.Error(err)
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
		log.S.Infof("[IP66] ip = %s, type = %s", ip.Addr, ip.Type1)
		result = append(result, ip)
	}

	log.S.Infof("IP66 done.")
	return
}
