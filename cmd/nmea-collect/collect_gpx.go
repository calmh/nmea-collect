package main

import (
	"context"
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

type gpxCollector struct {
	c <-chan string
	w *gpx.AutoGPX
}

func collectGPX(c <-chan string, w *gpx.AutoGPX) *gpxCollector {
	return &gpxCollector{
		c: c,
		w: w,
	}
}

func (c *gpxCollector) String() string {
	return fmt.Sprintf("gpx-collector@%p", c)
}

func (c *gpxCollector) Serve(ctx context.Context) error {
	exts := make(gpx.Extensions)
	defer c.w.Flush()

	const rmcTimeoutInterval = 5 * time.Minute
	rmcTimeout := time.NewTimer(rmcTimeoutInterval)
	defer rmcTimeout.Stop()

	for {
		select {
		case line := <-c.c:
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

			case nmea.TypeRMC:
				rmcTimeout.Reset(rmcTimeoutInterval)
				rmc := sent.(nmea.RMC)
				when := time.Date(rmc.Date.YY+2000, time.Month(rmc.Date.MM), rmc.Date.DD, rmc.Time.Hour, rmc.Time.Minute, rmc.Time.Second, rmc.Time.Millisecond*int(time.Millisecond), time.UTC)
				if c.w.Sample(rmc.Latitude, rmc.Longitude, when, exts) {
					gpxPositionsRecorded.Inc()
				}
				gpxPositionsSampled.Inc()
			}

		case <-rmcTimeout.C:
			c.w.Flush()

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
