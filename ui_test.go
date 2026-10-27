// Tests for ui.go: queue building and the review loop driven by scripted key presses.

package main

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestBuildQueue(t *testing.T) {
	cards := []Card{{"new1", ""}, {"late", ""}, {"future", ""}, {"new2", ""}, {"later", ""}}
	states := map[string]State{
		"late":   {Due: testNow.Add(-time.Hour)},
		"later":  {Due: testNow.Add(-48 * time.Hour)},
		"future": {Due: testNow.Add(time.Hour)},
	}
	fronts := func(q []*item) string {
		var s []string
		for _, it := range q {
			s = append(s, it.card.Front)
		}
		return strings.Join(s, ",")
	}
	if got := fronts(buildQueue(cards, states, testNow, 1)); got != "later,late,new1" {
		t.Errorf("limit 1: %s", got)
	}
	if got := fronts(buildQueue(cards, states, testNow, -1)); got != "later,late,new1,new2" {
		t.Errorf("unlimited: %s", got)
	}
}

// keys feeds one keypress per Read, the way a raw-mode terminal delivers them.
type keys []string

func (k *keys) Read(p []byte) (int, error) {
	if len(*k) == 0 {
		return 0, errors.New("test ran out of keys")
	}
	n := copy(p, (*k)[0])
	*k = (*k)[1:]
	return n, nil
}

// newTestSession returns a session over new cards with the given fronts.
func newTestSession(t *testing.T, input keys, fronts ...string) (*session, *strings.Builder) {
	t.Helper()
	var cards []Card
	for _, f := range fronts {
		cards = append(cards, Card{Front: f, Back: f + "-back"})
	}
	out := &strings.Builder{}
	return &session{
		deck:  "test.csv",
		store: openTestStore(t),
		in:    &input,
		out:   out,
		queue: buildQueue(cards, nil, testNow, -1),
	}, out
}

// shownOrder lists the card fronts in the order their front side was drawn.
func shownOrder(out string) string {
	var order []string
	for _, frame := range strings.Split(out, ansiClear) {
		if !strings.Contains(frame, "show answer") {
			continue
		}
		for _, f := range []string{"a", "b", "c"} {
			if strings.Contains(frame, ansiBold+"  "+f+ansiReset) {
				order = append(order, f)
			}
		}
	}
	return strings.Join(order, ",")
}

func TestSessionGradesAndRequeuesAgain(t *testing.T) {
	input := keys{
		"e", "x", "\x1b[A", " ", "q", // grade keys and arrows are ignored before flipping; a → again
		" ", " ", "E", // extra space ignored after flipping; uppercase works; b → good
		" ", "r", // c → easy
		" ", "w", // a again, testNow hard
	}
	s, out := newTestSession(t, input, "a", "b", "c")
	if err := s.run(); err != nil {
		t.Fatal(err)
	}
	if s.reviewed != 4 {
		t.Errorf("reviewed = %d, want 4", s.reviewed)
	}
	if got := shownOrder(out.String()); got != "a,b,c,a" {
		t.Errorf("shown order = %s, want a,b,c,a", got)
	}

	states := mustLoad(t, s.store, "test.csv")
	want := map[string]State{
		"a": {Ease: startEase + againEase + hardEase, Interval: 1},
		"b": {Ease: startEase, Interval: 1},
		"c": {Ease: startEase + easyEase, Interval: 4},
	}
	for f, w := range want {
		got := states[f]
		if got.Interval != w.Interval || got.Ease != w.Ease {
			t.Errorf("%s: ease=%v interval=%d, want ease=%v interval=%d", f, got.Ease, got.Interval, w.Ease, w.Interval)
		}
	}
}

func TestSessionAgainWithOneCardRepeatsIt(t *testing.T) {
	s, out := newTestSession(t, keys{" ", "q", " ", "q", " ", "e"}, "a")
	if err := s.run(); err != nil {
		t.Fatal(err)
	}
	if got := shownOrder(out.String()); got != "a,a,a" {
		t.Errorf("shown order = %s, want a,a,a", got)
	}
}

func TestSessionQuitKeepsAnsweredCards(t *testing.T) {
	for name, quit := range map[string]string{"esc": "\x1b", "ctrl-c": "\x03", "ctrl-d": "\x04"} {
		t.Run(name, func(t *testing.T) {
			s, _ := newTestSession(t, keys{" ", "e", " ", quit}, "a", "b")
			if err := s.run(); !errors.Is(err, errQuit) {
				t.Fatalf("err = %v, want errQuit", err)
			}
			states := mustLoad(t, s.store, "test.csv")
			if _, ok := states["a"]; !ok || len(states) != 1 || s.reviewed != 1 {
				t.Errorf("saved %v, reviewed %d; want only a saved and 1 reviewed", states, s.reviewed)
			}
		})
	}
}
