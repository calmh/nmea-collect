package main

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	nmea "github.com/adrianmo/go-nmea"
	"github.com/calmh/nmea-collect/gpx"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	waterDepth = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "nmea",
		Subsystem: "instruments",
		Name:      "water_depth_m",
	})
	heading = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "nmea",
		Subsystem: "instruments",
		Name:      "compass_heading",
	})
	waterTemp = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "nmea",
		Subsystem: "instruments",
		Name:      "water_temperature_c",
	})
	windAngle = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "nmea",
		Subsystem: "instruments",
		Name:      "apparent_wind_angle",
	})
	windSpeed = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "nmea",
		Subsystem: "instruments",
		Name:      "apparent_wind_speed_mps",
	})
	logDistance = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "nmea",
		Subsystem: "instruments",
		Name:      "total_log_distance_nm",
	})
	logSpeed = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "nmea",
		Subsystem: "instruments",
		Name:      "water_speed_kn",
	})
	position = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "nmea",
		Subsystem: "instruments",
		Name:      "gps_position",
	}, []string{"axis"})
	voltage = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "nmea",
		Subsystem: "ais",
		Name:      "supply_voltage",
	})
)

type instrumentsCollector struct {
	c      <-chan string
	exts   gpx.Extensions
	extMut sync.Mutex
}

func (l *instrumentsCollector) String() string {
	return fmt.Sprintf("instruments-collector@%p", l)
}

func (l *instrumentsCollector) Serve(ctx context.Context) error {
	const instrumentRetention = time.Minute
	instrumentTimeout := time.NewTimer(instrumentRetention)
	defer instrumentTimeout.Stop()
	positionTimeout := time.NewTimer(instrumentRetention)
	defer positionTimeout.Stop()

	l.extMut.Lock()
	l.exts = make(gpx.Extensions)
	l.extMut.Unlock()

	for {
		select {
		case line := <-l.c:
			sent, err := nmea.Parse(line)
			if err != nil {
				continue
			}

			switch sent.DataType() {
			case TypeDPT:
				dpt := sent.(DPT)
				waterDepth.Set(dpt.Depth)
				instrumentTimeout.Reset(instrumentRetention)
				l.extMut.Lock()
				l.exts.Set("waterdepth", fmt.Sprintf("%.01f", dpt.Depth))
				l.extMut.Unlock()

			case TypeHDG:
				hdg := sent.(HDG)
				heading.Set(hdg.Heading)
				instrumentTimeout.Reset(instrumentRetention)
				l.extMut.Lock()
				l.exts.Set("heading", fmt.Sprintf("%.0f", hdg.Heading))
				l.extMut.Unlock()

			case TypeMTW:
				mtw := sent.(MTW)
				waterTemp.Set(mtw.Temperature)
				instrumentTimeout.Reset(instrumentRetention)
				l.extMut.Lock()
				l.exts.Set("watertemp", fmt.Sprintf("%.01f", mtw.Temperature))
				l.extMut.Unlock()

			case TypeMWV:
				mwv := sent.(MWV)
				if mwv.Reference == "R" && mwv.Status == "A" {
					windAngle.Set(mwv.Angle)
					windSpeed.Set(mwv.Speed)
					instrumentTimeout.Reset(instrumentRetention)
					l.extMut.Lock()
					l.exts.Set("windangle", fmt.Sprintf("%.0f", mwv.Angle))
					l.exts.Set("windspeed", fmt.Sprintf("%.01f", mwv.Speed))
					l.extMut.Unlock()
				}

			case TypeVLW:
				mwv := sent.(VLW)
				logDistance.Set(mwv.TotalDistanceNauticalMiles)
				instrumentTimeout.Reset(instrumentRetention)
				l.extMut.Lock()
				l.exts.Set("log", fmt.Sprintf("%.1f", mwv.TotalDistanceNauticalMiles))
				l.extMut.Unlock()

			case nmea.TypeVHW:
				vhw := sent.(nmea.VHW)
				logSpeed.Set(vhw.SpeedThroughWaterKnots)
				instrumentTimeout.Reset(instrumentRetention)
				l.extMut.Lock()
				l.exts.Set("waterspeed", fmt.Sprintf("%.01f", vhw.SpeedThroughWaterKnots))
				l.extMut.Unlock()

			case nmea.TypeRMC:
				rmc := sent.(nmea.RMC)
				position.WithLabelValues("lat").Set(rmc.Latitude)
				position.WithLabelValues("lon").Set(rmc.Longitude)
				positionTimeout.Reset(instrumentRetention)

			case TypeSMT:
				rmc := sent.(SMT)
				voltage.Set(rmc.SupplyVoltage)
				l.extMut.Lock()
				l.exts.Set("supplyvoltage", fmt.Sprintf("%.01f", rmc.SupplyVoltage))
				l.extMut.Unlock()
			}

		case <-instrumentTimeout.C:
			waterDepth.Set(0)
			heading.Set(0)
			waterTemp.Set(0)
			windAngle.Set(0)
			windSpeed.Set(0)
			logDistance.Set(0)
			logSpeed.Set(0)

		case <-positionTimeout.C:
			position.WithLabelValues("lat").Set(0)
			position.WithLabelValues("lon").Set(0)

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (i *instrumentsCollector) GPXExtensions() gpx.Extensions {
	i.extMut.Lock()
	defer i.extMut.Unlock()
	copy := make(gpx.Extensions, len(i.exts))
	for k, v := range i.exts {
		copy[k] = v
	}
	return copy
}

type prometheusListener struct {
	addr string
}

func (l *prometheusListener) String() string {
	return fmt.Sprintf("prometheus-listener(%s)@%p", l.addr, l)
}

func (l *prometheusListener) Serve(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.Handle("/", promhttp.Handler())
	return http.ListenAndServe(l.addr, mux)
}
