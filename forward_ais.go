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

const (
	maxPacketSize = 1472
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

func forwardAIS(c <-chan string, addrs []string) {
	dsts := make([]net.Conn, 0, len(addrs))
	for _, addr := range addrs {
		dst, err := net.Dial("udp", addr)
		if err != nil {
			log.Printf("Can't forward to %s: %v", addr, err)
			continue
		}
		log.Println("Forwarding AIS to udp/" + addr)
		dsts = append(dsts, dst)

		dstAddr := dst.RemoteAddr().String()
		aisSentPackets.WithLabelValues(dstAddr)
		aisSentBytes.WithLabelValues(dstAddr)
		aisSendErrors.WithLabelValues(dstAddr)
	}
	if len(dsts) == 0 {
		log.Fatal("No valid AIS forward destination")
	}

	var lastWrite time.Time
	outBuf := new(bytes.Buffer)

	for line := range c {
		aisReceivedMessages.Inc()

		if outBuf.Len() > 0 && time.Since(lastWrite) > time.Minute ||
			outBuf.Len()+len(line)+1 > maxPacketSize {
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
			lastWrite = time.Now()
		}

		fmt.Fprintf(outBuf, "%s\n", line)
	}
}
