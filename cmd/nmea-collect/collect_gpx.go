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
	i *instrumentsCollector
}

func collectGPX(c <-chan string, w *gpx.AutoGPX, i *instrumentsCollector) *gpxCollector {
	return &gpxCollector{
		c: c,
		w: w,
		i: i,
	}
}

func (c *gpxCollector) String() string {
	return fmt.Sprintf("gpx-collector@%p", c)
}

func (c *gpxCollector) Serve(ctx context.Context) error {
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
			case nmea.TypeRMC:
				rmcTimeout.Reset(rmcTimeoutInterval)
				rmc := sent.(nmea.RMC)
				when := time.Date(rmc.Date.YY+2000, time.Month(rmc.Date.MM), rmc.Date.DD, rmc.Time.Hour, rmc.Time.Minute, rmc.Time.Second, rmc.Time.Millisecond*int(time.Millisecond), time.UTC)
				if c.w.Sample(rmc.Latitude, rmc.Longitude, when, c.i.GPXExtensions()) {
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
