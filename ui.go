// ui.go — the review session: builds the queue of cards to study and runs
// the interactive loop (show front → space → show back → grade).

package main

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"
)

// ANSI escape codes used for drawing.
const (
	ansiClear = "\x1b[2J\x1b[H" // clear screen, cursor to top-left
	ansiBold  = "\x1b[1m"
	ansiDim   = "\x1b[2m"
	ansiReset = "\x1b[0m"
	ansiRed   = "\x1b[31m"
	ansiYel   = "\x1b[33m"
	ansiGreen = "\x1b[32m"
	ansiBlue  = "\x1b[34m"
	ansiCyan  = "\x1b[36m"
)

// relearnGap is how many cards later a card answered Again is shown again.
const relearnGap = 3

// gradeKeys maps keys to grades and how each grade is drawn.
var gradeKeys = []struct {
	key   byte
	grade Grade
	name  string
	color string
}{
	{'q', Again, "again", ansiRed},
	{'w', Hard, "hard", ansiYel},
	{'e', Good, "good", ansiGreen},
	{'r', Easy, "easy", ansiCyan},
}

// gradeForKey returns the grade for one of the keys in gradeKeys.
func gradeForKey(k byte) Grade {
	for _, gk := range gradeKeys {
		if gk.key == k {
			return gk.grade
		}
	}
	panic(fmt.Sprintf("no grade for key %q", k))
}

// errQuit is returned when the user quits mid-session (esc, ctrl-c, ctrl-d).
var errQuit = errors.New("quit")

// item is a card waiting in the session queue.
type item struct {
	card  Card
	state State
	isNew bool // never reviewed before; shows a NEW tag
}

// buildQueue returns the cards to study: due cards (most overdue first)
// followed by up to newLimit new cards in CSV order (newLimit < 0 = no limit).
func buildQueue(cards []Card, states map[string]State, now time.Time, newLimit int) []*item {
	var due, fresh []*item
	for _, c := range cards {
		st, seen := states[c.Front]
		switch {
		case !seen:
			if newLimit < 0 || len(fresh) < newLimit {
				fresh = append(fresh, &item{card: c, state: NewState(), isNew: true})
			}
		case st.IsDue(now):
			due = append(due, &item{card: c, state: st})
		}
	}
	slices.SortStableFunc(due, func(a, b *item) int { return a.state.Due.Compare(b.state.Due) })
	return append(due, fresh...)
}

// session is one study run over a queue of cards.
type session struct {
	deck     string    // deck name used as the database key
	store    *Store    // where each answer is saved
	in       io.Reader // key presses, one per Read (as a raw-mode terminal delivers them)
	out      io.Writer // where the screen is drawn
	queue    []*item   // cards still to show; queue[0] is next
	reviewed int       // answers given so far
}

// run studies cards until the queue is empty or the user quits (errQuit).
// Each answer is saved immediately, so quitting never loses progress.
func (s *session) run() error {
	for len(s.queue) > 0 {
		it := s.queue[0]
		s.queue = s.queue[1:]

		s.render(it, false)
		if _, err := s.waitFor(" "); err != nil {
			return err
		}
		s.render(it, true)
		k, err := s.waitFor("qwer") // the keys in gradeKeys
		if err != nil {
			return err
		}
		g := gradeForKey(k)

		now := time.Now()
		it.state = Schedule(it.state, g, now)
		if err := s.store.Save(s.deck, it.card.Front, it.state, now); err != nil {
			return fmt.Errorf("saving review: %w", err)
		}
		it.isNew = false
		s.reviewed++

		if g == Again {
			s.queue = slices.Insert(s.queue, min(relearnGap, len(s.queue)), it)
		}
	}
	return nil
}

// render redraws the screen for a card, showing the back if flipped.
func (s *session) render(it *item, flipped bool) {
	var b strings.Builder
	b.WriteString(ansiClear)
	fmt.Fprintf(&b, "%s%s · %d left · %d reviewed%s\n\n", ansiDim, s.deck, len(s.queue)+1, s.reviewed, ansiReset)
	if it.isNew {
		fmt.Fprintf(&b, "  %sNEW%s\n\n", ansiBlue, ansiReset)
	}
	fmt.Fprintf(&b, "%s%s%s\n\n", ansiBold, indent(it.card.Front), ansiReset)

	if !flipped {
		fmt.Fprintf(&b, "%s  [space] show answer   [esc] quit%s\n", ansiDim, ansiReset)
	} else {
		fmt.Fprintf(&b, "  %s%s%s\n\n", ansiDim, strings.Repeat("─", 30), ansiReset)
		fmt.Fprintf(&b, "%s\n\n", indent(it.card.Back))
		for _, gk := range gradeKeys {
			fmt.Fprintf(&b, "  %s[%c] %s%s %s%s%s", gk.color, gk.key, gk.name, ansiReset,
				ansiDim, IntervalLabel(it.state, gk.grade), ansiReset)
		}
		fmt.Fprintf(&b, "\n\n%s  [esc] quit%s\n", ansiDim, ansiReset)
	}
	// Raw mode turns off the terminal's \n → \r\n translation, so do it here.
	io.WriteString(s.out, strings.ReplaceAll(b.String(), "\n", "\r\n"))
}

// waitFor blocks until one of keys is pressed (case-insensitive) and returns
// it. Other keys are ignored. Returns errQuit on esc, ctrl-c or ctrl-d.
func (s *session) waitFor(keys string) (byte, error) {
	buf := make([]byte, 16)
	for {
		n, err := s.in.Read(buf)
		if err != nil {
			return 0, err
		}
		if n == 0 {
			continue
		}
		k := buf[0]
		switch {
		case k == 0x1b && n > 1:
			continue // multi-byte escape sequence, e.g. an arrow key
		case k == 0x1b, k == 0x03, k == 0x04: // esc, ctrl-c, ctrl-d
			return 0, errQuit
		case k >= 'A' && k <= 'Z':
			k += 'a' - 'A'
		}
		if strings.IndexByte(keys, k) >= 0 {
			return k, nil
		}
	}
}

// indent indents every line of s by two spaces.
func indent(s string) string {
	return "  " + strings.ReplaceAll(s, "\n", "\n  ")
}
