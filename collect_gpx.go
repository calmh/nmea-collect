package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	nmea "github.com/adrianmo/go-nmea"
	"github.com/kastelo/nmea-collect/gpx"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	gpxPositionsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "nmea",
		Subsystem: "gpx",
		Name:      "sampled_positions_total",
	})
	gpxFilesCreatedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "nmea",
		Subsystem: "gpx",
		Name:      "files_created_total",
	})
	gpxInputMessages = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "nmea",
		Subsystem: "gpx",
		Name:      "input_messages_total",
	})
	gpxBadMessages = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "nmea",
		Subsystem: "gpx",
		Name:      "bad_messages_total",
	})
	gpxUnsupportedMessages = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "nmea",
		Subsystem: "gpx",
		Name:      "unsupported_messages_total",
	})
)

func collect(c <-chan string) {
	exts := make(gpx.Extensions)
	gpx := gpx.AutoGPX{
		Opener:                newGPXFile,
		SampleInterval:        cli.SampleInterval,
		TriggerDistanceMeters: cli.TriggerDistanceMeters,
		TriggerTimeWindow:     cli.TriggerTimeWindow,
		CooldownTimeWindow:    cli.CooldownTimeWindow,
	}
	defer gpx.Flush()

	for line := range c {
		gpxInputMessages.Inc()
		sent, err := nmea.Parse(line)
		if err != nil {
			if strings.Contains(err.Error(), "not supported") {
				gpxUnsupportedMessages.Inc()
				continue
			}
			gpxBadMessages.Inc()
			if cli.Verbose {
				log.Printf("parse: %q: %v", line, err)
			}
			continue
		}

		switch sent.DataType() {
		case TypeDPT:
			dpt := sent.(DPT)
			exts.Set("waterdepth", fmt.Sprintf("%.01f", dpt.Depth))

		case TypeHDG:
			hdg := sent.(HDG)
			exts.Set("heading", fmt.Sprintf("%.0f", hdg.Heading))

		case TypeMTW:
			mtw := sent.(MTW)
			exts.Set("watertemp", fmt.Sprintf("%.01f", mtw.Temperature))

		case TypeMWV:
			mwv := sent.(MWV)
			if mwv.Reference == "R" && mwv.Status == "A" {
				exts.Set("windangle", fmt.Sprintf("%.0f", mwv.Angle))
				exts.Set("windspeed", fmt.Sprintf("%.01f", mwv.Speed))
			}

		case TypeVLW:
			mwv := sent.(VLW)
			exts.Set("log", fmt.Sprintf("%.1f", mwv.TotalDistanceNauticalMiles))

		case nmea.TypeVHW:
			vhw := sent.(nmea.VHW)
			exts.Set("waterspeed", fmt.Sprintf("%.01f", vhw.SpeedThroughWaterKnots))

		case nmea.TypeGLL:
			rmc := sent.(nmea.GLL)
			when := time.Now()
			gpx.Sample(rmc.Latitude, rmc.Longitude, when, exts)
			gpxPositionsTotal.Inc()
		}
	}
}

func newGPXFile() (io.WriteCloser, error) {
	name := time.Now().UTC().Format("track-20060102-150405.gpx")
	log.Println("Creating new GPX track", name)
	gpxFilesCreatedTotal.Inc()
	return os.Create(name)
}
