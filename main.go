package main

import (
	"bufio"
	"io"
	"log"
	"net"
	"os"
	"time"

	"github.com/adrianmo/go-nmea"
	"github.com/alecthomas/kingpin"
	"github.com/kastelo/nmea-collect/gpx"
)

type record struct {
	when time.Time
	line string
}

func main() {
	minMove := kingpin.Flag("min-move", "Minimum movement (m)").Default("15").Float64()
	addr := kingpin.Arg("address", "NMEA TCP address").Required().String()
	kingpin.Parse()

	collect(*addr, *minMove)
}

func collect(addr string, minMove float64) {
	for {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			log.Println("collect:", err)
			time.Sleep(time.Minute)
			continue
		}

		log.Println("Connected to", addr)

		if err := collectReader(conn, minMove); err != nil {
			log.Println("collect:", err)
		}
		conn.Close()
	}
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
	conn.SetReadDeadline(time.Now().Add(time.Minute))
	for sc.Scan() {
		line := sc.Text()
		conn.SetReadDeadline(time.Now().Add(time.Minute))

		sent, err := nmea.Parse(line)
		if err != nil {
			log.Println("parse:", err)
			continue
		}

		switch sent.DataType() {
		case nmea.TypeRMC:
			rmc := sent.(nmea.RMC)
			when := time.Date(rmc.Date.YY+2000, time.Month(rmc.Date.MM), rmc.Date.DD, rmc.Time.Hour, rmc.Time.Minute, rmc.Time.Second, rmc.Time.Millisecond*int(time.Millisecond), time.UTC)
			gpx.Sample(rmc.Latitude, rmc.Longitude, when)
		}
	}
	return sc.Err()
}

func newGPXFile() (io.WriteCloser, error) {
	name := time.Now().UTC().Format("track-20060102-150405.gpx")
	log.Println("Creating new GPX track", name)
	return os.Create(name)
}
