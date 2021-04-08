package main

import "testing"

func TestParseAIS(t *testing.T) {
	t.Log(parseAIS("!AIVDM,2,1,8,A,54QsF2h2;?>UK84W620hTiV0:222222222222216:h>675380=jkiAi,0*79"))
}
