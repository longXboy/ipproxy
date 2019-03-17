package source

import (
	"github.com/longXboy/ipproxy/log"

	"regexp"
	"strconv"

	"github.com/Aiicy/htmlquery"
	"github.com/longXboy/ipproxy/api"
)

//feiyi get ip from feiyiproxy.com
func Feiyi() (result []api.IP) {
	log.S.Infof("FEIYI] start test")
	pollURL := "http://www.feiyiproxy.com/?page_id=1457"
	doc, _ := htmlquery.LoadURL(pollURL)
	trNode, err := htmlquery.Find(doc, "//div[@class='et_pb_code.et_pb_module.et_pb_code_1']//div//table//tbody//tr")
	log.S.Infof("[FEIYI] start up")
	if err != nil {
		log.S.Infof("FEIYI] parse pollUrl error")
		log.S.Warnf(err.Error())
	}
	//debug begin
	log.S.Infof("[FEIYI] len(trNode) = %d ", len(trNode))
	for i := 1; i < len(trNode); i++ {
		tdNode, _ := htmlquery.Find(trNode[i], "//td")
		ip := htmlquery.InnerText(tdNode[0])
		port := htmlquery.InnerText(tdNode[1])
		Type := htmlquery.InnerText(tdNode[3])
		speed := htmlquery.InnerText(tdNode[6])

		IP := api.NewIP()
		IP.Addr = ip + ":" + port

		if Type == "HTTPS" {
			IP.Type1 = "https"
			IP.Type2 = ""

		} else if Type == "HTTP" {
			IP.Type1 = "http"
		}
		IP.Speed = extractSpeed(speed)

		log.S.Infof("[FEIYI] ip.Addr = %s,ip.Type = %s,%s ip.Speed = %d", IP.Addr, IP.Type1, IP.Type2, IP.Speed)

		result = append(result, IP)
	}

	log.S.Infof("FEIYI done.")
	return
}

func extractSpeed(oritext string) int64 {
	reg := regexp.MustCompile(`\[1-9\]\d\*\\.\?\d\*`)
	temp := reg.FindString(oritext)
	if temp != "" {
		speed, _ := strconv.ParseInt(temp, 10, 64)
		return speed
	}
	return -1
}
