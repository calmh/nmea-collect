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

func main() {
	minMove := kingpin.Flag("min-move", "Minimum movement (m)").Default("50").Float64()
	pattern := kingpin.Flag("pattern", "File name pattern (Go time.Format syntax)").Default("nmea-20060102.txt").String()
	addr := kingpin.Arg("address", "NMEA TCP address").Required().String()
	kingpin.Parse()

	lines := make(chan string, 16)
	go collect(lines, *addr, *minMove)
	write(lines, *pattern)
}

func write(lines <-chan string, pattern string) {
	iw := &intervalFile{pattern: pattern}

	defer iw.Close()
	for line := range lines {
		fmt.Fprintln(iw, line)
	}
}

func collect(lines chan<- string, addr string, minMove float64) {
	for {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			log.Println("collect:", err)
			time.Sleep(time.Minute)
			continue
		}
		if err := collectReader(lines, conn, minMove); err != nil {
			log.Println("collect:", err)
		}
		conn.Close()
	}
}

func collectReader(lines chan<- string, conn net.Conn, minMove float64) error {
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
				lines <- line
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

func (i *intervalFile) Write(data []byte) (int, error) {
	i.mut.Lock()
	defer i.mut.Unlock()

	if time.Now().UTC().Format(i.pattern) != i.current {
		if err := i.reopen(); err != nil {
			return 0, err
		}
	}
	return i.fd.Write(data)
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
	fd, err := os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if os.IsExist(err) {
		fd, err = os.OpenFile(name+time.Now().UTC().Format("+20060102150405.000"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	}
	if err != nil {
		return err
	}
	i.fd = fd
	i.current = name
	return nil
}
