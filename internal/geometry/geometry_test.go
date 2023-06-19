package geometry

import (
	"testing"

	"calmh.dev/nmea-collect/internal/gpx/reader"
)

func TestCourse(t *testing.T) {
	cases := []struct {
		p0, p1 reader.GPXTrkPoint
		want   float64
	}{
		{reader.GPXTrkPoint{Lat: 0, Lon: 0}, reader.GPXTrkPoint{Lat: 0, Lon: 0}, 0},
		{reader.GPXTrkPoint{Lat: 0, Lon: 0}, reader.GPXTrkPoint{Lat: 0, Lon: 1}, 90},
		{reader.GPXTrkPoint{Lat: 0, Lon: 0}, reader.GPXTrkPoint{Lat: 1, Lon: 0}, 0},
		{reader.GPXTrkPoint{Lat: 0, Lon: 0}, reader.GPXTrkPoint{Lat: 1, Lon: 1}, 45},
		{reader.GPXTrkPoint{Lat: 0, Lon: 0}, reader.GPXTrkPoint{Lat: 1, Lon: -1}, 315},
		{reader.GPXTrkPoint{Lat: 0, Lon: 0}, reader.GPXTrkPoint{Lat: -1, Lon: 0}, 180},
		{reader.GPXTrkPoint{Lat: 0, Lon: 0}, reader.GPXTrkPoint{Lat: -1, Lon: 1}, 135},
		{reader.GPXTrkPoint{Lat: 0, Lon: 0}, reader.GPXTrkPoint{Lat: -1, Lon: -1}, 225},
	}

	for _, c := range cases {
		got := Course(c.p0, c.p1)
		if got < c.want-1 || got > c.want+1 {
			t.Errorf("course(%v, %v) == %f, want %f", c.p0, c.p1, got, c.want)
		}
	}
}
