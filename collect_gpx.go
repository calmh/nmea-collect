package main

import (
	"fmt"
	"strings"
	"time"

	nmea "github.com/adrianmo/go-nmea"
	"github.com/calmh/nmea-collect/gpx"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	gpxPositionsSampled = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "nmea",
		Subsystem: "gpx",
		Name:      "sampled_positions_total",
	})
	gpxPositionsRecorded = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "nmea",
		Subsystem: "gpx",
		Name:      "record_positions_total",
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

func collectGPX(c <-chan string, w *gpx.AutoGPX) {
	exts := make(gpx.Extensions)
	defer w.Flush()

	for line := range c {
		gpxInputMessages.Inc()
		sent, err := nmea.Parse(line)
		if err != nil {
			if strings.Contains(err.Error(), "not supported") {
				gpxUnsupportedMessages.Inc()
				continue
			}
			gpxBadMessages.Inc()
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
			gll := sent.(nmea.GLL)
			when := time.Now()
			if w.Sample(gll.Latitude, gll.Longitude, when, exts) {
				gpxPositionsRecorded.Inc()
			}
			gpxPositionsSampled.Inc()

		case nmea.TypeRMC:
			rmc := sent.(nmea.RMC)
			when := time.Now()
			if w.Sample(rmc.Latitude, rmc.Longitude, when, exts) {
				gpxPositionsRecorded.Inc()
			}
			gpxPositionsSampled.Inc()
		}
	}
}
