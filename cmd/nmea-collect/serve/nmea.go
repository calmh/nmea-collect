package serve

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	nmea "github.com/adrianmo/go-nmea"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const teeBufferSize = 4096

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
	nmeaMessagesEmpty = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "nmea",
		Subsystem: "input",
		Name:      "messages_empty_total",
	}, []string{"source"})
	nmeaMessagesNoChecksum = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "nmea",
		Subsystem: "input",
		Name:      "messages_no_checksum_total",
	}, []string{"source"})
	nmeaMessagesNonNMEA = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "nmea",
		Subsystem: "input",
		Name:      "messages_non_nmea_total",
	}, []string{"source"})
	nmeaMessagesTeeRead = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "nmea",
		Subsystem: "tee",
		Name:      "messages_input_total",
	}, []string{"tee"})
	nmeaMessagesTeeSent = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "nmea",
		Subsystem: "tee",
		Name:      "messages_output_total",
	}, []string{"tee"})
	nmeaMessagesTeeFilterSkipped = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "nmea",
		Subsystem: "tee",
		Name:      "messages_filter_skipped_total",
	}, []string{"tee"})
	nmeaMessagesTeeDropped = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "nmea",
		Subsystem: "tee",
		Name:      "messages_dropped_total",
	}, []string{"tee"})
)

func readTCPInto(c chan<- string, addr string) *lineWriter {
	return &lineWriter{
		reader:      func() (io.ReadCloser, error) { return tcpReader(addr) },
		name:        fmt.Sprintf("tcp/%s", addr),
		lines:       c,
		readTimeout: 15 * time.Second,
	}
}

func tcpReader(addr string) (io.ReadCloser, error) {
	conn, err := net.DialTimeout("tcp", addr, 15*time.Second)
	if err != nil {
		return nil, fmt.Errorf("reader: %w", err)
	}
	return conn, nil
}

func readHTTPInto(c chan<- string, port int) *lineWriter {
	return &lineWriter{
		reader: func() (io.ReadCloser, error) { return httpReader(port) },
		name:   fmt.Sprintf("http//:%d", port),
		lines:  c,
	}
}

func httpReader(port int) (io.ReadCloser, error) {
	rd, wr := io.Pipe()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, err := io.Copy(wr, r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
	srv := http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}
	go func() {
		srv.ListenAndServe()
	}()
	return rd, nil
}

func readUDPInto(c chan<- string, port int) *lineWriter {
	return &lineWriter{
		reader:      func() (io.ReadCloser, error) { return udpReader(port) },
		name:        fmt.Sprintf("udp/%d", port),
		lines:       c,
		readTimeout: 15 * time.Second,
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
	reader      func() (io.ReadCloser, error)
	name        string
	lines       chan<- string
	readTimeout time.Duration
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
	nmeaMessagesEmpty.WithLabelValues(r.name)
	nmeaMessagesNoChecksum.WithLabelValues(r.name)
	nmeaMessagesNonNMEA.WithLabelValues(r.name)

	if err := r.trySetDeadline(reader); err != nil {
		return err
	}

	for sc.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := r.trySetDeadline(reader); err != nil {
			return err
		}

		line := sc.Text()
		nmeaMessagesInput.WithLabelValues(r.name).Inc()
		if line == "" {
			nmeaMessagesEmpty.WithLabelValues(r.name).Inc()
			continue
		}
		switch line[0] {
		case '!', '$':
			idx := strings.LastIndexByte(line, '*')
			if idx == -1 {
				nmeaMessagesNoChecksum.WithLabelValues(r.name).Inc()
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
			nmeaMessagesNonNMEA.WithLabelValues(r.name).Inc()
		}
	}

	if err := sc.Err(); err != nil {
		return err
	}
	return io.EOF
}

func (r *lineWriter) trySetDeadline(v any) error {
	if r.readTimeout == 0 {
		return nil
	}
	type deadliner interface {
		SetReadDeadline(t time.Time) error
	}
	if rd, ok := v.(deadliner); ok {
		return rd.SetReadDeadline(time.Now().Add(r.readTimeout))
	}
	return nil
}

func linesInto(c chan<- string, r io.ReadCloser, name string) *lineWriter {
	return &lineWriter{
		reader: func() (io.ReadCloser, error) { return r, nil },
		name:   name,
		lines:  c,
	}
}

type Tee struct {
	name    string
	input   <-chan string
	prefix  string
	outputs []chan string
}

func NewTee(name string, input <-chan string) *Tee {
	return &Tee{name: name, input: input}
}

func NewFilteredTee(name string, input <-chan string, prefix string) *Tee {
	return &Tee{name: name, input: input, prefix: prefix}
}

func (t *Tee) String() string {
	if t.prefix == "" {
		return fmt.Sprintf("nmea-tee@%p", t)
	}
	return fmt.Sprintf("nmea-tee(%q)@%p", t.prefix, t)
}

func (t *Tee) Output() <-chan string {
	c := make(chan string, teeBufferSize)
	t.outputs = append(t.outputs, c)
	return c
}

func (t *Tee) Serve(ctx context.Context) error {
	for {
		select {
		case line := <-t.input:
			nmeaMessagesTeeRead.WithLabelValues(t.name).Inc()
			if !strings.HasPrefix(line, t.prefix) {
				nmeaMessagesTeeFilterSkipped.WithLabelValues(t.name).Inc()
				continue
			}
			for _, out := range t.outputs {
				select {
				case out <- line:
					nmeaMessagesTeeSent.WithLabelValues(t.name).Inc()
				case <-ctx.Done():
					return ctx.Err()
				default:
					nmeaMessagesTeeDropped.WithLabelValues(t.name).Inc()
				}
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
