package main

import (
	"log"
	"net/http"
	"os"
	"time"

	nmea "github.com/adrianmo/go-nmea"
	"github.com/alecthomas/kong"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var cli struct {
	SampleInterval        time.Duration `help:"Time between recorded track points" default:"30s"`
	TriggerDistanceMeters float64       `help:"Minimum movement to start track (m)" default:"25"`
	TriggerTimeWindow     time.Duration `help:"Time window for starting track" default:"1m"`
	CooldownTimeWindow    time.Duration `help:"Time window before ending track" default:"5m"`
	TCPAddr               []string
	UDPPort               []int
	Verbose               bool
	ForwardAISUDP         []string `name:"forward-ais-udp"`
	ListenAllTCP          string   `default:":2000"`
	ListenAISTCP          string   `default:":2010" name:"listen-ais-tcp"`
	ListenPrometheus      string   `default:":9140"`
}

func main() {
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

	collect(prefix(c, "$"))
}
