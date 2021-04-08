package main

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	tcpIncomingConnections = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "nmea",
		Subsystem: "tcp",
		Name:      "incoming_connections_total",
	}, []string{"source"})
	tcpForwardedMessages = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "nmea",
		Subsystem: "tcp",
		Name:      "forwarded_messages_total",
	}, []string{"source"})
	tcpCurrentConnections = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "nmea",
		Subsystem: "tcp",
		Name:      "current_connections",
	}, []string{"source"})
)

type tcpForwarder struct {
	input <-chan string
	addr  string
	conns []net.Conn
	mut   sync.Mutex
}

func forwardTCP(input <-chan string, addr string) {
	f := &tcpForwarder{
		input: input,
		addr:  addr,
	}
	go f.listen(addr)
	f.run()
}

func (f *tcpForwarder) run() {
	tcpForwardedMessages.WithLabelValues(f.addr)
	tcpCurrentConnections.WithLabelValues(f.addr)

	for line := range f.input {
		f.mut.Lock()
		for i := 0; i < len(f.conns); i++ {
			f.conns[i].SetWriteDeadline(time.Now().Add(time.Second))
			if _, err := fmt.Fprintf(f.conns[i], "%s\n", line); err != nil {
				f.conns[i].Close()
				f.conns = append(f.conns[:i], f.conns[i+1:]...)
				i--
			}
			tcpForwardedMessages.WithLabelValues(f.addr).Inc()
		}
		tcpCurrentConnections.WithLabelValues(f.addr).Set(float64(len(f.conns)))
		f.mut.Unlock()
	}
}

func (f *tcpForwarder) listen(addr string) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Println("Listen:", err)
		return
	}
	defer l.Close()
	log.Println("Listening on", l.Addr())

	tcpIncomingConnections.WithLabelValues(f.addr)

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Println("Accept:", err)
			return
		}

		f.mut.Lock()
		f.conns = append(f.conns, conn)
		f.mut.Unlock()

		tcpIncomingConnections.WithLabelValues(f.addr).Inc()
	}
}
