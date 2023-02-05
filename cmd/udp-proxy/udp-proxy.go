package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/thejerf/suture/v4"
)

const (
	readTimeout  = 1 * time.Minute
	writeTimeout = 1 * time.Second
)

func main() {
	flag.Parse()
	if flag.NArg() == 0 {
		log.Fatal("No forwarding specs given")
	}

	main := suture.NewSimple("main")
	for _, f := range flag.Args() {
		portStr, dst, ok := strings.Cut(f, ":")
		if !ok {
			log.Fatal("Invalid forwarding spec:", f)
		}
		port, err := strconv.Atoi(portStr)
		if err != nil {
			log.Fatal("Invalid port:", portStr)
		}
		forward := &forwarder{
			srcPort: port,
			dstAddr: dst,
		}
		main.Add(forward)
	}

	if err := main.Serve(context.Background()); err != nil {
		log.Fatal(err)
	}
}

type forwarder struct {
	srcPort int
	dstAddr string
}

func (f *forwarder) Serve(ctx context.Context) error {
	sourceAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", f.srcPort))
	if err != nil {
		return fmt.Errorf("resolve source address %s: %w", fmt.Sprintf(":%d", f.srcPort), err)
	}

	destAddr, err := net.ResolveUDPAddr("udp", f.dstAddr)
	if err != nil {
		return fmt.Errorf("resolve destination address %s: %w", f.dstAddr, err)
	}

	sourceConn, err := net.ListenUDP("udp", sourceAddr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", sourceAddr, err)
	}
	defer sourceConn.Close()

	destConn, err := net.DialUDP("udp", nil, destAddr)
	if err != nil {
		return fmt.Errorf("dial destination address %s: %w", destAddr, err)
	}
	defer destConn.Close()

	log.Printf("Proxying UDP %s -> %s", sourceAddr, destAddr)
	b := make([]byte, 65536)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		sourceConn.SetReadDeadline(time.Now().Add(readTimeout))
		n, _, err := sourceConn.ReadFromUDP(b)
		if err != nil {
			return fmt.Errorf("receive packet: %w", err)
		}
		destConn.SetWriteDeadline(time.Now().Add(writeTimeout))
		if _, err := destConn.Write(b[0:n]); err != nil {
			return fmt.Errorf("forward packet: %w", err)
		}
	}
}
