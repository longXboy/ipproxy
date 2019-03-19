package source

import (
	"github.com/Aiicy/htmlquery"
	"github.com/longXboy/ipproxy/api"
	"github.com/longXboy/ipproxy/log"
)

// KDL get ip from kuaidaili.com
func KDL() (result []api.IP) {
	pollURL := "http://www.kuaidaili.com/free/inha/"
	doc, _ := htmlquery.LoadURL(pollURL)
	trNode, err := htmlquery.Find(doc, "//table[@class='table.table-bordered.table-striped']//tbody//tr")
	if err != nil {
		log.S.Warnf(err.Error())
	}
	for i := 0; i < len(trNode); i++ {
		tdNode, _ := htmlquery.Find(trNode[i], "//td")
		ip := htmlquery.InnerText(tdNode[0])
		port := htmlquery.InnerText(tdNode[1])
		Type := htmlquery.InnerText(tdNode[3])
		speed := htmlquery.InnerText(tdNode[5])

		IP := api.NewIP("kdl")
		IP.Addr = ip + ":" + port
		if Type == "HTTPS" {
			IP.Type1 = "http"
			IP.Type2 = "https"
		} else if Type == "HTTP" {
			IP.Type1 = "http"
		}
		IP.Speed = extractSpeed(speed)
		result = append(result, IP)
	}

	log.S.Infof("[kuaidaili] done")
	return
}
