package api

import (
	"net/http"
	"time"
)

const (
	IPNormal    int = 0
	IPForbidden int = 1
	IPConnErr   int = 2
	IPOtherErr  int = 3
)

// IP struct
type IP struct {
	Addr            string
	Type1           string
	Type2           string        `json:",omitempty"`
	Speed           time.Duration `json:",omitempty"`
	Source          string
	Url             string
	Status          int
	Retry           int64
	LastForbiddenTs int64
	Client          *http.Client
}

// NewIP .
func NewIP(source string) IP {
	//init the speed to 10 Sec
	return IP{
		Speed:  time.Second * 10,
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
