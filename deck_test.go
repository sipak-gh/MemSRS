// Tests for deck.go: CSV parsing, using testdata/messy.csv for edge cases.

package main

import (
	"slices"
	"strings"
	"testing"
)

func TestLoadCSVMessy(t *testing.T) {
	cards, skipped, err := LoadCSV("testdata/messy.csv")
	if err != nil {
		t.Fatal(err)
	}
	want := []Card{
		{"hola", "hello"},
		{"a, b", "comma, inside"},
		{"multi\nline", "back\nside"},
		{"extra", "cols"},
		{"padded", "spaces"},
	}
	if !slices.Equal(cards, want) {
		t.Errorf("cards = %q\nwant    %q", cards, want)
	}
	if skipped != 3 { // only-one, empty-back, duplicate hola
		t.Errorf("skipped = %d, want 3", skipped)
	}
}

func TestParseCSVNoHeader(t *testing.T) {
	cards, _, _ := parseCSV(strings.NewReader("a,b\nc,d\n"))
	if len(cards) != 2 {
		t.Errorf("got %d cards, want 2", len(cards))
	}
}
