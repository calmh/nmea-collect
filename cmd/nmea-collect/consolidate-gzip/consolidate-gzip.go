package consolidate

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/exp/slices"
	"golang.org/x/exp/slog"
)

type CLI struct {
	Path string `arg:"" required:""`
	From string `default:"nmea-raw.20060102-150405.gz"`
	To   string `default:"nmea-raw.200601.gz"`
}

func (cli *CLI) Run() error {
	entries, err := os.ReadDir(cli.Path)
	if err != nil {
		return err
	}
	slices.SortFunc(entries, func(a, b os.DirEntry) bool {
		return a.Name() < b.Name()
	})

	curMon := time.Now().Format("200601")
	buf := make([]byte, 65536)
	var curOutfile string
	var curGw *gzip.Writer
	var curW *os.File
	var toDelete []string
	for _, e := range entries {
		t, err := time.Parse(cli.From, e.Name())
		if err != nil {
			continue
		}
		if t.Format("200601") == curMon {
			continue
		}

		fd, err := os.Open(filepath.Join(cli.Path, e.Name()))
		if err != nil {
			slog.Error("Reading infile", "from", e.Name(), "error", err)
			continue
		}
		gr, err := gzip.NewReader(fd)
		if err != nil {
			slog.Error("Reading infile", "from", e.Name(), "error", err)
			fd.Close()
			continue
		}

		outFile := t.Format(cli.To)
		if curOutfile != outFile {
			if curGw != nil {
				curGw.Close()
			}
			if curW != nil {
				curW.Close()
			}
			for _, f := range toDelete {
				os.Remove(filepath.Join(cli.Path, f))
			}
			curW, err = os.Create(filepath.Join(cli.Path, outFile))
			if err != nil {
				return fmt.Errorf("create outfile: %w", err)
			}
			curGw = gzip.NewWriter(curW)
			curOutfile = outFile
		}

		slog.Info("Consolidate", "from", e.Name(), "to", outFile)
		for {
			n, err := gr.Read(buf)
			if n > 0 {
				if _, err := curGw.Write(buf[:n]); err != nil {
					return fmt.Errorf("%s: %w", outFile, err)
				}
			}
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				slog.Error("Consolidate", "from", e.Name(), "to", outFile, "error", err)
				break
			}
		}
		toDelete = append(toDelete, e.Name())
		fd.Close()
	}

	for _, f := range toDelete {
		os.Remove(filepath.Join(cli.Path, f))
	}

	return nil
}
