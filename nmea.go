package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

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

func readTCPInto(c chan<- string, addr string) *lineWriter {
	return &lineWriter{
		reader: func() (io.ReadCloser, error) { return tcpReader(addr) },
		name:   fmt.Sprintf("tcp/%s", addr),
		lines:  c,
	}
}

func tcpReader(addr string) (io.ReadCloser, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("reader: %w", err)
	}
	return conn, nil
}

func readUDPInto(c chan<- string, port int) *lineWriter {
	return &lineWriter{
		reader: func() (io.ReadCloser, error) { return udpReader(port) },
		name:   fmt.Sprintf("udp/%d", port),
		lines:  c,
	}
}

func udpReader(port int) (io.ReadCloser, error) {
	laddr := &net.UDPAddr{Port: port}
	conn, err := net.ListenUDP("udp", laddr)
	if err != nil {
		return nil, fmt.Errorf("reader: %w", err)
	}
	return conn, nil
}

func readSerialInto(c chan<- string, dev string) *lineWriter {
	return &lineWriter{
		reader: func() (io.ReadCloser, error) { return os.Open(dev) },
		name:   dev,
		lines:  c,
	}
}

type lineWriter struct {
	reader func() (io.ReadCloser, error)
	name   string
	lines  chan<- string
}

func (r *lineWriter) String() string {
	return fmt.Sprintf("%s@%p", r.name, r)
}

func (r *lineWriter) Serve(ctx context.Context) error {
	reader, err := r.reader()
	if err != nil {
		return err
	}
	defer reader.Close()

	sc := bufio.NewScanner(reader)
	sc.Buffer(make([]byte, 0, 65536), 65536)

	nmeaMessagesInput.WithLabelValues(r.name)
	nmeaMessagesBad.WithLabelValues(r.name)

	for sc.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := sc.Text()
		nmeaMessagesInput.WithLabelValues(r.name).Inc()
		if line == "" {
			nmeaMessagesBad.WithLabelValues(r.name).Inc()
			continue
		}
		switch line[0] {
		case '!', '$':
			idx := strings.LastIndexByte(line, '*')
			if idx == -1 {
				nmeaMessagesBad.WithLabelValues(r.name).Inc()
				continue
			}
			chk := nmea.Checksum(line[1:idx])
			if chk != line[idx+1:] {
				nmeaMessagesBad.WithLabelValues(r.name).Inc()
				continue
			}
			select {
			case r.lines <- line:
			case <-ctx.Done():
				return ctx.Err()
			}
		default:
			nmeaMessagesBad.WithLabelValues(r.name).Inc()
		}
	}

	return sc.Err()
}

func linesInto(c chan<- string, r io.ReadCloser, name string) *lineWriter {
	return &lineWriter{
		reader: func() (io.ReadCloser, error) { return r, nil },
		name:   name,
		lines:  make(chan string, 1),
	}
}

type Tee struct {
	input   <-chan string
	prefix  string
	outputs []chan string
}

func NewTee(input <-chan string) *Tee {
	return &Tee{input: input}
}

func NewFilteredTee(input <-chan string, prefix string) *Tee {
	return &Tee{input: input, prefix: prefix}
}

func (t *Tee) String() string {
	if t.prefix == "" {
		return fmt.Sprintf("nmea-tee@%p", t)
	}
	return fmt.Sprintf("nmea-tee(%q)@%p", t.prefix, t)
}

func (t *Tee) Output() <-chan string {
	c := make(chan string, 1)
	t.outputs = append(t.outputs, c)
	return c
}

func (t *Tee) Serve(ctx context.Context) error {
	for {
		select {
		case line := <-t.input:
			if !strings.HasPrefix(line, t.prefix) {
				continue
			}
			for _, out := range t.outputs {
				select {
				case out <- line:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
