package main

import (
	nmea "github.com/adrianmo/go-nmea"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
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
)

func exposeMetrics(c <-chan string) {
	for line := range c {
		sent, err := nmea.Parse(line)
		if err != nil {
			continue
		}

		switch sent.DataType() {
		case TypeDPT:
			dpt := sent.(DPT)
			waterDepth.Set(dpt.Depth)

		case TypeHDG:
			hdg := sent.(HDG)
			heading.Set(hdg.Heading)

		case TypeMTW:
			mtw := sent.(MTW)
			waterTemp.Set(mtw.Temperature)

		case TypeMWV:
			mwv := sent.(MWV)
			if mwv.Reference == "R" && mwv.Status == "A" {
				windAngle.Set(mwv.Angle)
				windSpeed.Set(mwv.Speed)
			}

		case TypeVLW:
			mwv := sent.(VLW)
			logDistance.Set(mwv.TotalDistanceNauticalMiles)

		case nmea.TypeVHW:
			vhw := sent.(nmea.VHW)
			logSpeed.Set(vhw.SpeedThroughWaterKnots)

		case nmea.TypeGLL:
			rmc := sent.(nmea.GLL)
			position.WithLabelValues("lat").Set(rmc.Latitude)
			position.WithLabelValues("lon").Set(rmc.Longitude)
		}
	}
}
