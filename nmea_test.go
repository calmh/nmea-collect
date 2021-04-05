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
