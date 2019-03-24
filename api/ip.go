package api

// IP struct
type IP struct {
	Addr      string
	Type1     string
	Type2     string `json:",omitempty"`
	Speed     int64  `json:",omitempty"`
	Source    string
	Url       string
	Forbidden bool `json:",omitempty"`
}

// NewIP .
func NewIP(source string) IP {
	//init the speed to 100 Sec
	return IP{
		Speed:  100,
		Source: source,
	}
}

func (ip *IP) GetUrl() string {
	if ip.Url != "" {
		return ip.Url
	} else if ip.Type2 == "https" {
		return "https://" + ip.Addr
	} else {
		return "http://" + ip.Addr
	}
}
