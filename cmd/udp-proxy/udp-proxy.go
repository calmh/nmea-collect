package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
)

func main() {
	flag.Parse()
	if flag.NArg() == 0 {
		log.Fatal("No forwarding specs given")
	}
	for _, f := range flag.Args() {
		portStr, dst, ok := strings.Cut(f, ":")
		if !ok {
			log.Fatal("Invalid forwarding spec:", f)
		}
		port, err := strconv.Atoi(portStr)
		if err != nil {
			log.Fatal("Invalid port:", portStr)
		}
		if err := forward(port, dst); err != nil {
			log.Fatal(err)
		}
	}
	select {}
}

func forward(srcPort int, dstAddr string) error {
	sourceAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", srcPort))
	if err != nil {
		return fmt.Errorf("could not resolve source address %s: %w", fmt.Sprintf(":%d", srcPort), err)
	}

	destAddr, err := net.ResolveUDPAddr("udp", dstAddr)
	if err != nil {
		return fmt.Errorf("could not resolve destination address %s: %w", dstAddr, err)
	}

	sourceConn, err := net.ListenUDP("udp", sourceAddr)
	if err != nil {
		return fmt.Errorf("could not listen on %s: %w", sourceAddr, err)
	}

	destConn, err := net.DialUDP("udp", nil, destAddr)
	if err != nil {
		return fmt.Errorf("could not dial destination address %s: %w", destAddr, err)
	}

	go func() {
		log.Printf("Starting udp-proxy %s -> %s", sourceAddr, destAddr)
		b := make([]byte, 65536)
		for {
			n, _, err := sourceConn.ReadFromUDP(b)
			if err != nil {
				log.Println("Could not receive packet:", err)
				continue
			}
			if _, err := destConn.Write(b[0:n]); err != nil {
				log.Println("Could not forward packet:", err)
			}
		}
	}()
	return nil
}
