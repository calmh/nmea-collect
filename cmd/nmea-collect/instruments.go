package main

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	nmea "github.com/adrianmo/go-nmea"
	"github.com/calmh/nmea-collect/gpx"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	waterDepth = newLiveGauge(prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "nmea",
		Subsystem: "instruments",
		Name:      "water_depth_m",
	}))
	heading = newLiveGauge(prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "nmea",
		Subsystem: "instruments",
		Name:      "compass_heading",
	}))
	waterTemp = newLiveGauge(prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "nmea",
		Subsystem: "instruments",
		Name:      "water_temperature_c",
	}))
	windAngle = newLiveGauge(prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "nmea",
		Subsystem: "instruments",
		Name:      "apparent_wind_angle",
	}))
	windSpeed = newLiveGauge(prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "nmea",
		Subsystem: "instruments",
		Name:      "apparent_wind_speed_mps",
	}))
	windSpeedMed = newLiveGauge(prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "nmea",
		Subsystem: "instruments",
		Name:      "apparent_wind_speed_median_mps",
	}))
	windSpeedMax = newLiveGauge(prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "nmea",
		Subsystem: "instruments",
		Name:      "apparent_wind_speed_max_mps",
	}))
	windSpeedMin = newLiveGauge(prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "nmea",
		Subsystem: "instruments",
		Name:      "apparent_wind_speed_min_mps",
	}))
	logDistance = newLiveGauge(prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "nmea",
		Subsystem: "instruments",
		Name:      "total_log_distance_nm",
	}))
	logSpeed = newLiveGauge(prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "nmea",
		Subsystem: "instruments",
		Name:      "water_speed_kn",
	}))
	voltage = newLiveGauge(prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "nmea",
		Subsystem: "ais",
		Name:      "supply_voltage",
	}))
	airTemp = newLiveGauge(prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "nmea",
		Subsystem: "instruments",
		Name:      "air_temperature_c",
	}))
	insideTemp = newLiveGauge(prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "nmea",
		Subsystem: "instruments",
		Name:      "inside_temperature_c",
	}))
	baroPressure = newLiveGauge(prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "nmea",
		Subsystem: "instruments",
		Name:      "barometric_pressure_mb",
	}))

	position = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "nmea",
		Subsystem: "instruments",
		Name:      "gps_position",
	}, []string{"axis"})
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
	positionTimeout := time.NewTimer(instrumentRetention)
	defer positionTimeout.Stop()

	l.extMut.Lock()
	l.exts = make(gpx.Extensions)
	l.extMut.Unlock()

	var windspeed measurement
	windspeedSwitched := time.Now()

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
				l.extMut.Lock()
				l.exts.Set("waterdepth", fmt.Sprintf("%.01f", dpt.Depth))
				l.extMut.Unlock()

			case TypeHDG:
				hdg := sent.(HDG)
				heading.Set(hdg.Heading)
				l.extMut.Lock()
				l.exts.Set("heading", fmt.Sprintf("%.0f", hdg.Heading))
				l.extMut.Unlock()

			case TypeMTW:
				mtw := sent.(MTW)
				waterTemp.Set(mtw.Temperature)
				l.extMut.Lock()
				l.exts.Set("watertemp", fmt.Sprintf("%.01f", mtw.Temperature))
				l.extMut.Unlock()

			case TypeMWV:
				mwv := sent.(MWV)
				if mwv.Reference == "R" && mwv.Status == "A" {
					windAngle.Set(mwv.Angle)
					windSpeed.Set(mwv.Speed)

					period := time.Now().Truncate(time.Minute)
					if !period.Equal(windspeedSwitched) {
						windspeed.Finalize()
						windSpeedMax.Set(windspeed.Max())
						windSpeedMed.Set(windspeed.Median())
						windSpeedMin.Set(windspeed.Min())
						windspeed.Reset()
						windspeedSwitched = period
					}
					windspeed.Observe(mwv.Speed)

					l.extMut.Lock()
					l.exts.Set("windangle", fmt.Sprintf("%.0f", mwv.Angle))
					l.exts.Set("windspeed", fmt.Sprintf("%.01f", mwv.Speed))
					l.extMut.Unlock()
				}

			case TypeVLW:
				mwv := sent.(VLW)
				logDistance.Set(mwv.TotalDistanceNauticalMiles)
				l.extMut.Lock()
				l.exts.Set("log", fmt.Sprintf("%.1f", mwv.TotalDistanceNauticalMiles))
				l.extMut.Unlock()

			case nmea.TypeVHW:
				vhw := sent.(nmea.VHW)
				logSpeed.Set(vhw.SpeedThroughWaterKnots)
				l.extMut.Lock()
				l.exts.Set("waterspeed", fmt.Sprintf("%.01f", vhw.SpeedThroughWaterKnots))
				l.extMut.Unlock()

			case nmea.TypeRMC:
				rmc := sent.(nmea.RMC)
				position.WithLabelValues("lat").Set(rmc.Latitude)
				position.WithLabelValues("lon").Set(rmc.Longitude)
				positionTimeout.Reset(instrumentRetention)

			case TypeSMT:
				smt := sent.(SMT)
				voltage.Set(smt.SupplyVoltage)
				l.extMut.Lock()
				l.exts.Set("supplyvoltage", fmt.Sprintf("%.01f", smt.SupplyVoltage))
				l.extMut.Unlock()

			case TypeXDR:
				xdr := sent.(XDR)
				for _, m := range xdr.Measurements {
					if m.TransducerType == "C" && m.Name == "Air" {
						airTemp.Set(m.Value)
						l.extMut.Lock()
						l.exts.Set("airtemperature", fmt.Sprintf("%.01f", m.Value))
						l.extMut.Unlock()
					} else if m.TransducerType == "C" && m.Name == "ENV_INSIDE_T" {
						insideTemp.Set(m.Value)
						l.extMut.Lock()
						l.exts.Set("insidetemperature", fmt.Sprintf("%.01f", m.Value))
						l.extMut.Unlock()
					} else if m.TransducerType == "P" && m.Name == "Baro" {
						m.Value /= 100.0
						baroPressure.Set(m.Value)
						l.extMut.Lock()
						l.exts.Set("baropressure", fmt.Sprintf("%.01f", m.Value))
						l.extMut.Unlock()
					}
				}
			}

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

type measurement []float64

func (m *measurement) Observe(v float64) {
	*m = append(*m, v)
}

func (m *measurement) Reset() {
	*m = (*m)[:0]
}

func (m *measurement) Finalize() {
	sort.Float64s(*m)
}

func (m *measurement) Median() float64 {
	if len(*m) == 0 {
		return 0
	}
	return (*m)[len(*m)/2]
}

func (m *measurement) Max() float64 {
	if len(*m) == 0 {
		return 0
	}
	return (*m)[len(*m)-1]
}

func (m *measurement) Min() float64 {
	if len(*m) == 0 {
		return 0
	}
	return (*m)[0]
}

type liveGauge struct {
	gauge      prometheus.Gauge
	mut        sync.Mutex
	unregister *time.Timer
}

func newLiveGauge(gauge prometheus.Gauge) *liveGauge {
	return &liveGauge{
		gauge: gauge,
	}
}

const gaugeLifeTime = 5 * time.Second

func (g *liveGauge) Set(v float64) {
	g.gauge.Set(v)

	g.mut.Lock()
	defer g.mut.Unlock()

	if g.unregister == nil {
		_ = prometheus.Register(g.gauge)
		g.unregister = time.AfterFunc(gaugeLifeTime, func() {
			g.mut.Lock()
			defer g.mut.Unlock()
			prometheus.Unregister(g.gauge)
			g.unregister = nil
		})
	} else {
		g.unregister.Reset(gaugeLifeTime)
	}
}
