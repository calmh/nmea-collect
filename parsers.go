package main

import (
	"github.com/adrianmo/go-nmea"
)

const (
	TypeDPT = "DPT"
	TypeHDG = "HDG"
	TypeMTW = "MTW"
	TypeMWV = "MWV"
	TypeVLW = "VLW"
)

var parsers = map[string]nmea.ParserFunc{
	TypeDPT: parseDPT,
	TypeHDG: parseHDG,
	TypeMTW: parseMTW,
	TypeMWV: parseMWV,
	TypeVLW: parseVLW,
}

// Mean Temperature of Water
type MTW struct {
	nmea.BaseSentence
	Temperature float64
	Unit        string
}

func parseMTW(s nmea.BaseSentence) (nmea.Sentence, error) {
	p := nmea.NewParser(s)
	p.AssertType(TypeMTW)
	m := MTW{
		BaseSentence: s,
		Temperature:  p.Float64(0, "temperature"),
		Unit:         p.String(1, "unit"),
	}
	return m, p.Err()
}

//  Heading - Deviation & Variation
type HDG struct {
	nmea.BaseSentence
	Heading            float64
	Deviation          float64
	DeviationDirection string
	Variation          float64
	VariationDirection string
}

func parseHDG(s nmea.BaseSentence) (nmea.Sentence, error) {
	p := nmea.NewParser(s)
	p.AssertType(TypeHDG)
	m := HDG{
		BaseSentence:       s,
		Heading:            p.Float64(0, "heading"),
		Deviation:          p.Float64(1, "deviation"),
		DeviationDirection: p.String(2, "deviation direction"),
		Variation:          p.Float64(3, "deviation"),
		VariationDirection: p.String(4, "deviation direction"),
	}
	return m, p.Err()
}

//  Depth
type DPT struct {
	nmea.BaseSentence
	Depth float64
}

func parseDPT(s nmea.BaseSentence) (nmea.Sentence, error) {
	p := nmea.NewParser(s)
	p.AssertType(TypeDPT)
	m := DPT{
		BaseSentence: s,
		Depth:        p.Float64(0, "depth"),
	}
	return m, p.Err()
}

//  Wind speed and angle
type MWV struct {
	nmea.BaseSentence
	Angle     float64
	Reference string
	Speed     float64
	SpeedUnit string
	Status    string
}

func parseMWV(s nmea.BaseSentence) (nmea.Sentence, error) {
	p := nmea.NewParser(s)
	p.AssertType(TypeMWV)
	m := MWV{
		BaseSentence: s,
		Angle:        p.Float64(0, "angle"),
		Reference:    p.String(1, "reference"),
		Speed:        p.Float64(2, "speed"),
		SpeedUnit:    p.String(3, "unit"),
		Status:       p.String(4, "status"),
	}
	if m.Angle > 180 {
		m.Angle = -(360 - m.Angle)
	}
	return m, p.Err()
}

//  Distance through water
type VLW struct {
	nmea.BaseSentence
	TotalDistanceNauticalMiles      float64
	DistancesinceResetNauticalMiles float64
}

func parseVLW(s nmea.BaseSentence) (nmea.Sentence, error) {
	p := nmea.NewParser(s)
	p.AssertType(TypeVLW)
	m := VLW{
		BaseSentence:                    s,
		TotalDistanceNauticalMiles:      p.Float64(0, "total distance"),
		DistancesinceResetNauticalMiles: p.Float64(2, "distance since reset"),
	}
	return m, p.Err()
}
