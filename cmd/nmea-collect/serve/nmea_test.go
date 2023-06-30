package serve

import (
	"testing"

	nmea "github.com/adrianmo/go-nmea"
)

func TestParseXDR(t *testing.T) {
	sent, err := nmea.Parse(`$YDXDR,C,4.4,C,Air,P,98950,P,Baro,C,5.4,C,ENV_INSIDE_T*1E`)
	if err != nil {
		t.Fatal(err)
	}
	xdr := sent.(nmea.XDR)
	if len(xdr.Measurements) != 3 {
		t.Fatal("expected 3 measurements")
	}
	if xdr.Measurements[0].Value != 4.4 {
		t.Error("bad temperature 2", xdr.Measurements[0].Value)
	}
	if xdr.Measurements[1].Value != 98950 {
		t.Error("bad pressure", xdr.Measurements[1].Value)
	}
	if xdr.Measurements[2].Value != 5.4 {
		t.Error("bad temperature 2", xdr.Measurements[2].Value)
	}
}

func TestParseDIN(t *testing.T) {
	sent, err := nmea.Parse(`$PCDIN,01F214,47B319FE,55,00C8040000FFFFC4*51`)
	if err != nil {
		t.Fatal(err)
	}
	din := sent.(nmea.PCDIN)
	if v := pcdinBatteryVoltage(din); v != 12.24 {
		t.Error("bad battery voltage", v)
	}
}
