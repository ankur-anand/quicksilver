package main

import (
	"net"
	"fmt"
	"os"
)

func main() {
	result := make(map[string]int)
	for i:=0; i<10000; i++ {
		ips, err := net.LookupIP("ce-gateway.ce-orchestration.k8s.prod.walmart.com")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not get IPs: %v\n", err)
			os.Exit(1)
		}
		for _, ip := range ips {
			//fmt.Printf("ce-gateway.ce-orchestration.k8s.prod.walmart.com. IN A %s\n", ip.String())
			result[ip.String()]++
		}
	}
	fmt.Println(result)
}


//package main
//
//import (
//	"context"
//	"net"
//	"time"
//)

//func main() {
//	r := &net.Resolver{
//		PreferGo: true,
//		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
//			d := net.Dialer{
//				Timeout: time.Millisecond * time.Duration(10000),
//				}
//				return d.DialContext(ctx, network, "8.8.8.8:53")
//		},
//		}
//		ip, _ := r.LookupHost(context.Background(), "www.google.com")
//
//		print(ip[0])
//}