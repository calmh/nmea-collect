package main

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
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
	rawFilesCreated = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "nmea",
		Subsystem: "gpx",
		Name:      "raw_files_created_total",
	})
)

type rawCollector struct {
	filePat  string
	bufSize  int
	window   time.Duration
	compress bool
	c        <-chan string
}

func collectRAW(filePat string, bufSize int, window time.Duration, compress bool, c <-chan string) *rawCollector {
	return &rawCollector{
		filePat:  filePat,
		bufSize:  bufSize,
		window:   window,
		compress: compress,
		c:        c,
	}
}

func (r *rawCollector) String() string {
	return fmt.Sprintf("raw-collector(%q)@%p", r.filePat, r)
}

func (r *rawCollector) Serve(ctx context.Context) error {
	var fd io.WriteCloser
	defer func() {
		if fd != nil {
			fd.Close()
		}
	}()

	var lastZDA time.Time
	var day time.Time
	for {
		select {
		case line := <-r.c:
			now := time.Now().UTC()
			truncS := now.Truncate(time.Second)
			truncDay := now.Truncate(r.window)

			if !truncDay.Equal(day) {
				if fd != nil {
					fd.Close()
				}
				name := truncS.Format(r.filePat)
				log.Println("Creating raw file", name)
				nfd, err := os.Create(name)
				if err != nil {
					return err
				}
				if r.compress {
					gw := gzip.NewWriter(nfd)
					bw := bufio.NewWriterSize(gw, r.bufSize)
					fd = &bufWriter{Writer: bw, flusher: bw, closers: []io.Closer{gw, nfd}}
				} else {
					bw := bufio.NewWriterSize(nfd, r.bufSize)
					fd = &bufWriter{Writer: bw, flusher: bw, closers: []io.Closer{nfd}}
				}
				day = truncDay
				rawFilesCreated.Inc()
			}

			if !truncS.Equal(lastZDA) {
				line := fmt.Sprintf("VRZDA,%s,%02d,%02d,%04d,00,00", now.Format("150405.00"), now.Day(), now.Month(), now.Year())
				fmt.Fprintf(fd, "$%s*%s\r\n", line, nmea.Checksum(line))
				lastZDA = truncS
			}

			fmt.Fprintf(fd, "%s\r\n", line)
			rawMessagesRecorded.Inc()

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

type bufWriter struct {
	io.Writer
	flusher interface{ Flush() error }
	closers []io.Closer
}

func (w *bufWriter) Close() error {
	w.flusher.Flush()
	for _, c := range w.closers {
		_ = c.Close()
	}
	return nil
}
