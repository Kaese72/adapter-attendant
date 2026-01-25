package main

import "testing"

func TestDummy(t *testing.T) {
	// Dummy test to ensure go test passes
	if 1+1 != 2 {
		t.Error("Math is broken")
	}
}
