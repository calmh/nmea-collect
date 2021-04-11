package main

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/adrianmo/go-nmea"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	rawMessagesRecorded = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "nmea",
		Subsystem: "gpx",
		Name:      "raw_recorded_total",
	})
)

func collectRAW(c <-chan string) error {
	pat := "raw.20060102.nmea.gz"
	cur := ""
	var fd io.WriteCloser
	defer func() {
		if fd != nil {
			fd.Close()
		}
	}()

	var lastZDA time.Time
	for line := range c {
		now := time.Now().UTC()
		trunc := now.Truncate(time.Second)

		name := trunc.Format(pat)
		if name != cur {
			if fd != nil {
				fd.Close()
			}
			nfd, err := os.Create(name)
			if err != nil {
				return err
			}
			gw := gzip.NewWriter(nfd)
			bw := bufio.NewWriterSize(gw, 65536)
			fd = &bufWriter{Writer: gw, bw: bw}
			cur = name
		}

		if !trunc.Equal(lastZDA) {
			line := fmt.Sprintf("VRZDA,%s,%02d,%02d,%04d,00,00", now.Format("150405.00"), now.Day(), now.Month(), now.Year())
			fmt.Fprintf(fd, "$%s*%s\n", line, nmea.Checksum(line))
			lastZDA = trunc
		}

		fmt.Fprintf(fd, "%s\n", line)
	}

	return nil
}

type bufWriter struct {
	*gzip.Writer
	bw *bufio.Writer
}

func (w *bufWriter) Close() error {
	w.bw.Flush()
	return w.Writer.Close()
}
