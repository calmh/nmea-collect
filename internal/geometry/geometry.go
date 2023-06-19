package geometry

import (
	"math"

	"calmh.dev/nmea-collect/internal/gpx/reader"
)

func Distance(p0, p1 reader.GPXTrkPoint) float64 {
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

func Course(p0, p1 reader.GPXTrkPoint) float64 {
	// β = atan2(X,Y),
	// X = cos θb * sin ∆L
	// Y = cos θa * sin θb – sin θa * cos θb * cos ∆L
	p0.Lat *= math.Pi / 180
	p0.Lon *= math.Pi / 180
	p1.Lat *= math.Pi / 180
	p1.Lon *= math.Pi / 180
	X := math.Cos(p1.Lat) * math.Sin(p1.Lon-p0.Lon)
	Y := math.Cos(p0.Lat)*math.Sin(p1.Lat) - math.Sin(p0.Lat)*math.Cos(p1.Lat)*math.Cos(p1.Lon-p0.Lon)
	v := math.Atan2(X, Y) / math.Pi * 180
	if v < 0 {
		v += 360
	}
	return v
}

func CardinalDirection(degrees int) string {
	if degrees < 23 {
		return "N"
	} else if degrees < 68 {
		return "NE"
	} else if degrees < 113 {
		return "E"
	} else if degrees < 158 {
		return "SE"
	} else if degrees < 203 {
		return "S"
	} else if degrees < 248 {
		return "SW"
	} else if degrees < 293 {
		return "W"
	} else if degrees < 338 {
		return "NW"
	} else {
		return "N"
	}
}
