// Tests for srs.go: the scheduling rules.

package main

import (
	"testing"
	"time"
)

// testNow is the fixed "current time" used across tests: mid-afternoon, so
// start-of-day and due-date rounding are visible.
var testNow = time.Date(2026, 10, 26, 15, 0, 0, 0, time.Local)

func TestScheduleNewCard(t *testing.T) {
	for g, want := range map[Grade]int{Hard: 1, Good: 1, Easy: 4} {
		st := Schedule(NewState(), g, testNow)
		if st.Interval != want {
			t.Errorf("grade %d: interval=%d, want %d", g, st.Interval, want)
		}
		if wantDue := startOfDay(testNow).AddDate(0, 0, want); !st.Due.Equal(wantDue) {
			t.Errorf("grade %d: due=%v, want %v", g, st.Due, wantDue)
		}
	}
}

func TestScheduleProgression(t *testing.T) {
	st := NewState()
	var got []int
	for range 4 {
		st = Schedule(st, Good, testNow)
		got = append(got, st.Interval)
	}
	want := []int{1, 3, 8, 20} // 1×2.5=2.5→3, 3×2.5=7.5→8, 8×2.5=20
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("intervals = %v, want %v", got, want)
		}
	}
}

func TestScheduleGradesOrdered(t *testing.T) {
	// Low ease and a short interval used to make Hard and Good equal.
	for _, st := range []State{{Ease: 2.5, Interval: 20}, {Ease: minEase, Interval: 3}, {Ease: 2.65, Interval: 4}} {
		h, g, e := intervalFor(st, Hard), intervalFor(st, Good), intervalFor(st, Easy)
		if !(st.Interval < h && h < g && g < e) {
			t.Errorf("%+v: want %d < hard < good < easy, got %d %d %d", st, st.Interval, h, g, e)
		}
	}
}

func TestScheduleAgain(t *testing.T) {
	st := Schedule(Schedule(NewState(), Good, testNow), Good, testNow)
	st = Schedule(st, Again, testNow)
	if st.Interval != 0 || !st.IsDue(testNow) {
		t.Errorf("after again: %+v", st)
	}
	for range 20 {
		st = Schedule(st, Again, testNow)
	}
	if st.Ease != minEase {
		t.Errorf("ease = %v, want floor %v", st.Ease, minEase)
	}
}
