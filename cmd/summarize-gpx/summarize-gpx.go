package main

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"time"

	"github.com/alecthomas/kong"
)

var cli struct {
	Files             []string `arg:""`
	DeleteShorterThan float64  `placeholder:"DIST"`
	DeleteErrored     bool
}

func main() {
	kong.Parse(&cli)

	for _, f := range cli.Files {
		fd, err := os.Open(f)
		if err != nil {
			fmt.Printf("%s: %v\n", f, err)
			continue
		}
		points, err := points(fd)
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

type gpx struct {
	Tracks []struct {
		Segments []struct {
			Points []gpxTrkPoint `xml:"trkpt"`
		} `xml:"trkseg"`
	} `xml:"trk"`
}

type gpxTrkPoint struct {
	Lat        float64         `xml:"lat,attr"`
	Lon        float64         `xml:"lon,attr"`
	Time       time.Time       `xml:"time"`
	Extensions gpxExtensionSet `xml:"extensions"`
}

type gpxExtensionSet struct {
	Children []gpxExtension `xml:",any"`
}

func (e gpxExtensionSet) Named(name string) gpxExtension {
	for _, c := range e.Children {
		if c.XMLName.Local == name {
			return c
		}
	}
	return gpxExtension{}
}

type gpxExtension struct {
	XMLName xml.Name
	Value   float64 `xml:",chardata"`
}

func points(r io.Reader) ([][]gpxTrkPoint, error) {
	dec := xml.NewDecoder(r)
	var points [][]gpxTrkPoint
	for {
		var g gpx
		if err := dec.Decode(&g); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, err
		}
		for _, trk := range g.Tracks {
			for _, seg := range trk.Segments {
				points = append(points, seg.Points)
			}
		}
	}
	return points, nil
}

func summarize(tracks [][]gpxTrkPoint) summary {
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

		var prev *gpxTrkPoint
		for i, p := range points {
			if prev != nil {
				td := p.Time.Sub(prev.Time)
				dist := distance(*prev, p)
				s.tripDistance += dist
				s.sog.record(dist/td.Hours(), td)
				s.windSpeed.record(p.Extensions.Named("windspeed").Value*3600/1852, td)
				s.waterSpeed.record(p.Extensions.Named("waterspeed").Value, td)
			}
			prev = &points[i]
		}
	}

	return s
}

func printSummary(w io.Writer, s summary) {
	fmt.Fprintf(w, "Start: %v\nEnd:   %v\nDuration: %s\n", s.start.Local(), s.end.Local(), s.duration.Round(time.Minute))
	fmt.Fprintf(w, "Distance: %.01f nm\n", s.tripDistance)

	fmt.Fprintf(w, "SOG: %.1f kt avg (med %.1f kt, max %.1f kt)\n", s.sog.avg(), s.sog.med(), s.sog.max)
	fmt.Fprintf(w, "STW: %.1f kt avg (med %.1f kt, max %.1f kt)\n", s.waterSpeed.avg(), s.waterSpeed.med(), s.waterSpeed.max)
	fmt.Fprintf(w, "Wind: %.1f kt avg (%.1f kt med, %.1f kt min, %.1f kt max)\n", s.windSpeed.avg(), s.windSpeed.med(), s.windSpeed.min, s.windSpeed.max)
}

type summary struct {
	start        time.Time
	end          time.Time
	duration     time.Duration
	sog          metric
	windSpeed    metric
	waterSpeed   metric
	tripDistance float64
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

func distance(p0, p1 gpxTrkPoint) float64 {
	const PI float64 = 3.141592653589793
	radlat1 := float64(PI * p0.Lat / 180)
	radlat2 := float64(PI * p1.Lat / 180)
	theta := float64(p0.Lon - p1.Lon)
	radtheta := float64(PI * theta / 180)
	dist := math.Sin(radlat1)*math.Sin(radlat2) + math.Cos(radlat1)*math.Cos(radlat2)*math.Cos(radtheta)

	if dist > 1 {
		dist = 1
	}
	return math.Acos(dist) * 180 / PI * 60
}
