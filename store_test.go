// Tests for store.go, each against a fresh SQLite file in a temp dir.

package main

import (
	"path/filepath"
	"testing"
	"time"
)

// openTestStore opens a fresh database in a temp dir, closed when the test ends.
func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := OpenStore(filepath.Join(t.TempDir(), dbName))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// mustLoad loads a deck's states, failing the test on error.
func mustLoad(t *testing.T, s *Store, deck string) map[string]State {
	t.Helper()
	states, err := s.Load(deck)
	if err != nil {
		t.Fatal(err)
	}
	return states
}

// mustSave saves a card's state, failing the test on error.
func mustSave(t *testing.T, s *Store, deck, front string, st State, at time.Time) {
	t.Helper()
	if err := s.Save(deck, front, st, at); err != nil {
		t.Fatal(err)
	}
}

func TestStoreRoundTripAndReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), dbName)
	s, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	st := Schedule(NewState(), Easy, testNow)
	mustSave(t, s, "a.csv", "hola", st, testNow)
	s.Close()

	s, err = OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, ok := mustLoad(t, s, "a.csv")["hola"]
	if !ok || got.Ease != st.Ease || got.Interval != st.Interval || !got.Due.Equal(st.Due) {
		t.Errorf("loaded %+v (found=%v), want %+v", got, ok, st)
	}
}

func TestStoreUpdateOverwritesState(t *testing.T) {
	s := openTestStore(t)
	mustSave(t, s, "a.csv", "hola", Schedule(NewState(), Good, testNow), testNow)
	again := Schedule(NewState(), Again, testNow)
	mustSave(t, s, "a.csv", "hola", again, testNow)

	states := mustLoad(t, s, "a.csv")
	if len(states) != 1 || states["hola"].Interval != 0 || states["hola"].Ease != again.Ease {
		t.Errorf("states = %+v, want one card with %+v", states, again)
	}
}

func TestStoreDecksAreSeparate(t *testing.T) {
	s := openTestStore(t)
	mustSave(t, s, "a.csv", "hola", NewState(), testNow)
	mustSave(t, s, "b.csv", "hola", NewState(), testNow)
	mustSave(t, s, "b.csv", "perro", NewState(), testNow)

	a := mustLoad(t, s, "a.csv")
	b := mustLoad(t, s, "b.csv")
	if len(a) != 1 || len(b) != 2 {
		t.Errorf("a has %d cards, b has %d; want 1 and 2", len(a), len(b))
	}
}

func TestStoreNewIntroducedCountsFirstReviewOnly(t *testing.T) {
	s := openTestStore(t)
	today := startOfDay(testNow)
	yesterday := testNow.Add(-24 * time.Hour)

	mustSave(t, s, "a.csv", "old", NewState(), yesterday)
	mustSave(t, s, "a.csv", "old", NewState(), testNow) // reviewed again today: still not new today
	mustSave(t, s, "a.csv", "new1", NewState(), testNow)
	mustSave(t, s, "a.csv", "new2", NewState(), testNow)
	mustSave(t, s, "b.csv", "other", NewState(), testNow)

	n, err := s.NewIntroducedSince("a.csv", today)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("new today = %d, want 2", n)
	}
}
