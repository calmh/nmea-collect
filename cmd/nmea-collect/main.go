package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"

	"calmh.dev/nmea-collect/cmd/nmea-collect/consolidate-gzip"
	"calmh.dev/nmea-collect/cmd/nmea-collect/serve"
	"calmh.dev/nmea-collect/cmd/nmea-collect/summarize-gpx"
	"github.com/alecthomas/kong"
	"github.com/lmittmann/tint"
	"github.com/mattn/go-isatty"
	"golang.org/x/exp/slog"
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

	var handler slog.Handler
	if isatty.IsTerminal(os.Stderr.Fd()) {
		handler = tint.NewHandler(os.Stderr, &tint.Options{TimeFormat: "15:04:05.000"})
	} else {
		handler = slog.NewTextHandler(os.Stderr, nil)
	}
	logger := slog.New(handler)
	slog.SetDefault(logger)

	var cli CLI
	kongCtx := kong.Parse(&cli)
	kongCtx.BindTo(ctx, (*context.Context)(nil))
	kongCtx.Bind(logger)

	if err := kongCtx.Run(); err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("Run", "error", err)
		os.Exit(1)
	}
}
