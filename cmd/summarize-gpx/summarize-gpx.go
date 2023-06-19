package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"calmh.dev/nmea-collect/internal/geometry"
	"calmh.dev/nmea-collect/internal/gpx/reader"
	"github.com/alecthomas/kong"
)

var cli struct {
	Files             []string `arg:""`
	DeleteShorterThan float64  `placeholder:"DIST"`
	DeleteErrored     bool
}

const waypointInterval = time.Hour

func main() {
	kong.Parse(&cli)

	for _, f := range cli.Files {
		fd, err := os.Open(f)
		if err != nil {
			fmt.Printf("%s: %v\n", f, err)
			continue
		}
		points, err := reader.Points(fd)
		if err != nil {
			fd.Close()
			fmt.Printf("%s: %v\n", f, err)
			if cli.DeleteErrored {
				os.Remove(f)
			}
			continue
		}
		fd.Close()

		sum := summarize(points)
		if cli.DeleteShorterThan > 0 && sum.tripDistance < cli.DeleteShorterThan {
			fmt.Printf("%s: empty\n", f)
			os.Remove(f)
			continue
		}

		fmt.Printf("%s\n", f)
		printSummary(os.Stdout, sum)
		fmt.Printf("\n")
	}
}

func summarize(tracks [][]reader.GPXTrkPoint) summary {
	var s summary

	for _, points := range tracks {
		start := points[0]
		last := points[len(points)-1]

		if s.start.IsZero() || s.start.After(start.Time) {
			s.start = start.Time
		}
		if s.end.IsZero() || s.end.Before(last.Time) {
			s.end = last.Time
		}
		s.duration += last.Time.Sub(start.Time)

		var prev *reader.GPXTrkPoint
		var p reader.GPXTrkPoint
		for i := range points {
			p = points[i]
			if prev != nil {
				td := p.Time.Sub(prev.Time)
				dist := geometry.Distance(prev.Lat, prev.Lon, p.Lat, p.Lon)
				cog := geometry.Bearing(prev.Lat, prev.Lon, p.Lat, p.Lon)
				s.tripDistance += dist
				s.sog.record(dist/td.Hours(), td)
				s.windSpeed.record(p.Extensions.Named("windspeed").Value, td)
				s.waterSpeed.record(p.Extensions.Named("waterspeed").Value, td)

				if prev.Time.Truncate(waypointInterval) != p.Time.Truncate(waypointInterval) {
					s.waypoints = append(s.waypoints, waypoint{
						time:      p.Time,
						log:       p.Extensions.Named("log").Value,
						trip:      s.tripDistance,
						sog:       s.sog.avg(),
						windSpeed: p.Extensions.Named("windspeed").Value,
						windDir:   int(p.Extensions.Named("heading").Value+p.Extensions.Named("windangle").Value) % 360,
						cog:       cog,
					})
				}
			} else {
				s.waypoints = append(s.waypoints, waypoint{
					time:      p.Time,
					log:       p.Extensions.Named("log").Value,
					windSpeed: p.Extensions.Named("windspeed").Value,
					windDir:   int(p.Extensions.Named("heading").Value+p.Extensions.Named("windangle").Value) % 360,
				})
			}
			prev = &points[i]
		}
		s.waypoints = append(s.waypoints, waypoint{
			time:      p.Time,
			log:       p.Extensions.Named("log").Value,
			trip:      s.tripDistance,
			windSpeed: p.Extensions.Named("windspeed").Value,
			windDir:   int(p.Extensions.Named("heading").Value+p.Extensions.Named("windangle").Value) % 360,
		})
	}

	return s
}

func printSummary(w io.Writer, s summary) {
	fmt.Fprintf(w, "Start: %v\nEnd:   %v\nDuration: %s\n", s.start.Local(), s.end.Local(), s.duration.Round(time.Minute))
	fmt.Fprintf(w, "Distance: %.01f nm\n", s.tripDistance)

	fmt.Fprintf(w, "SOG: %.1f kt avg (med %.1f kt, max %.1f kt)\n", s.sog.avg(), s.sog.med(), s.sog.max)
	fmt.Fprintf(w, "STW: %.1f kt avg (med %.1f kt, max %.1f kt)\n", s.waterSpeed.avg(), s.waterSpeed.med(), s.waterSpeed.max)
	fmt.Fprintf(w, "Wind: %.1f m/s avg (%.1f m/s med, %.1f m/s min, %.1f m/s max)\n", s.windSpeed.avg(), s.windSpeed.med(), s.windSpeed.min, s.windSpeed.max)

	fmt.Fprintf(w, "\nWaypoints:\n")
	for _, wp := range s.waypoints {
		fmt.Fprintf(w, "%s @ %.1f nm (+%.1f nm)\n", wp.time.Local().Format("2/1 15:04"), wp.log, wp.trip)
		fmt.Fprintf(w, "\tWind: %s %.1f m/s\n", geometry.CardinalDirection(wp.windDir), wp.windSpeed)
		if wp.sog > 0 {
			fmt.Fprintf(w, "\tSOG: %.1f kt\n\tCOG: %.0f\n", wp.sog, wp.cog)
		}
	}
}

type summary struct {
	start        time.Time
	end          time.Time
	duration     time.Duration
	sog          metric
	windSpeed    metric
	waterSpeed   metric
	tripDistance float64
	waypoints    []waypoint
}

type waypoint struct {
	time      time.Time
	log       float64
	trip      float64
	sog       float64
	cog       float64
	windSpeed float64
	windDir   int
}

type metric struct {
	sum      float64
	dur      time.Duration
	min      float64
	max      float64
	all      []float64
	notFirst bool
}

func (m *metric) record(val float64, dur time.Duration) {
	m.sum += val * dur.Seconds()
	m.dur += dur
	m.all = append(m.all, val)
	if !m.notFirst {
		m.max = val
		m.min = val
		m.notFirst = true
	} else {
		if val > m.max {
			m.max = val
		}
		if val < m.min {
			m.min = val
		}
	}
}

func (m *metric) avg() float64 {
	return m.sum / m.dur.Seconds()
}

func (m *metric) med() float64 {
	sort.Float64s(m.all)
	return m.all[len(m.all)/2]
}
