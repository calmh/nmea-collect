package gpx

import (
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"math"
	"sort"
	"strings"
	"time"
)

type AutoGPX struct {
	Opener                func() (io.WriteCloser, error)
	SampleInterval        time.Duration
	TriggerDistanceMeters float64
	TriggerTimeWindow     time.Duration
	CooldownTimeWindow    time.Duration

	samples     []sample
	destination io.WriteCloser
}

type sample struct {
	lat, lon   float64
	when       time.Time
	extensions map[string]string
}

func (s sample) gpx() string {
	ext := ""
	if len(s.extensions) > 0 {
		var exts []string
		for k, v := range s.extensions {
			exts = append(exts, fmt.Sprintf("<nmea:%s>%s</nmea:%s>", k, v, k))
		}
		sort.Strings(exts)
		ext = fmt.Sprintf("<extensions>%s</extensions>", strings.Join(exts, ""))
	}
	return fmt.Sprintf(`<trkpt lat="%f" lon="%f"><time>%s</time>%s</trkpt>`, s.lat, s.lon, s.when.Format(time.RFC3339), ext)
}

func (g *AutoGPX) Sample(lat, lon float64, when time.Time, extensions map[string]string) bool {
	s := sample{lat, lon, when, extensions}
	// If this is the first sample, keep it and return.
	if len(g.samples) == 0 {
		g.samples = append(g.samples, s)
		return true
	}

	// If the latest sample is still within the sample interval, ignore this one.
	if when.Sub(g.samples[len(g.samples)-1].when) < g.SampleInterval {
		return false
	}

	g.samples = append(g.samples, s)

	if g.destination == nil {
		// Clean out samples older than the trigger time window.
		keep := g.oldestNewerThanIdx(when.Add(-g.TriggerTimeWindow))
		g.samples = g.samples[keep:]

		// Check if we've moved far enough to start recording.
		d := distance(g.samples[0], s)
		if d > g.TriggerDistanceMeters {
			g.startRecording()
		}

		return true
	}

	if old, ok := g.latestOlderThan(when.Add(-g.CooldownTimeWindow)); ok && distance(old, s) < g.TriggerDistanceMeters {
		g.stopRecording()
		return true
	}

	g.record(s)

	// Clean out samples older than the cooldown time window.
	keep := g.oldestNewerThanIdx(when.Add(-g.CooldownTimeWindow))
	keep--
	if keep > 0 {
		g.samples = g.samples[keep:]
	}

	return true
}

func (g *AutoGPX) startRecording() {
	fd, err := g.Opener()
	if err != nil {
		log.Println("Opening file:", err)
		return
	}
	g.destination = fd

	header := `<gpx xmlns="http://www.topografix.com/GPX/1/1"><trk><trkseg>`
	if _, err := fmt.Fprintln(g.destination, header); err != nil {
		log.Println("Writing to file:", err)
		return
	}
	for _, s := range g.samples {
		if _, err := fmt.Fprintln(g.destination, s.gpx()); err != nil {
			log.Println("Writing to file:", err)
			return
		}
	}
}

func (g *AutoGPX) record(s sample) {
	if _, err := fmt.Fprintln(g.destination, s.gpx()); err != nil {
		log.Println("Writing to file:", err)
	}
}

func (g *AutoGPX) stopRecording() {
	footer := `</trkseg></trk></gpx>`
	if _, err := fmt.Fprintln(g.destination, footer); err != nil {
		log.Println("Writing to file:", err)
	}
	if err := g.destination.Close(); err != nil {
		log.Println("Closing file:", err)
	}
	g.destination = nil
	g.samples = nil
}

// latestOlderThan returns the newest sample older than t
func (g *AutoGPX) latestOlderThan(t time.Time) (sample, bool) {
	idx := g.oldestNewerThanIdx(t)
	if idx < 1 {
		return sample{}, false
	}
	return g.samples[idx-1], true
}

// oldestNewerThanIdx returns the index of the oldest sample newer than t
func (g *AutoGPX) oldestNewerThanIdx(t time.Time) int {
	return sort.Search(len(g.samples), func(i int) bool {
		return g.samples[i].when.After(t)
	})
}

// distance between two samples, in meters
func distance(s1, s2 sample) float64 {
	d1 := math.Abs(s1.lat - s2.lat)
	d2 := math.Abs(s1.lon - s2.lon)
	return math.Sqrt(d1*d1+d2*d2) * 60 * 1852
}

// StringMap is a map[string]string.
type StringMap map[string]string

// StringMap marshals into XML.
func (s StringMap) MarshalXML(e *xml.Encoder, start xml.StartElement) error {

	tokens := []xml.Token{start}

	for key, value := range s {
		t := xml.StartElement{Name: xml.Name{"", key}}
		tokens = append(tokens, t, xml.CharData(value), xml.EndElement{t.Name})
	}

	tokens = append(tokens, xml.EndElement{start.Name})

	for _, t := range tokens {
		err := e.EncodeToken(t)
		if err != nil {
			return err
		}
	}

	// flush to ensure tokens are written
	err := e.Flush()
	if err != nil {
		return err
	}

	return nil
}
