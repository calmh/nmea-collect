package main

import (
	"github.com/adrianmo/go-nmea"
)

const (
	TypeMTW = "MTW"
	TypeHDG = "HDG"
)

var parsers = map[string]nmea.ParserFunc{
	TypeMTW: parseMTW,
	TypeHDG: parseHDG,
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
