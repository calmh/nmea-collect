package serve

import (
	"context"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"calmh.dev/nmea-collect/internal/gpx/writer"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/thejerf/suture/v4"
)

type CLI struct {
	InputTCPConnect []string `help:"TCP connect input addresses (e.g., 172.16.1.2:2000)" placeholder:"ADDR" group:"Input"`
	InputUDPListen  []int    `help:"UDP broadcast input listen ports (e.g., 2000)" placeholder:"PORT" group:"Input"`
	InputHTTPListen []int    `help:"HTTP input listen ports (e.g., 8080)" placeholder:"PORT" group:"Input"`
	InputSerial     []string `help:"Serial port inputs (e.g., /dev/ttyS0)" placeholder:"DEV" group:"Input"`
	InputStdin      bool     `help:"Read NMEA from standard input" group:"Input"`

	ForwardUDPAll              []string      `help:"UDP output destination address (all NMEA)" placeholder:"ADDR" group:"UDP output"`
	ForwardUDPAllMaxPacketSize int           `help:"Maximum UDP payload size (all NMEA)" default:"1472" group:"UDP output"`
	ForwardUDPAllMaxDelay      time.Duration `help:"Maximum UDP buffer delay (all NMEA)" default:"1s" group:"UDP output"`

	ForwardUDPAIS              []string      `name:"forward-ais-udp" help:"UDP output destination address (AIS only)" placeholder:"ADDR" group:"UDP output"`
	ForwardUDPAISMaxPacketSize int           `help:"Maximum UDP payload size (AIS only)" name:"forward-ais-udp-max-packet-size" default:"1472" group:"UDP output"`
	ForwardUDPAISMaxDelay      time.Duration `help:"Maximum UDP buffer delay (AIS only)" name:"forward-ais-udp-max-delay" default:"10s" group:"UDP output"`

	ForwardAllTCPListen string `default:":2000" help:"TCP listen address (all NMEA)" placeholder:"ADDR" group:"TCP output"`
	ForwardAISTCPListen string `default:":2010" name:"forward-ais-tcp-listen" help:"TCP listen address (AIS only)" placeholder:"ADDR" group:"TCP output"`

	OutputGPXPattern         string        `default:"track-20060102-150405.gpx" help:"File naming pattern, see https://golang.org/pkg/time/#Time.Format" group:"GPX File Output"`
	OutputGPXSampleInterval  time.Duration `help:"Time between track points" default:"10s" group:"GPX File Output"`
	OutputGPXMovingDistance  float64       `help:"Minimum travel in time window to consider us moving (meters)" default:"25" group:"GPX File Output"`
	OutputGPXStartTimeWindow time.Duration `help:"Movement time window for starting track" default:"1m" group:"GPX File Output"`
	OutputGPXStopTimeWindow  time.Duration `help:"Movement time window before ending track" default:"5m" group:"GPX File Output"`

	OutputRawPattern       string        `default:"nmea-raw.20060102-150405.gz" help:"File naming pattern, see https://golang.org/pkg/time/#Time.Format" group:"Raw NMEA File Output"`
	OutputRawBufferSize    int           `default:"131072" help:"Write buffer for output file" group:"Raw NMEA File Output"`
	OutputRawUncompressed  bool          `help:"Write uncompressed NMEA (default is gzipped)" group:"Raw NMEA File Output"`
	OutputRawTimeWindow    time.Duration `default:"24h" help:"How often to create a new raw file" group:"Raw NMEA File Output"`
	OutputRawFlushInterval time.Duration `default:"5m" help:"How often to flush raw data to disk" group:"Raw NMEA File Output"`

	PrometheusMetricsListen string `default:"127.0.0.1:9140" help:"HTTP listen address for Prometheus metrics endpoint" placeholder:"ADDR" group:"Metrics"`
}

func (cli *CLI) Run(ctx context.Context) error {
	sup := suture.NewSimple("main")
	input := make(chan string, 4096)
	tee := NewTee("main", input)
	sup.Add(tee)

	if cli.InputStdin {
		log.Println("Reading NMEA from stdin")
		sup.Add(linesInto(input, os.Stdin, "stdin"))
	}

	for _, addr := range cli.InputTCPConnect {
		log.Println("Reading NMEA from TCP", addr)
		sup.Add(readTCPInto(input, addr))
	}

	for _, port := range cli.InputUDPListen {
		log.Println("Reading NMEA on UDP port", port)
		sup.Add(readUDPInto(input, port))
	}

	for _, port := range cli.InputHTTPListen {
		log.Println("Reading NMEA from HTTP POSTs on port", port)
		sup.Add(readHTTPInto(input, port))
	}

	for _, dev := range cli.InputSerial {
		log.Println("Reading NMEA from serial device", dev)
		sup.Add(readSerialInto(input, dev))
	}

	if cli.ForwardAllTCPListen != "" {
		log.Println("Forwarding NMEA to incoming connections on", cli.ForwardAllTCPListen)
		sup.Add(forwardTCP(tee.Output(), cli.ForwardAllTCPListen))
	}

	if len(cli.ForwardUDPAll) > 0 {
		log.Println("Forwarding NMEA to UDP", strings.Join(cli.ForwardUDPAll, ", "))
		sup.Add(forwardUDP(tee.Output(), cli.ForwardUDPAll, cli.ForwardUDPAllMaxPacketSize, cli.ForwardUDPAllMaxDelay))
	}

	var ais *Tee

	if len(cli.ForwardUDPAIS) > 0 {
		if ais == nil {
			ais = NewFilteredTee("AIS", tee.Output(), "!AI")
			sup.Add(ais)
		}
		log.Println("Forwarding AIS to UDP", strings.Join(cli.ForwardUDPAIS, ", "))
		sup.Add(forwardUDP(ais.Output(), cli.ForwardUDPAIS, cli.ForwardUDPAISMaxPacketSize, cli.ForwardUDPAISMaxDelay))
	}

	if cli.ForwardAISTCPListen != "" {
		if ais == nil {
			ais = NewFilteredTee("AIS", tee.Output(), "!AI")
			sup.Add(ais)
		}
		log.Println("Forwarding AIS to incoming connections on", cli.ForwardAISTCPListen)
		sup.Add(forwardTCP(ais.Output(), cli.ForwardAISTCPListen))
	}

	instruments := &instrumentsCollector{c: tee.Output()}
	sup.Add(instruments)

	aisCounter := &aisContactsCounter{c: tee.Output()}
	sup.Add(aisCounter)

	if cli.PrometheusMetricsListen != "" {
		url := &url.URL{Scheme: "http", Host: cli.PrometheusMetricsListen, Path: "/metrics"}
		log.Println("Exporting instruments and metrics on", url)
		sup.Add(&prometheusListener{cli.PrometheusMetricsListen})
	}

	if cli.OutputRawPattern != "" {
		log.Println("Writing raw files to files named like", cli.OutputRawPattern)
		sup.Add(collectRAW(cli.OutputRawPattern, cli.OutputRawBufferSize, cli.OutputRawTimeWindow, cli.OutputRawFlushInterval, !cli.OutputRawUncompressed, tee.Output()))
	}

	if cli.OutputGPXPattern != "" {
		gpx := &writer.AutoGPX{
			Opener: func(t time.Time) (io.WriteCloser, error) {
				return newGPXFile(cli.OutputGPXPattern, t)
			},
			SampleInterval:        cli.OutputGPXSampleInterval,
			TriggerDistanceMeters: cli.OutputGPXMovingDistance,
			TriggerTimeWindow:     cli.OutputGPXStartTimeWindow,
			CooldownTimeWindow:    cli.OutputGPXStopTimeWindow,
		}

		log.Println("Collecting GPX tracks to files named like", cli.OutputGPXPattern)
		nonAIS := NewFilteredTee("non-AIS", tee.Output(), "$")
		sup.Add(nonAIS)
		sup.Add(collectGPX(nonAIS.Output(), gpx, instruments))
	}

	return sup.Serve(ctx)
}

var gpxFilesCreatedTotal = promauto.NewCounter(prometheus.CounterOpts{
	Namespace: "nmea",
	Subsystem: "gpx",
	Name:      "files_created_total",
})

func newGPXFile(pattern string, t time.Time) (io.WriteCloser, error) {
	name := t.UTC().Format(pattern)
	log.Println("Creating new GPX track", name)
	gpxFilesCreatedTotal.Inc()
	_ = os.MkdirAll(filepath.Dir(name), 0o755)
	return os.Create(name)
}
