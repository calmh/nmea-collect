package main

import (
	"context"
	"fmt"
	"os"
	"time"

	nmea "github.com/adrianmo/go-nmea"
)

// srtAISProber sends periodical voltage probe messages, causing the
// connected AIS transponder to respond with the current input voltage.
type srtAISProber struct {
	dev      string
	interval time.Duration
}

func (p *srtAISProber) Serve(ctx context.Context) error {
	fd, err := os.OpenFile(p.dev, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer fd.Close()

	i := 1
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cmd := fmt.Sprintf("PSMT,0,0,0x00000000,1,vin,%d", i)
			cmd = fmt.Sprintf("$%s*%s", cmd, nmea.Checksum(cmd))
			if _, err := fmt.Fprintf(fd, "%s\r\n", cmd); err != nil {
				return err
			}
			i = (i + 1) % 10000

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
