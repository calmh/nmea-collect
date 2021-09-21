package main

import (
	"testing"

	nmea "github.com/adrianmo/go-nmea"
)

func init() {
	for key, parser := range parsers {
		_ = nmea.RegisterParser(key, parser)
	}
}

func TestParseAIS(t *testing.T) {
	t.Log(parseAIS("!AIVDM,2,1,8,A,54QsF2h2;?>UK84W620hTiV0:222222222222216:h>675380=jkiAi,0*79"))
}

func TestParseXDR(t *testing.T) {
	sent, err := nmea.Parse(`$YDXDR,C,14.3,C,Air*11`)
	if err != nil {
		t.Fatal(err)
	}
	xdr := sent.(XDR)
	if xdr.Measurement != 14.3 {
		t.Error("bad temperature", xdr.Measurement)
	}
}
