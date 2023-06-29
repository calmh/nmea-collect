package serve

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/thejerf/suture/v4"
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
	suture.Service
}

func forwardTCP(input <-chan string, addr string) suture.Service {
	sup := suture.NewSimple("tcp-forwarder-supervisor/" + addr)
	f := &tcpForwarder{
		input: input,
		addr:  addr,
	}
	sup.Add(f)
	l := &tcpListener{
		addr:      addr,
		forwarder: f,
	}
	sup.Add(l)
	return sup
}

func (f *tcpForwarder) String() string {
	return fmt.Sprintf("tcp-forwarder(%s)@%p", f.addr, f)
}

func (f *tcpForwarder) addConn(conn net.Conn) {
	f.mut.Lock()
	f.conns = append(f.conns, conn)
	f.mut.Unlock()
}

func (f *tcpForwarder) Serve(ctx context.Context) error {
	tcpForwardedMessages.WithLabelValues(f.addr)
	tcpCurrentConnections.WithLabelValues(f.addr)

	for {
		select {
		case line := <-f.input:
			f.mut.Lock()
			for i := 0; i < len(f.conns); i++ {
				_ = f.conns[i].SetWriteDeadline(time.Now().Add(time.Second))
				if _, err := fmt.Fprintf(f.conns[i], "%s\n", line); err != nil {
					_ = f.conns[i].Close()
					f.conns = append(f.conns[:i], f.conns[i+1:]...)
					i--
				}
				tcpForwardedMessages.WithLabelValues(f.addr).Inc()
			}
			tcpCurrentConnections.WithLabelValues(f.addr).Set(float64(len(f.conns)))
			f.mut.Unlock()

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

type tcpListener struct {
	addr      string
	forwarder *tcpForwarder
}

func (t *tcpListener) String() string {
	return fmt.Sprintf("tcp-listener(%s)@%p", t.addr, t)
}

func (t *tcpListener) Serve(ctx context.Context) error {
	l, err := net.Listen("tcp", t.addr)
	if err != nil {
		return err
	}
	defer l.Close()

	go func() {
		<-ctx.Done()
		_ = l.Close()
	}()

	tcpIncomingConnections.WithLabelValues(t.addr)

	for {
		conn, err := l.Accept()
		if err != nil {
			return err
		}

		t.forwarder.addConn(conn)
		tcpIncomingConnections.WithLabelValues(t.addr).Inc()
	}
}
