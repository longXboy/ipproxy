package source

import (
	"github.com/longXboy/ipproxy/log"

	"github.com/Aiicy/htmlquery"
	"github.com/longXboy/ipproxy/api"
)

//PLP get ip from proxylistplus.com
func PLP() (result []api.IP) {
	pollURL := "https://list.proxylistplus.com/Fresh-HTTP-Proxy-List-1"
	doc, err := htmlquery.LoadURL(pollURL)
	if err != nil {
		log.S.Errorf(err.Error())
		return
	}
	trNode, err := htmlquery.Find(doc, "//div[@class='hfeed site']//table[@class='bg']//tbody//tr")
	if err != nil {
		log.S.Warnf(err.Error())
	}
	for i := 3; i < len(trNode); i++ {
		tdNode, _ := htmlquery.Find(trNode[i], "//td")
		ip := htmlquery.InnerText(tdNode[1])
		port := htmlquery.InnerText(tdNode[2])
		Type := htmlquery.InnerText(tdNode[6])

		IP := api.NewIP("plp")
		IP.Addr = ip + ":" + port

		if Type == "yes" {
			IP.Type1 = "http"
			IP.Type2 = "https"

		} else if Type == "no" {
			IP.Type1 = "http"
		}

		result = append(result, IP)
	}

	log.S.Infof("PLP done.")
	return
}
