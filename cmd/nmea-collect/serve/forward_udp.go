package serve

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	aisReceivedMessages = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "nmea",
		Subsystem: "ais",
		Name:      "received_messages_total",
	})
	aisSentPackets = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "nmea",
		Subsystem: "ais",
		Name:      "udp_sent_packets_total",
	}, []string{"destination"})
	aisSentBytes = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "nmea",
		Subsystem: "ais",
		Name:      "udp_sent_bytes_total",
	}, []string{"destination"})
	aisSendErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "nmea",
		Subsystem: "ais",
		Name:      "udp_send_errors_total",
	}, []string{"destination"})
)

type udpForwarder struct {
	c             <-chan string
	addrs         []string
	maxPacketSize int
	maxDelay      time.Duration
	buf           bytes.Buffer
}

func forwardUDP(c <-chan string, addrs []string, maxPacketSize int, maxDelay time.Duration) *udpForwarder {
	return &udpForwarder{
		c:             c,
		addrs:         addrs,
		maxPacketSize: maxPacketSize,
		maxDelay:      maxDelay,
	}
}

func (f *udpForwarder) String() string {
	return fmt.Sprintf("udp-forwarder(%s)@%p", strings.Join(f.addrs, "-"), f)
}

func (f *udpForwarder) Serve(ctx context.Context) error {
	dsts := make([]net.Conn, 0, len(f.addrs))
	for _, addr := range f.addrs {
		dst, err := net.Dial("udp", addr)
		if err != nil {
			log.Printf("Can't forward to %s: %v", addr, err)
			continue
		}
		defer dst.Close()
		dsts = append(dsts, dst)

		dstAddr := dst.RemoteAddr().String()
		aisSentPackets.WithLabelValues(dstAddr)
		aisSentBytes.WithLabelValues(dstAddr)
		aisSendErrors.WithLabelValues(dstAddr)
	}
	if len(dsts) == 0 {
		return errors.New("no UDP forward destination")
	}

	timer := time.NewTimer(f.maxDelay)
	defer timer.Stop()

	for {
		select {
		case line := <-f.c:
			aisReceivedMessages.Inc()

			if f.buf.Len()+len(line)+2 > f.maxPacketSize {
				f.flush(dsts)
				timer.Reset(f.maxDelay)
			}

			fmt.Fprintf(&f.buf, "%s\r\n", line)

		case <-timer.C:
			f.flush(dsts)
			timer.Reset(f.maxDelay)
		}
	}
}

func (f *udpForwarder) flush(dsts []net.Conn) {
	if f.buf.Len() == 0 {
		return
	}
	for _, dst := range dsts {
		_, err := dst.Write(f.buf.Bytes())
		dstAddr := dst.RemoteAddr().String()
		if err != nil {
			aisSendErrors.WithLabelValues(dstAddr).Inc()
			continue
		}
		aisSentPackets.WithLabelValues(dstAddr).Inc()
		aisSentBytes.WithLabelValues(dstAddr).Add(float64(f.buf.Len()))
	}
	f.buf.Reset()
}
