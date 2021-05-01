package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"time"

	nmea "github.com/adrianmo/go-nmea"
	"github.com/alecthomas/kong"
	"github.com/calmh/nmea-collect/gpx"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	var cli struct {
		InputTCPConnect []string `help:"TCP connect input addresses (e.g., 172.16.1.2:2000)" placeholder:"ADDR" group:"Input"`
		InputUDPListen  []int    `help:"UDP broadcast input listen ports (e.g., 2000)" placeholder:"PORT" group:"Input"`
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

		OutputGPXSampleInterval  time.Duration `help:"Time between track points" default:"10s" group:"GPX File Output"`
		OutputGPXMovingDistance  float64       `help:"Minimum travel in time window to consider us moving (m)" default:"25" group:"GPX File Output"`
		OutputGPXStartTimeWindow time.Duration `help:"Movement time window for starting track" default:"1m" group:"GPX File Output"`
		OutputGPXStopTimeWindow  time.Duration `help:"Movement time window before ending track" default:"5m" group:"GPX File Output"`

		OutputRawPattern      string        `default:"nmea-raw.20060102-150405.gz" help:"File naming pattern, see https://golang.org/pkg/time/#Time.Format" group:"Raw NMEA File Output"`
		OutputRawBufferSize   int           `default:"131072" help:"Write buffer for output file" group:"Raw NMEA File Output"`
		OutputRawUncompressed bool          `help:"Write uncompressed NMEA (default is gzipped)" group:"Raw NMEA File Output"`
		OutputRawTimeWindow   time.Duration `default:"24h" help:"How often to create a new raw file" placeholder:"DURATION" group:"Raw NMEA File Output"`

		PrometheusMetricsListen string `default:"127.0.0.1:9140" help:"HTTP listen address for Prometheus metrics endpoint" placeholder:"ADDR" group:"Metrics"`
	}

	log.SetFlags(0)
	kong.Parse(&cli)

	for key, parser := range parsers {
		nmea.RegisterParser(key, parser)
	}

	c := make(chan string)

	if cli.InputStdin {
		go copyInto(c, lines(os.Stdin, "stdin"))
	}

	for _, addr := range cli.InputTCPConnect {
		go readTCPInto(c, addr)
	}

	for _, port := range cli.InputUDPListen {
		go readUDPInto(c, port)
	}

	for _, dev := range cli.InputSerial {
		go readSerialInto(c, dev)
	}

	if cli.ForwardAllTCPListen != "" {
		a, b := tee(c)
		c = a
		go forwardTCP(b, cli.ForwardAllTCPListen)
	}

	if len(cli.ForwardUDPAll) > 0 {
		a, b := tee(c)
		c = a
		go forwardUDP(b, cli.ForwardUDPAll, cli.ForwardUDPAllMaxPacketSize, cli.ForwardUDPAllMaxDelay)
	}

	if len(cli.ForwardUDPAIS) > 0 {
		a, b := tee(c)
		c = a
		ais := prefix(b, "!AI")
		go forwardUDP(ais, cli.ForwardUDPAIS, cli.ForwardUDPAISMaxPacketSize, cli.ForwardUDPAISMaxDelay)
	}

	if cli.ForwardAISTCPListen != "" {
		a, b := tee(c)
		c = a
		ais := prefix(b, "!AI")
		go forwardTCP(ais, cli.ForwardAISTCPListen)
	}

	if cli.PrometheusMetricsListen != "" {
		a, b := tee(c)
		c = a
		go exposeMetrics(b)
		http.Handle("/metrics", promhttp.Handler())
		go http.ListenAndServe(cli.PrometheusMetricsListen, nil)
	}

	if cli.OutputRawPattern != "" {
		a, b := tee(c)
		c = a
		go collectRAW(cli.OutputRawPattern, cli.OutputRawBufferSize, cli.OutputRawTimeWindow, !cli.OutputRawUncompressed, b)
	}

	gpx := &gpx.AutoGPX{
		Opener:                newGPXFile,
		SampleInterval:        cli.OutputGPXSampleInterval,
		TriggerDistanceMeters: cli.OutputGPXMovingDistance,
		TriggerTimeWindow:     cli.OutputGPXStartTimeWindow,
		CooldownTimeWindow:    cli.OutputGPXStopTimeWindow,
	}

	collectGPX(prefix(c, "$"), gpx)
}

var gpxFilesCreatedTotal = promauto.NewCounter(prometheus.CounterOpts{
	Namespace: "nmea",
	Subsystem: "gpx",
	Name:      "files_created_total",
})

func newGPXFile() (io.WriteCloser, error) {
	name := time.Now().UTC().Format("track-20060102-150405.gpx")
	log.Println("Creating new GPX track", name)
	gpxFilesCreatedTotal.Inc()
	return os.Create(name)
}
