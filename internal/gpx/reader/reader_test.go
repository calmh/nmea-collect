package reader

import (
	"os"
	"testing"
)

func TestParse(t *testing.T) {
	fd, err := os.Open("testdata/track-20210516-123356.gpx")
	if err != nil {
		t.Fatal(err)
	}
	points, err := Points(fd)
	if err != nil {
		t.Fatal(err)
	}
	if len(points) == 0 {
		t.Fatal("no points")
	}
	t.Log(points)
}
