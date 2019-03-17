package api

// IP struct
type IP struct {
	Addr  string
	Type1 string
	Type2 string `json:",omitempty"`
	Speed int64  `json:",omitempty"`
}

// NewIP .
func NewIP() IP {
	//init the speed to 100 Sec
	return IP{Speed: 100}
}
