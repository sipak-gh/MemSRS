// Command memsrs reviews flashcards from a CSV file in the terminal using
// spaced repetition, like a minimal Anki.
//
//	memsrs [-new N] deck.csv
//
// Review progress is stored in srs.db in the same directory as the CSV.
//
// Code layout:
//
//	main.go   command-line handling, terminal setup, end-of-session summary
//	deck.go   CSV parsing
//	srs.go    scheduling algorithm (what happens when you grade a card)
//	store.go  SQLite persistence
//	ui.go     study queue and the interactive review loop
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/term"
)

// dbName is the database file created next to the CSV.
const dbName = "srs.db"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "memsrs:", err)
		os.Exit(1)
	}
}

// run parses flags, loads the deck and its saved progress, studies whatever
// is due, then prints a summary.
func run() error {
	newPerDay := flag.Int("new", 20, "max new cards introduced per day (-1 = unlimited)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `usage: memsrs [-new N] deck.csv

Review flashcards from a CSV (front,back) with spaced repetition.
Progress is stored in %s next to the CSV.

  [space] show answer
  [q] again  [w] hard  [e] good  [r] easy
  [esc] quit

`, dbName)
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	path, err := filepath.Abs(flag.Arg(0))
	if err != nil {
		return err
	}
	cards, skipped, err := LoadCSV(path)
	if err != nil {
		return err
	}
	if skipped > 0 {
		fmt.Fprintf(os.Stderr, "skipped %d empty, malformed or duplicate rows\n", skipped)
	}
	if len(cards) == 0 {
		return errors.New("no cards found in " + path)
	}

	store, err := OpenStore(filepath.Join(filepath.Dir(path), dbName))
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer store.Close()

	deck := filepath.Base(path)
	states, err := store.Load(deck)
	if err != nil {
		return err
	}

	// The new-card limit is per day: subtract new cards already studied today.
	now := time.Now()
	newLimit := *newPerDay
	if newLimit >= 0 {
		today, err := store.NewIntroducedSince(deck, startOfDay(now))
		if err != nil {
			return err
		}
		newLimit = max(0, newLimit-today)
	}

	if queue := buildQueue(cards, states, now, newLimit); len(queue) > 0 {
		reviewed, err := study(deck, store, queue)
		if err != nil {
			return err
		}
		fmt.Printf("Reviewed %d cards.\n", reviewed)
		if states, err = store.Load(deck); err != nil {
			return err
		}
	}
	printSummary(cards, states, time.Now())
	return nil
}

// study runs an interactive session and returns how many answers were given.
// Quitting early is not an error.
//
// The terminal is put in raw mode so each key press is delivered immediately
// (no enter needed) and isn't echoed. It is always restored afterwards.
func study(deck string, store *Store, queue []*item) (int, error) {
	fd := int(os.Stdin.Fd())
	old, err := term.MakeRaw(fd)
	if err != nil {
		return 0, fmt.Errorf("stdin must be a terminal: %w", err)
	}
	defer func() {
		term.Restore(fd, old)
		fmt.Print(ansiClear)
	}()
	s := &session{deck: deck, store: store, in: os.Stdin, out: os.Stdout, queue: queue}
	err = s.run()
	if errors.Is(err, errQuit) {
		err = nil
	}
	return s.reviewed, err
}

// printSummary prints how many cards are due or new, and when the next
// scheduled review is.
func printSummary(cards []Card, states map[string]State, now time.Time) {
	var due, unseen int
	var next time.Time
	for _, c := range cards {
		st, seen := states[c.Front]
		switch {
		case !seen:
			unseen++
		case st.IsDue(now):
			due++
		case next.IsZero() || st.Due.Before(next):
			next = st.Due
		}
	}
	fmt.Printf("%d cards: %d due now, %d new", len(cards), due, unseen)
	if !next.IsZero() {
		fmt.Printf(", next review %s", next.Format("Mon Jan 2"))
	}
	fmt.Println()
}
