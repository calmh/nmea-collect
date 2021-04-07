package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/BertoldVdb/go-ais"
	nmea "github.com/adrianmo/go-nmea"
	"github.com/alecthomas/kong"
	"github.com/kastelo/nmea-collect/gpx"
)

var cli struct {
	SampleInterval        time.Duration `help:"Time between recorded track points" default:"30s"`
	TriggerDistanceMeters float64       `help:"Minimum movement to start track (m)" default:"25"`
	TriggerTimeWindow     time.Duration `help:"Time window for starting track" default:"1m"`
	CooldownTimeWindow    time.Duration `help:"Time window before ending track" default:"5m"`
	TCPAddr               []string
	UDPPort               []int
	Verbose               bool
	ForwardAISUDP         []string `name:"forward-ais-udp"`
	ForwardMinPacket      int      `default:"1400"`
	DedupBufferSize       int
	AISDedupTime          time.Duration
	ListenAllTCP          string `default:":2000"`
	ListenAISTCP          string `default:":2010" name:"listen-ais-tcp"`
}

func main() {
	log.SetFlags(0)
	kong.Parse(&cli)

	for key, parser := range parsers {
		nmea.RegisterParser(key, parser)
	}

	c := make(chan string)

	go copyInto(c, lines(os.Stdin, "stdin"))

	for _, addr := range cli.TCPAddr {
		go readTCPInto(c, addr)
	}

	for _, port := range cli.UDPPort {
		go readUDPInto(c, port)
	}

	if cli.ListenAllTCP != "" {
		a, b := tee(c)
		c = a
		forwardTCP(b, cli.ListenAllTCP)
	}

	if len(cli.ForwardAISUDP) > 0 {
		a, b := tee(c)
		c = a
		ais := prefix(b, "!AI")
		if cli.DedupBufferSize > 0 {
			ais = dedup(ais)
		}
		forwardAIS(ais)
	}

	if cli.ListenAISTCP != "" {
		a, b := tee(c)
		c = a
		ais := prefix(b, "!AI")
		forwardTCP(ais, cli.ListenAISTCP)
	}

	collect(prefix(c, "$"))

	select {}
}

func readTCPInto(c chan<- string, addr string) {
	for {
		r, err := tcpReader(addr)
		if err != nil {
			log.Fatalln("TCP input:", err)
		}
		copyInto(c, lines(r, "tcp/"+addr))
		time.Sleep(5 * time.Second)
	}
}

func tcpReader(addr string) (io.Reader, error) {
	log.Println("Connecting to", addr)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("reader: %w", err)
	}
	return conn, nil
}

func readUDPInto(c chan<- string, port int) {
	for {
		r, err := udpReader(port)
		if err != nil {
			log.Println("UDP input:", err)
		}
		copyInto(c, lines(r, fmt.Sprintf("udp/%d", port)))
		time.Sleep(5 * time.Second)
	}
}

func udpReader(port int) (io.Reader, error) {
	log.Println("Listening for UDP on port", port)
	laddr := &net.UDPAddr{Port: port}
	conn, err := net.ListenUDP("udp", laddr)
	if err != nil {
		return nil, fmt.Errorf("reader: %w", err)
	}
	return conn, nil
}

func forwardAIS(c <-chan string) error {
	dsts := make([]net.Conn, len(cli.ForwardAISUDP))
	for i, addr := range cli.ForwardAISUDP {
		dst, err := net.Dial("udp", addr)
		if err != nil {
			return err
		}
		log.Println("Forwarding AIS to udp/" + addr)
		dsts[i] = dst
	}

	dedup := make(map[ais.Header]time.Time)
	go func() {
		var lastWrite time.Time
		nextLog := time.Now().Truncate(time.Minute).Add(time.Minute)
		outBuf := new(bytes.Buffer)
		recvd, sent, skip := 0, 0, 0

		for line := range c {
			recvd++

			if cli.AISDedupTime > 0 {
				messageID, ok := parseAIS(line)
				if ok {
					if seen := dedup[*messageID]; time.Since(seen) < cli.AISDedupTime {
						skip++
						continue
					}
					dedup[*messageID] = time.Now()
				}
			}

			fmt.Fprintf(outBuf, "%s\r\n", line)
			if outBuf.Len() > cli.ForwardMinPacket || time.Since(lastWrite) > time.Minute {
				for _, dst := range dsts {
					_, err := dst.Write(outBuf.Bytes())
					if err != nil {
						log.Printf("Write %v: %v", dst.RemoteAddr(), err)
					}
					sent++
				}
				outBuf.Reset()
				lastWrite = time.Now()
			}

			if time.Since(nextLog) > 0 {
				log.Printf("forwardAIS: received %d messages, skipped %d, sent %d packets", recvd, skip, sent)
				recvd, sent, skip = 0, 0, 0
				nextLog = time.Now().Truncate(time.Minute).Add(time.Minute)

				if cli.AISDedupTime > 0 {
					cutoff := time.Now().Add(-cli.AISDedupTime)
					for id, seen := range dedup {
						if seen.Before(cutoff) {
							delete(dedup, id)
						}
					}
				}
			}
		}
	}()

	return nil
}

func collect(c <-chan string) {
	exts := make(gpx.Extensions)
	gpx := gpx.AutoGPX{
		Opener:                newGPXFile,
		SampleInterval:        cli.SampleInterval,
		TriggerDistanceMeters: cli.TriggerDistanceMeters,
		TriggerTimeWindow:     cli.TriggerTimeWindow,
		CooldownTimeWindow:    cli.CooldownTimeWindow,
	}
	defer gpx.Flush()

	nextLog := time.Now().Truncate(time.Minute).Add(time.Minute)
	go func() {
		for line := range c {
			sent, err := nmea.Parse(line)
			if err != nil {
				if strings.Contains(err.Error(), "not supported") {
					continue
				}
				if cli.Verbose {
					log.Printf("parse: %q: %v", line, err)
				}
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

			if time.Since(nextLog) > 0 {
				log.Printf("collect: %s", exts)
				nextLog = time.Now().Truncate(time.Minute).Add(time.Minute)
			}
		}
	}()
}

func newGPXFile() (io.WriteCloser, error) {
	name := time.Now().UTC().Format("track-20060102-150405.gpx")
	log.Println("Creating new GPX track", name)
	return os.Create(name)
}

type deduper []string

func (d deduper) Check(s string) bool {
	for _, c := range d {
		if c == s {
			return false
		}
	}
	copy(d[1:], d)
	d[0] = s
	return true
}

func dedup(c <-chan string) chan string {
	dedup := make(deduper, cli.DedupBufferSize)
	res := make(chan string)
	recv, dup := 0, 0
	nextLog := time.Now().Truncate(time.Minute).Add(time.Minute)
	go func() {
		defer close(res)
		for line := range c {
			recv++
			if dedup.Check(line) {
				res <- line
			} else {
				dup++
			}
			if cli.Verbose && time.Since(nextLog) > 0 {
				log.Printf("dedup: received %d messages (%d duplicates)", recv, dup)
				recv, dup = 0, 0
				nextLog = time.Now().Truncate(time.Minute).Add(time.Minute)
			}
		}
	}()
	return res
}

func prefix(c <-chan string, prefix string) <-chan string {
	res := make(chan string)
	go func() {
		defer close(res)
		for s := range c {
			if strings.HasPrefix(s, prefix) {
				res <- s
			}
		}
	}()
	return res
}

func lines(r io.Reader, name string) <-chan string {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 65536), 65536)

	res := make(chan string)
	recv, badPrefix, noChecksum, badChecksum := 0, 0, 0, 0
	nextLog := time.Now().Truncate(time.Minute).Add(time.Minute)
	go func() {
		defer close(res)
		for sc.Scan() {
			if cli.Verbose && time.Since(nextLog) > 0 {
				log.Printf("%s: received %d messages (skipped %d bad prefix, %d missing checksum, %d bad checksum)", name, recv, badPrefix, noChecksum, badChecksum)
				recv, badPrefix, noChecksum, badChecksum = 0, 0, 0, 0
				nextLog = time.Now().Truncate(time.Minute).Add(time.Minute)
			}

			line := sc.Text()
			recv++
			if line == "" {
				badPrefix++
				continue
			}
			switch line[0] {
			case '!', '$':
				idx := strings.LastIndexByte(line, '*')
				if idx == -1 {
					noChecksum++
					continue
				}
				chk := nmea.Checksum(line[1:idx])
				if chk != line[idx+1:] {
					badChecksum++
					continue
				}
				res <- line
			default:
				badPrefix++
			}
		}
	}()
	return res
}

func copyInto(dst chan<- string, src <-chan string) {
	for s := range src {
		dst <- s
	}
}

func tee(c <-chan string) (chan string, chan string) {
	a := make(chan string)
	b := make(chan string)
	go func() {
		defer close(a)
		defer close(b)
		for s := range c {
			a <- s
			b <- s
		}
	}()
	return a, b
}

type tcpForwarder struct {
	input <-chan string
	conns []net.Conn
	mut   sync.Mutex
}

func forwardTCP(input <-chan string, addr string) {
	f := &tcpForwarder{
		input: input,
	}
	go f.run()
	go f.listen(addr)
}

func (f *tcpForwarder) run() {
	for line := range f.input {
		f.mut.Lock()
		for i := 0; i < len(f.conns); i++ {
			f.conns[i].SetWriteDeadline(time.Now().Add(time.Second))
			if _, err := fmt.Fprintf(f.conns[i], "%s\n", line); err != nil {
				log.Println("Dropping connection from", f.conns[i].RemoteAddr(), "due to", err)
				f.conns[i].Close()
				f.conns = append(f.conns[:i], f.conns[i+1:]...)
				i--
			}
		}
		f.mut.Unlock()
	}
}

func (f *tcpForwarder) listen(addr string) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Println("Listen:", err)
		return
	}
	defer l.Close()
	log.Println("Listening on", l.Addr())

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Println("Accept:", err)
			return
		}

		log.Println("Incoming connection from", conn.RemoteAddr(), "to", l.Addr())
		f.mut.Lock()
		f.conns = append(f.conns, conn)
		f.mut.Unlock()
	}
}
