package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"os"
	"sync"
	"time"

	"github.com/adrianmo/go-nmea"
	"github.com/alecthomas/kingpin"
)

type record struct {
	when time.Time
	line string
}

func main() {
	minMove := kingpin.Flag("min-move", "Minimum movement (m)").Default("50").Float64()
	pattern := kingpin.Flag("pattern", "File name pattern (Go time.Format syntax)").Default("nmea-20060102.txt").String()
	addr := kingpin.Arg("address", "NMEA TCP address").Required().String()
	kingpin.Parse()

	lines := make(chan record, 16)
	go collect(lines, *addr, *minMove)
	write(lines, *pattern)
}

func write(records <-chan record, pattern string) {
	iw := &intervalFile{pattern: pattern}

	defer iw.Close()
	for rec := range records {
		iw.WriteLine(rec.line, rec.when)
	}
}

func collect(records chan<- record, addr string, minMove float64) {
	for {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			log.Println("collect:", err)
			time.Sleep(time.Minute)
			continue
		}
		if err := collectReader(records, conn, minMove); err != nil {
			log.Println("collect:", err)
		}
		conn.Close()
	}
}

func collectReader(records chan<- record, conn net.Conn, minMove float64) error {
	var lat, lon float64
	var date nmea.Date
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
		if sent.DataType() == nmea.TypeRMC {
			rmc := sent.(nmea.RMC)
			if rmc.Date != date || distance(lat, lon, rmc.Latitude, rmc.Longitude) > minMove {
				records <- record{
					when: time.Date(rmc.Date.YY, time.Month(rmc.Date.MM), rmc.Date.DD, rmc.Time.Hour, rmc.Time.Minute, rmc.Time.Second, rmc.Time.Millisecond*int(time.Millisecond), time.UTC),
					line: line,
				}
				lat = rmc.Latitude
				lon = rmc.Longitude
				date = rmc.Date
			}
		}
	}
	return sc.Err()
}

func distance(lat1, lon1, lat2, lon2 float64) float64 {
	s1 := math.Abs(lat1 - lat2)
	s2 := math.Abs(lon1 - lon2)
	return math.Sqrt(s1*s1+s2*s2) * 60 * 1852
}

type intervalFile struct {
	pattern string
	current string
	fd      io.WriteCloser
	mut     sync.Mutex
}

func (i *intervalFile) WriteLine(line string, when time.Time) (int, error) {
	i.mut.Lock()
	defer i.mut.Unlock()

	if when.UTC().Format(i.pattern) != i.current {
		if err := i.reopen(); err != nil {
			return 0, err
		}
	}
	return fmt.Fprintf(i.fd, "%s\n", line)
}

func (i *intervalFile) Close() error {
	i.mut.Lock()
	defer i.mut.Unlock()
	defer func() { i.fd = nil }()
	return i.fd.Close()
}

func (i *intervalFile) reopen() error {
	if i.fd != nil {
		i.fd.Close()
		i.fd = nil
	}

	name := time.Now().UTC().Format(i.pattern)
	fd, err := os.OpenFile(name, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	i.fd = fd
	i.current = name
	return nil
}
