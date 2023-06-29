package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

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

	ctx, cancel := context.WithCancel(context.Background())

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()

	var cli CLI
	kongCtx := kong.Parse(&cli)
	kongCtx.BindTo(ctx, (*context.Context)(nil))
	if err := kongCtx.Run(); err != nil {
		log.Fatal(err)
	}
}
