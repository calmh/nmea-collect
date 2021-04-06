package main

import "testing"

func TestDedup(t *testing.T) {
	d := make(deduper, 10)
	if !d.Check("a") {
		t.Fatal("false positive")
	}
	if !d.Check("b") {
		t.Fatal("false positive")
	}
	if !d.Check("c") {
		t.Fatal("false positive")
	}
	if d.Check("a") {
		t.Fatal("false negative")
	}
	if !d.Check("d") {
		t.Fatal("false positive")
	}
	if d.Check("c") {
		t.Fatal("false negative")
	}
}

func TestParseAIS(t *testing.T) {
	t.Log(parseAIS("!AIVDM,2,1,8,A,54QsF2h2;?>UK84W620hTiV0:222222222222216:h>675380=jkiAi,0*79"))
}
