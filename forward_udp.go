package main

import (
	"bytes"
	"fmt"
	"log"
	"net"
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

func forwardUDP(c <-chan string, addrs []string, maxPacketSize int, maxDelay time.Duration) {
	dsts := make([]net.Conn, 0, len(addrs))
	for _, addr := range addrs {
		dst, err := net.Dial("udp", addr)
		if err != nil {
			log.Printf("Can't forward to %s: %v", addr, err)
			continue
		}
		log.Println("Forwarding to udp/" + addr)
		dsts = append(dsts, dst)

		dstAddr := dst.RemoteAddr().String()
		aisSentPackets.WithLabelValues(dstAddr)
		aisSentBytes.WithLabelValues(dstAddr)
		aisSendErrors.WithLabelValues(dstAddr)
	}
	if len(dsts) == 0 {
		log.Fatal("No valid UDP forward destination")
	}

	outBuf := new(bytes.Buffer)

	flush := func() {
		if outBuf.Len() == 0 {
			return
		}
		for _, dst := range dsts {
			_, err := dst.Write(outBuf.Bytes())
			dstAddr := dst.RemoteAddr().String()
			if err != nil {
				log.Printf("Write %v: %v", dst.RemoteAddr(), err)
				aisSendErrors.WithLabelValues(dstAddr).Inc()
				continue
			}
			aisSentPackets.WithLabelValues(dstAddr).Inc()
			aisSentBytes.WithLabelValues(dstAddr).Add(float64(outBuf.Len()))
		}
		outBuf.Reset()
	}

	timer := time.NewTimer(maxDelay)
	defer timer.Stop()

	for {
		select {
		case line := <-c:
			aisReceivedMessages.Inc()

			if outBuf.Len()+len(line)+2 > maxPacketSize {
				flush()
				timer.Reset(maxDelay)
			}

			fmt.Fprintf(outBuf, "%s\r\n", line)

		case <-timer.C:
			flush()
			timer.Reset(maxDelay)
		}
	}
}
