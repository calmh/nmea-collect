package geometry

import (
	"testing"
)

func TestBearing(t *testing.T) {
	cases := []struct {
		lat1, lon1 float64
		lat2, lon2 float64
		want       float64
	}{
		{0, 0, 0, 0, 0},
		{0, 0, 0, 1, 90},
		{0, 0, 1, 0, 0},
		{0, 0, 1, 1, 45},
		{0, 0, -1, 1, 135},
		{0, 0, -1, -1, 225},
		{0, 0, 1, -1, 315},
		{0, 0, 0, -1, 270},
		{0, 0, -1, 0, 180},
	}

	for _, c := range cases {
		got := Bearing(c.lat1, c.lon1, c.lat2, c.lon2)
		if got < c.want-1 || got > c.want+1 {
			t.Errorf("Course(%f, %f, %f, %f) == %f, want %f", c.lat1, c.lon1, c.lat2, c.lon2, got, c.want)
		}
	}
}

func TestDistance(t *testing.T) {
	cases := []struct {
		lat1, lon1 float64
		lat2, lon2 float64
		want       float64
	}{
		{0, 0, 0, 0, 0},
		{0, 0, 0, 1, 60},
		{0, 0, 1, 0, 60},
		{0, 0, 0, -1, 60},
		{0, 0, -1, 0, 60},
		{80, 90, 80, 91, 10},
	}

	for _, c := range cases {
		got := Distance(c.lat1, c.lon1, c.lat2, c.lon2)
		if got < c.want-1 || got > c.want+1 {
			t.Errorf("Distance(%f, %f, %f, %f) == %f, want %f", c.lat1, c.lon1, c.lat2, c.lon2, got, c.want)
		}
	}
}
