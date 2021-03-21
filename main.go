package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"

	"github.com/adrianmo/go-nmea"
	"github.com/alecthomas/kong"
	"github.com/kastelo/nmea-collect/gpx"
)

var cli struct {
	MinMove float64 `help:"Minimum trackpoint move (m)" default:"15"`
	TCPAddr string  `xor:"addr"`
	UDPPort int     `xor:"addr"`
}

func main() {
	log.SetFlags(0)
	kong.Parse(&cli)

	if cli.TCPAddr != "" {
		log.Println("Connecting to", cli.TCPAddr)
		errLoop(func() error { return collectTCP(cli.TCPAddr, cli.MinMove) })
	} else if cli.UDPPort != 0 {
		log.Println("Listening on port", cli.UDPPort)
		errLoop(func() error { return collectUDP(cli.UDPPort, cli.MinMove) })
	} else {
		log.Fatal("must set --tcp-addr or --udp-port")
	}
}

func errLoop(fn func() error) {
	deadline := os.ErrDeadlineExceeded
	for {
		err := fn()
		if errors.Is(err, deadline) {
			log.Printf("No data received (%v)", err)
			continue
		}
		log.Println("Receive error:", err)
		time.Sleep(time.Minute)
	}
}

func collectTCP(addr string, minMove float64) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("collectTCP: %w", err)
	}
	defer conn.Close()

	if err := collectReader(conn, minMove); err != nil {
		return fmt.Errorf("collectTCP: %w", err)
	}
	return nil
}

func collectUDP(port int, minMove float64) error {
	laddr := &net.UDPAddr{Port: port}
	conn, err := net.ListenUDP("udp", laddr)
	if err != nil {
		return fmt.Errorf("collectUDP: %w", err)
	}
	defer conn.Close()

	if err := collectReader(conn, minMove); err != nil {
		return fmt.Errorf("collectUDP: %w", err)
	}
	return nil
}

func collectReader(conn net.Conn, minMove float64) error {
	gpx := gpx.AutoGPX{
		Opener:                newGPXFile,
		SampleInterval:        10 * time.Second,
		TriggerDistanceMeters: minMove,
		TriggerTimeWindow:     60 * time.Second,
		CooldownTimeWindow:    300 * time.Second,
	}

	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 0, 65536), 65536)
	_ = conn.SetReadDeadline(time.Now().Add(time.Minute))
	exts := make(map[string]string)
	for sc.Scan() {
		line := sc.Text()
		_ = conn.SetReadDeadline(time.Now().Add(time.Minute))

		sent, err := nmea.Parse(line)
		if err != nil {
			log.Println("parse:", err)
			continue
		}

		switch sent.DataType() {
		case nmea.TypeDPT:
			dpt := sent.(nmea.DPT)
			exts["gpxx:Depth"] = fmt.Sprint(dpt.Depth)
		case nmea.TypeRMC:
			rmc := sent.(nmea.RMC)
			when := time.Date(rmc.Date.YY+2000, time.Month(rmc.Date.MM), rmc.Date.DD, rmc.Time.Hour, rmc.Time.Minute, rmc.Time.Second, rmc.Time.Millisecond*int(time.Millisecond), time.UTC)
			if gpx.Sample(rmc.Latitude, rmc.Longitude, when, exts) {
				exts = make(map[string]string)
			}
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
