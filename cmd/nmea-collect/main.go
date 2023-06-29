package main

import (
	"log"

	"calmh.dev/nmea-collect/cmd/nmea-collect/consolidate-gzip"
	"calmh.dev/nmea-collect/cmd/nmea-collect/serve"
	"calmh.dev/nmea-collect/cmd/nmea-collect/summarize-gpx"
	"github.com/alecthomas/kong"
)

type CLI struct {
	Serve           serve.CLI       `cmd:"" default:"" help:"Process incoming NMEA data"`
	ConsolidateGzip consolidate.CLI `cmd:"" help:"Consolidate GZIP files"`
	SummarizeGPX    summarize.CLI   `cmd:"" help:"Summarize GPX files"`
}

func main() {
	log.SetFlags(0)

	var cli CLI
	ctx := kong.Parse(&cli)
	if err := ctx.Run(); err != nil {
		log.Fatal(err)
	}
}
