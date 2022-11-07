package main

import (
	"log"
	"net"

	"github.com/alecthomas/kong"
)

var opts struct {
	Source string   `long:"source" default:":1457" description:"Source port to listen on"`
	Target []string `long:"target" description:"Target address to forward to"`
	Quiet  bool     `long:"quiet" description:"whether to print logging info or not"`
}

func main() {
	kong.Parse(&opts)
	sourceAddr, err := net.ResolveUDPAddr("udp", opts.Source)
	if err != nil {
		log.Fatalln("Could not resolve source address:", opts.Source)
		return
	}

	var targetAddr []*net.UDPAddr
	for _, v := range opts.Target {
		addr, err := net.ResolveUDPAddr("udp", v)
		if err != nil {
			log.Fatalln("Could not resolve target address:", v)
			return
		}
		targetAddr = append(targetAddr, addr)
	}

	sourceConn, err := net.ListenUDP("udp", sourceAddr)
	if err != nil {
		log.Fatalln("Could not listen on address:", opts.Source)
		return
	}

	defer sourceConn.Close()

	var targetConn []*net.UDPConn
	for _, v := range targetAddr {
		conn, err := net.DialUDP("udp", nil, v)
		if err != nil {
			log.Fatalln("Could not connect to target address:", v)
			return
		}

		defer conn.Close()
		targetConn = append(targetConn, conn)
	}

	log.Printf(">> Starting udpproxy, Source at %v, Target at %v...", opts.Source, opts.Target)

	for {
		b := make([]byte, 65536)
		n, _, err := sourceConn.ReadFromUDP(b)
		if err != nil {
			log.Println("Could not receive packet:", err)
			continue
		}

		for _, v := range targetConn {
			if _, err := v.Write(b[0:n]); err != nil {
				log.Println("Could not forward packet:", err)
			}
		}
	}
}
