package main

import (
	"encoding/binary"
	"encoding/hex"
	"strconv"

	"github.com/adrianmo/go-nmea"
)

const (
	TypeDPT = "DPT"
	TypeHDG = "HDG"
	TypeMTW = "MTW"
	TypeMWV = "MWV"
	TypeVLW = "VLW"
	TypeSMT = "SMT"
	TypeXDR = "XDR"
	// Not sure why the next can't be juse DIN in $PCDIN, but this is how the NMEA parser looks for it.
	TypeDIN = "CDIN" // http://www.seasmart.net/pdf/SeaSmart_HTTP_Protocol_RevG_043012.pdf
)

var parsers = map[string]nmea.ParserFunc{
	TypeDPT: parseDPT,
	TypeHDG: parseHDG,
	TypeMTW: parseMTW,
	TypeMWV: parseMWV,
	TypeVLW: parseVLW,
	TypeSMT: parseSMT,
	TypeXDR: parseXDR,
	TypeDIN: parseDIN,
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

// Heading - Deviation & Variation
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

// Depth
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

// Wind speed and angle
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

// Distance through water
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

//  Voltage

type SMT struct {
	nmea.BaseSentence
	SupplyVoltage float64
}

func parseSMT(s nmea.BaseSentence) (nmea.Sentence, error) {
	p := nmea.NewParser(s)
	p.AssertType(TypeSMT)
	m := SMT{
		BaseSentence:  s,
		SupplyVoltage: p.Float64(3, "voltage") / 1000,
	}
	return m, p.Err()
}

//  Transducer reading

type XDR struct {
	nmea.BaseSentence
	Measurements []XDRMeasurement
}

type XDRMeasurement struct {
	TransducerType string
	Value          float64
	Unit           string
	Name           string
}

func parseXDR(s nmea.BaseSentence) (nmea.Sentence, error) {
	p := nmea.NewParser(s)
	p.AssertType(TypeXDR)
	m := XDR{
		BaseSentence: s,
	}

	for i := 0; i < len(p.Fields); i += 4 {
		m.Measurements = append(m.Measurements, XDRMeasurement{
			TransducerType: p.String(i+0, "transducer type"),
			Value:          p.Float64(i+1, "measurement"),
			Unit:           p.String(i+2, "unit"),
			Name:           p.String(i+3, "name"),
		})
	}

	return m, p.Err()
}

type DIN struct {
	nmea.BaseSentence
	PGN       string
	Timestamp int
	Source    int
	Data      []byte
}

func (d DIN) BatteryVoltage() float64 {
	if d.PGN == "01F214" {
		v := binary.LittleEndian.Uint16(d.Data[1:])
		return float64(v) / 100
	}
	return 0
}

func parseDIN(s nmea.BaseSentence) (nmea.Sentence, error) {
	p := nmea.NewParser(s)
	p.AssertType(TypeDIN)
	m := DIN{
		BaseSentence: s,
		PGN:          p.String(0, "PGN"),
		Timestamp:    hexInt(p.String(1, "timestamp")),
		Source:       hexInt(p.String(2, "source")),
		Data:         hexBytes(p.String(3, "data")),
	}
	return m, p.Err()
}

func hexInt(s string) int {
	i, err := strconv.ParseInt(s, 16, 64)
	if err != nil {
		return 0
	}
	return int(i)
}

func hexBytes(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil
	}
	return b
}
