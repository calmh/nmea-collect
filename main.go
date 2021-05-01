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
		SampleInterval        time.Duration `help:"Time between recorded track points" default:"10s"`
		TriggerDistanceMeters float64       `help:"Minimum movement to start track (m)" default:"25"`
		TriggerTimeWindow     time.Duration `help:"Time window for starting track" default:"1m"`
		CooldownTimeWindow    time.Duration `help:"Time window before ending track" default:"5m"`
		TCPAddr               []string      `help:"TCP input addr (e.g., 172.16.1.2:2000)" placeholder:"ADDR"`
		UDPPort               []int         `help:"UDP input listen port (e.g., 2000)" placeholder:"PORT"`
		Serial                []string      `help:"Serial port input (e.g., /dev/ttyS0)" placeholder:"DEV"`
		Verbose               bool
		ForwardAISUDP         []string `name:"forward-ais-udp" help:"Forward AIS to UDP target" placeholder:"ADDR"`
		ListenAllTCP          string   `default:":2000" help:"TCP output listen address (all NMEA)" placeholder:"ADDR"`
		ListenAISTCP          string   `default:":2010" name:"listen-ais-tcp" help:"TCP output listen address (AIS only)" placeholder:"ADDR"`
		ListenPrometheus      string   `default:":9140"`
		RawFilePattern        string   `default:"nmea-raw.20060102-150405.gz"`
		RawFileWriteBuffer    int      `default:"131072"`
	}

	log.SetFlags(0)
	kong.Parse(&cli)

	for key, parser := range parsers {
		nmea.RegisterParser(key, parser)
	}

	c := make(chan string)

	go copyInto(c, lines(os.Stdin, "stdin"))

	for _, addr := range cli.TCPAddr {
		go readTCPInto(c, addr)
	}

	for _, port := range cli.UDPPort {
		go readUDPInto(c, port)
	}

	for _, dev := range cli.Serial {
		go readSerialInto(c, dev)
	}

	if cli.ListenAllTCP != "" {
		a, b := tee(c)
		c = a
		go forwardTCP(b, cli.ListenAllTCP)
	}

	if len(cli.ForwardAISUDP) > 0 {
		a, b := tee(c)
		c = a
		ais := prefix(b, "!AI")
		go forwardAIS(ais, cli.ForwardAISUDP)
	}

	if cli.ListenAISTCP != "" {
		a, b := tee(c)
		c = a
		ais := prefix(b, "!AI")
		go forwardTCP(ais, cli.ListenAISTCP)
	}

	if cli.ListenPrometheus != "" {
		a, b := tee(c)
		c = a
		go exposeMetrics(b)
		http.Handle("/metrics", promhttp.Handler())
		go http.ListenAndServe(cli.ListenPrometheus, nil)
	}

	if cli.RawFilePattern != "" {
		a, b := tee(c)
		c = a
		go collectRAW(cli.RawFilePattern, cli.RawFileWriteBuffer, b)
	}

	gpx := &gpx.AutoGPX{
		Opener:                newGPXFile,
		SampleInterval:        cli.SampleInterval,
		TriggerDistanceMeters: cli.TriggerDistanceMeters,
		TriggerTimeWindow:     cli.TriggerTimeWindow,
		CooldownTimeWindow:    cli.CooldownTimeWindow,
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
