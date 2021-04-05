package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/adrianmo/go-nmea"
	"github.com/alecthomas/kong"
	"github.com/kastelo/nmea-collect/gpx"
)

var cli struct {
	SampleInterval        time.Duration `help:"Time between recorded track points" default:"30s"`
	TriggerDistanceMeters float64       `help:"Minimum movement to start track (m)" default:"25"`
	TriggerTimeWindow     time.Duration `help:"Time window for starting track" default:"1m"`
	CooldownTimeWindow    time.Duration `help:"Time window before ending track" default:"5m"`
	TCPAddr               string        `xor:"addr"`
	UDPPort               int           `xor:"addr"`
	Verbose               bool
}

func main() {
	log.SetFlags(0)
	kong.Parse(&cli)

	for key, parser := range parsers {
		nmea.RegisterParser(key, parser)
	}

	if cli.TCPAddr != "" {
		log.Println("Connecting to", cli.TCPAddr)
		errLoop(func() error { return collectTCP() })
	} else if cli.UDPPort != 0 {
		log.Println("Listening on port", cli.UDPPort)
		errLoop(func() error { return collectUDP() })
	} else {
		collectReader(os.Stdin)
	}
}

func errLoop(fn func() error) {
	for {
		err := fn()
		log.Println("Receive error:", err)
		time.Sleep(time.Minute)
		log.Println("Retrying...")
	}
}

func collectTCP() error {
	conn, err := net.Dial("tcp", cli.TCPAddr)
	if err != nil {
		return fmt.Errorf("collectTCP: %w", err)
	}
	defer conn.Close()

	if err := collectReader(conn); err != nil {
		return fmt.Errorf("collectTCP: %w", err)
	}
	return nil
}

func collectUDP() error {
	laddr := &net.UDPAddr{Port: cli.UDPPort}
	conn, err := net.ListenUDP("udp", laddr)
	if err != nil {
		return fmt.Errorf("collectUDP: %w", err)
	}
	defer conn.Close()

	if err := collectReader(conn); err != nil {
		return fmt.Errorf("collectUDP: %w", err)
	}
	return nil
}

func collectReader(r io.Reader) error {
	exts := make(gpx.Extensions)
	gpx := gpx.AutoGPX{
		Opener:                newGPXFile,
		SampleInterval:        cli.SampleInterval,
		TriggerDistanceMeters: cli.TriggerDistanceMeters,
		TriggerTimeWindow:     cli.TriggerTimeWindow,
		CooldownTimeWindow:    cli.CooldownTimeWindow,
	}
	defer gpx.Flush()

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 65536), 65536)

	for sc.Scan() {
		line := sc.Text()

		sent, err := nmea.Parse(line)
		if err != nil {
			if strings.Contains(err.Error(), "not supported") {
				continue
			}
			log.Printf("parse: %q: %v", line, err)
			continue
		}

		switch sent.DataType() {
		case TypeDPT:
			dpt := sent.(DPT)
			exts.Set("waterdepth", fmt.Sprintf("%.01f", dpt.Depth))

		case TypeHDG:
			hdg := sent.(HDG)
			exts.Set("heading", fmt.Sprintf("%.0f", hdg.Heading))

		case TypeMTW:
			mtw := sent.(MTW)
			exts.Set("watertemp", fmt.Sprintf("%.01f", mtw.Temperature))

		case TypeMWV:
			mwv := sent.(MWV)
			if mwv.Reference == "R" && mwv.Status == "A" {
				exts.Set("windangle", fmt.Sprintf("%.0f", mwv.Angle))
				exts.Set("windspeed", fmt.Sprintf("%.01f", mwv.Speed))
			}

		case TypeVLW:
			mwv := sent.(VLW)
			exts.Set("log", fmt.Sprintf("%.1f", mwv.TotalDistanceNauticalMiles))

		case nmea.TypeVHW:
			vhw := sent.(nmea.VHW)
			exts.Set("waterspeed", fmt.Sprintf("%.01f", vhw.SpeedThroughWaterKnots))

		case nmea.TypeRMC:
			rmc := sent.(nmea.RMC)
			when := time.Date(rmc.Date.YY+2000, time.Month(rmc.Date.MM), rmc.Date.DD, rmc.Time.Hour, rmc.Time.Minute, rmc.Time.Second, rmc.Time.Millisecond*int(time.Millisecond), time.UTC)
			if cli.Verbose {
				log.Println(when, rmc.Latitude, rmc.Longitude, exts)
			}
			gpx.Sample(rmc.Latitude, rmc.Longitude, when, exts)
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("reader: %w", err)
	}
	return nil
}

func newGPXFile() (io.WriteCloser, error) {
	name := time.Now().UTC().Format("track-20060102-150405.gpx")
	log.Println("Creating new GPX track", name)
	return os.Create(name)
}
