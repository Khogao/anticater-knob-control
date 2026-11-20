package main

import "testing"

func TestVivid(t *testing.T) {
	for in, want := range map[string]string{"D97757": "FF3E00", "FFFFFF": "FFFFFF", "2B579A": "0065FF"} {
		if got := vivid(in); got != want {
			t.Errorf("vivid(%s) = %s, want %s", in, got, want)
		}
	}
}
