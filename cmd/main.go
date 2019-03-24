package main

import (
	"context"
	"fmt"

	"time"

	"github.com/longXboy/ipproxy"
	"github.com/longXboy/ipproxy/log"
)

func main() {

	defer log.Close()
	ip := ipproxy.NewPool(&ipproxy.Config{})
	time.Sleep(time.Second * 50)
	ips := ip.Get(context.Background(), 100)
	fmt.Println("ips:", ips)
	time.Sleep(time.Hour)
}
