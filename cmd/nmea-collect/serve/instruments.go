package serve

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"net/http"
	"sort"
	"sync"
	"time"

	"calmh.dev/nmea-collect/internal/gpx/writer"
	nmea "github.com/adrianmo/go-nmea"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
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
	batteryVoltage = newLiveGauge(prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "nmea",
		Subsystem: "instruments",
		Name:      "battery_voltage",
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

	position = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "nmea",
		Subsystem: "instruments",
		Name:      "gps_position",
	}, []string{"axis"})
)

type instrumentsCollector struct {
	c      <-chan string
	exts   writer.Extensions
	extMut sync.Mutex
}

func (l *instrumentsCollector) String() string {
	return fmt.Sprintf("instruments-collector@%p", l)
}

func (l *instrumentsCollector) Serve(ctx context.Context) error {
	const instrumentRetention = time.Minute
	positionTimeout := time.NewTimer(instrumentRetention)
	defer positionTimeout.Stop()
	positionRegistered := false
	defer func() {
		if positionRegistered {
			prometheus.Unregister(position)
		}
	}()

	l.extMut.Lock()
	l.exts = make(writer.Extensions)
	l.extMut.Unlock()

	windSpeedOverTime := measurement{period: time.Minute}

	for {
		select {
		case line := <-l.c:
			sent, err := nmea.Parse(line)
			if err != nil {
				continue
			}

			switch sent.DataType() {
			case nmea.TypeDPT:
				dpt := sent.(nmea.DPT)
				waterDepth.Set(dpt.Depth)
				l.extMut.Lock()
				l.exts.Set("waterdepth", fmt.Sprintf("%.01f", dpt.Depth))
				l.extMut.Unlock()

			case nmea.TypeHDG:
				hdg := sent.(nmea.HDG)
				heading.Set(hdg.Heading)
				l.extMut.Lock()
				l.exts.Set("heading", fmt.Sprintf("%.0f", hdg.Heading))
				l.extMut.Unlock()

			case nmea.TypeMTW:
				mtw := sent.(nmea.MTW)
				waterTemp.Set(mtw.Temperature)
				l.extMut.Lock()
				l.exts.Set("watertemp", fmt.Sprintf("%.01f", mtw.Temperature))
				l.extMut.Unlock()

			case nmea.TypeMWV:
				mwv := sent.(nmea.MWV)
				if mwv.Reference == "R" && mwv.StatusValid {
					windAngle.Set(mwv.WindAngle)
					windSpeed.Set(mwv.WindSpeed)

					windSpeedOverTime.Observe(mwv.WindSpeed)
					min, med, max := windSpeedOverTime.MinMedianMax()
					windSpeedMin.Set(min)
					windSpeedMed.Set(med)
					windSpeedMax.Set(max)

					l.extMut.Lock()
					l.exts.Set("windangle", fmt.Sprintf("%.0f", mwv.WindAngle))
					l.exts.Set("windspeed", fmt.Sprintf("%.01f", mwv.WindSpeed))
					l.extMut.Unlock()
				}

			case nmea.TypeVLW:
				mwv := sent.(nmea.VLW)
				logDistance.Set(mwv.TotalInWater)
				l.extMut.Lock()
				l.exts.Set("log", fmt.Sprintf("%.1f", mwv.TotalInWater))
				l.extMut.Unlock()

			case nmea.TypeVHW:
				vhw := sent.(nmea.VHW)
				logSpeed.Set(vhw.SpeedThroughWaterKnots)
				l.extMut.Lock()
				l.exts.Set("waterspeed", fmt.Sprintf("%.01f", vhw.SpeedThroughWaterKnots))
				l.extMut.Unlock()

			case nmea.TypeGLL:
				gll := sent.(nmea.GLL)
				if gll.Validity == "A" {
					position.WithLabelValues("lat").Set(gll.Latitude)
					position.WithLabelValues("lon").Set(gll.Longitude)
					if !positionRegistered {
						prometheus.Register(position)
						positionRegistered = true
					}
					positionTimeout.Reset(instrumentRetention)
				}

			case nmea.TypeXDR:
				xdr := sent.(nmea.XDR)
				for _, m := range xdr.Measurements {
					if m.TransducerType == "C" && m.TransducerName == "Air" {
						airTemp.Set(m.Value)
						l.extMut.Lock()
						l.exts.Set("airtemperature", fmt.Sprintf("%.01f", m.Value))
						l.extMut.Unlock()
					} else if m.TransducerType == "C" && m.TransducerName == "ENV_INSIDE_T" {
						insideTemp.Set(m.Value)
						l.extMut.Lock()
						l.exts.Set("insidetemperature", fmt.Sprintf("%.01f", m.Value))
						l.extMut.Unlock()
					} else if m.TransducerType == "P" && m.TransducerName == "Baro" {
						m.Value /= 100.0
						baroPressure.Set(m.Value)
						l.extMut.Lock()
						l.exts.Set("baropressure", fmt.Sprintf("%.01f", m.Value))
						l.extMut.Unlock()
					}
				}

			case nmea.TypePCDIN:
				din := sent.(nmea.PCDIN)
				v := pcdinBatteryVoltage(din)
				if v > 0 {
					batteryVoltage.Set(v)
					l.extMut.Lock()
					l.exts.Set("batteryvoltage", fmt.Sprintf("%.01f", v))
					l.extMut.Unlock()
				}
			}

		case <-positionTimeout.C:
			if positionRegistered {
				prometheus.Unregister(position)
				positionRegistered = false
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (i *instrumentsCollector) GPXExtensions() writer.Extensions {
	i.extMut.Lock()
	defer i.extMut.Unlock()
	copy := make(writer.Extensions, len(i.exts))
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

	list, err := net.Listen("tcp", l.addr)
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		list.Close()
	}()

	return http.Serve(list, mux)
}

type measurement struct {
	values []value
	period time.Duration
}

type value struct {
	t time.Time
	v float64
}

func (m *measurement) Observe(v float64) {
	m.values = append(m.values, value{time.Now(), v})
	for len(m.values) > 0 && time.Since(m.values[0].t) > m.period {
		m.values = m.values[1:]
	}
}

func (m *measurement) sortedValues() []float64 {
	var values []float64
	for _, v := range m.values {
		values = append(values, v.v)
	}
	sort.Float64s(values)
	return values
}

func (m *measurement) MinMedianMax() (float64, float64, float64) {
	values := m.sortedValues()
	if len(values) == 0 {
		return 0, 0, 0
	}
	return values[0], values[len(values)/2], values[len(values)-1]
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

func pcdinBatteryVoltage(d nmea.PCDIN) float64 {
	if d.PGN == 0x1F214 {
		v := binary.LittleEndian.Uint16(d.Data[1:])
		return float64(v) / 100
	}
	return 0
}
