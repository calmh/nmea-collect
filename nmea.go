package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"

	nmea "github.com/adrianmo/go-nmea"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	nmeaMessagesInput = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "nmea",
		Subsystem: "input",
		Name:      "messages_input_total",
	}, []string{"source"})
	nmeaMessagesBad = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "nmea",
		Subsystem: "input",
		Name:      "messages_bad_total",
	}, []string{"source"})
)

func readTCPInto(c chan<- string, addr string) {
	for {
		r, err := tcpReader(addr)
		if err != nil {
			log.Fatalln("TCP input:", err)
		}
		copyInto(c, lines(r, "tcp/"+addr))
		time.Sleep(5 * time.Second)
	}
}

func tcpReader(addr string) (io.Reader, error) {
	log.Printf("Reading NMEA from tcp/%s", addr)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("reader: %w", err)
	}
	return conn, nil
}

func readUDPInto(c chan<- string, port int) {
	for {
		r, err := udpReader(port)
		if err != nil {
			log.Println("UDP input:", err)
		}
		copyInto(c, lines(r, fmt.Sprintf("udp/%d", port)))
		time.Sleep(5 * time.Second)
	}
}

func udpReader(port int) (io.Reader, error) {
	log.Printf("Reading NMEA from udp/%d", port)
	laddr := &net.UDPAddr{Port: port}
	conn, err := net.ListenUDP("udp", laddr)
	if err != nil {
		return nil, fmt.Errorf("reader: %w", err)
	}
	return conn, nil
}

func prefix(c <-chan string, prefix string) <-chan string {
	res := make(chan string)
	go func() {
		defer close(res)
		for s := range c {
			if strings.HasPrefix(s, prefix) {
				res <- s
			}
		}
	}()
	return res
}

func lines(r io.Reader, name string) <-chan string {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 65536), 65536)

	nmeaMessagesInput.WithLabelValues(name)
	nmeaMessagesBad.WithLabelValues(name)

	res := make(chan string)
	go func() {
		defer close(res)
		for sc.Scan() {
			line := sc.Text()
			nmeaMessagesInput.WithLabelValues(name).Inc()
			if line == "" {
				nmeaMessagesBad.WithLabelValues(name).Inc()
				continue
			}
			switch line[0] {
			case '!', '$':
				idx := strings.LastIndexByte(line, '*')
				if idx == -1 {
					nmeaMessagesBad.WithLabelValues(name).Inc()
					continue
				}
				chk := nmea.Checksum(line[1:idx])
				if chk != line[idx+1:] {
					nmeaMessagesBad.WithLabelValues(name).Inc()
					continue
				}
				res <- line
			default:
				nmeaMessagesBad.WithLabelValues(name).Inc()
			}
		}
	}()
	return res
}

func copyInto(dst chan<- string, src <-chan string) {
	for s := range src {
		dst <- s
	}
}

func tee(c <-chan string) (chan string, chan string) {
	a := make(chan string)
	b := make(chan string)
	go func() {
		defer close(a)
		defer close(b)
		for s := range c {
			a <- s
			b <- s
		}
	}()
	return a, b
}
