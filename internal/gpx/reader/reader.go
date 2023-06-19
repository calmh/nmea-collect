package reader

import (
	"encoding/xml"
	"errors"
	"io"
	"time"
)

type GPX struct {
	Tracks []struct {
		Segments []struct {
			Points []GPXTrkPoint `xml:"trkpt"`
		} `xml:"trkseg"`
	} `xml:"trk"`
}

type GPXTrkPoint struct {
	Lat        float64         `xml:"lat,attr"`
	Lon        float64         `xml:"lon,attr"`
	Time       time.Time       `xml:"time"`
	Extensions GPXExtensionSet `xml:"extensions"`
}

type GPXExtensionSet struct {
	Children []GPXExtension `xml:",any"`
}

func (e GPXExtensionSet) Named(name string) GPXExtension {
	for _, c := range e.Children {
		if c.XMLName.Local == name {
			return c
		}
	}
	return GPXExtension{}
}

type GPXExtension struct {
	XMLName xml.Name
	Value   float64 `xml:",chardata"`
}

func Points(r io.Reader) ([][]GPXTrkPoint, error) {
	dec := xml.NewDecoder(r)
	var points [][]GPXTrkPoint
	for {
		var g GPX
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
