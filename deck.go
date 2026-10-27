// deck.go — reading flashcards from a CSV file.

package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
)

// Card is one flashcard. Front identifies the card in the database, so
// editing the back of a card keeps its review progress; editing the front
// makes it a new card.
type Card struct {
	Front, Back string
}

// headerPairs are first rows that are treated as a header and skipped
// (compared case-insensitively).
var headerPairs = map[string]string{
	"front":    "back",
	"question": "answer",
	"term":     "definition",
}

// LoadCSV reads cards from a CSV file. See parseCSV for the format.
func LoadCSV(path string) (cards []Card, skipped int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	return parseCSV(f)
}

// parseCSV reads "front,back" rows. Quoted fields may contain commas and
// newlines, and columns after the second are ignored. Rows missing a front or
// back, and rows repeating an earlier front, are skipped and counted.
func parseCSV(r io.Reader) (cards []Card, skipped int, err error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1 // rows may have any number of columns
	cr.LazyQuotes = true

	seen := map[string]bool{}
	for first := true; ; first = false {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, 0, fmt.Errorf("parsing csv: %w", err)
		}
		if first {
			rec[0] = strings.TrimPrefix(rec[0], "\ufeff") // byte-order mark added by Excel
			if len(rec) >= 2 && isHeader(rec[0], rec[1]) {
				continue
			}
		}
		if len(rec) < 2 {
			skipped++
			continue
		}
		front, back := strings.TrimSpace(rec[0]), strings.TrimSpace(rec[1])
		if front == "" || back == "" || seen[front] {
			skipped++
			continue
		}
		seen[front] = true
		cards = append(cards, Card{Front: front, Back: back})
	}
	return cards, skipped, nil
}

func isHeader(a, b string) bool {
	want, ok := headerPairs[strings.ToLower(strings.TrimSpace(a))]
	return ok && want == strings.ToLower(strings.TrimSpace(b))
}
