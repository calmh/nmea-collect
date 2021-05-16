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
)

func main() {
	points, err := points(os.Stdin)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	summarize(os.Stdout, points)
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

func summarize(w io.Writer, tracks [][]gpxTrkPoint) {
	var sog, windSpeed, waterSpeed metric

	for _, points := range tracks {
		start := points[0]
		last := points[len(points)-1]
		td := last.Time.Sub(start.Time)

		fmt.Fprintf(w, "Start: %v\nEnd:   %v\nDuration: %s\n", start.Time.Local(), last.Time.Local(), td.Round(time.Minute))

		var prev *gpxTrkPoint
		var tripDistance float64
		for i, p := range points {
			if prev != nil {
				td := p.Time.Sub(prev.Time)
				dist := distance(*prev, p)
				tripDistance += dist
				sog.record(dist/td.Hours(), td)
				windSpeed.record(p.Extensions.Named("windspeed").Value*3600/1852, td)
				waterSpeed.record(p.Extensions.Named("waterspeed").Value, td)
			}
			prev = &points[i]
		}

		fmt.Fprintf(w, "Distance: %.1f NM\n---\n", tripDistance)
	}

	fmt.Fprintf(w, "SOG: %.1f kt avg (med %.1f kt, max %.1f kt)\n", sog.avg(), sog.med(), sog.max)
	fmt.Fprintf(w, "STW: %.1f kt avg (med %.1f kt, max %.1f kt)\n", waterSpeed.avg(), waterSpeed.med(), waterSpeed.max)
	fmt.Fprintf(w, "Wind: %.1f kt avg (%.1f kt med, %.1f kt min, %.1f kt max)\n", windSpeed.avg(), windSpeed.med(), windSpeed.min, windSpeed.max)
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
