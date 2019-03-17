package main

import (
	"context"
	"fmt"
	"time"

	"github.com/longXboy/ipproxy"
	"github.com/longXboy/ipproxy/log"
)

func main() {
	log.Init()
	defer log.Close()
	ip := ipproxy.NewPool()
	time.Sleep(time.Second * 30)
	ips := ip.Get(context.Background(), 10)
	fmt.Println("ips:", ips)
	time.Sleep(time.Hour)
}
