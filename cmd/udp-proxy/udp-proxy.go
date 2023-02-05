package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/thejerf/suture/v4"
)

const (
	readTimeout  = 1 * time.Minute
	writeTimeout = 1 * time.Second
	httpTimeout  = 10 * time.Second
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
		if strings.HasPrefix(dst, "http://") || strings.HasPrefix(dst, "https://") {
			main.Add(&httpForwarder{
				srcPort: port,
				dstAddr: dst,
			})
		} else {
			main.Add(&udpForwarder{
				srcPort: port,
				dstAddr: dst,
			})
		}
	}

	if err := main.Serve(context.Background()); err != nil {
		log.Fatal(err)
	}
}

type udpForwarder struct {
	srcPort int
	dstAddr string
}

func (f *udpForwarder) Serve(ctx context.Context) error {
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

type httpForwarder struct {
	srcPort int
	dstAddr string
}

func (f *httpForwarder) Serve(ctx context.Context) error {
	sourceAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", f.srcPort))
	if err != nil {
		return fmt.Errorf("resolve source address %s: %w", fmt.Sprintf(":%d", f.srcPort), err)
	}

	sourceConn, err := net.ListenUDP("udp", sourceAddr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", sourceAddr, err)
	}
	defer sourceConn.Close()

	log.Printf("Proxying UDP %s -> %s", sourceAddr, f.dstAddr)
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

		ctx, cancel := context.WithDeadline(ctx, time.Now().Add(httpTimeout))
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.dstAddr, bytes.NewReader(b[0:n]))
		if err != nil {
			return fmt.Errorf("create HTTP request: %w", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("forward packet: %w", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("forward packet: %s", resp.Status)
		}
	}
}
