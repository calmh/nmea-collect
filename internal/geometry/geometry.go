package geometry

import (
	"math"
)

// Distance returns the distance between the two points, in nautical miles.
func Distance(lat1, lon1, lat2, lon2 float64) float64 {
	lat1 *= math.Pi / 180
	lon1 *= math.Pi / 180
	lat2 *= math.Pi / 180
	lon2 *= math.Pi / 180
	cosD := math.Sin(lat1)*math.Sin(lat2) + math.Cos(lat1)*math.Cos(lat2)*math.Cos(lon2-lon1)
	if cosD > 1 {
		cosD = 1
	}
	return math.Acos(cosD) * 180 / math.Pi * 60
}

// Bearing returns the bearing from point 1 to point 2, in degrees.
func Bearing(lat1, lon1, lat2, lon2 float64) float64 {
	lat1 *= math.Pi / 180
	lon1 *= math.Pi / 180
	lat2 *= math.Pi / 180
	lon2 *= math.Pi / 180
	X := math.Cos(lat2) * math.Sin(lon2-lon1)
	Y := math.Cos(lat1)*math.Sin(lat2) - math.Sin(lat1)*math.Cos(lat2)*math.Cos(lon2-lon1)
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
