// srs.go — the spaced-repetition scheduler (SM-2 style, modelled on Anki).
//
// Every card has two numbers:
//
//   - Interval: days between reviews. 0 means the card is new or was just
//     forgotten ("learning").
//   - Ease: how fast the interval grows on a successful review (starts at 2.5).
//
// Answering a card updates them like this:
//
//	Again  forgotten: interval → 0, ease −0.20, card is shown again this session
//	Hard   interval × 1.2,               ease −0.15
//	Good   interval × ease
//	Easy   interval × ease × 1.3,        ease +0.15
//
// A learning card has no interval to multiply, so it gets fixed intervals
// instead: Hard/Good → 1 day, Easy → 4 days. For cards with an interval,
// Hard < Good < Easy always holds and every interval grows by at least a day.
//
// Due dates fall on midnight, so a card due in "1d" can be studied at any
// time tomorrow rather than exactly 24 hours later.

package main

import (
	"fmt"
	"math"
	"time"
)

// Grade is the user's answer to a card.
type Grade int

const (
	Again Grade = iota + 1 // [q]
	Hard                   // [w]
	Good                   // [e]
	Easy                   // [r]
)

// Tuning knobs for the scheduler. See the file comment for how they're used.
const (
	startEase = 2.5 // ease of a brand-new card
	minEase   = 1.3 // ease never drops below this

	againEase = -0.20 // ease change per grade
	hardEase  = -0.15
	easyEase  = +0.15

	hardFactor = 1.2 // Hard multiplies the interval by this
	easyBonus  = 1.3 // Easy multiplies the interval by ease × this

	learnHardGood = 1 // days until next review for a learning card answered Hard/Good
	learnEasy     = 4 // ... and answered Easy
)

// State is the SRS metadata stored for one card.
type State struct {
	Ease     float64
	Interval int       // days; 0 = new or forgotten
	Due      time.Time // card is shown in sessions at or after this time
}

// NewState is the state of a card that has never been reviewed.
func NewState() State { return State{Ease: startEase} }

// IsDue reports whether the card should be reviewed at time now.
func (s State) IsDue(now time.Time) bool { return !s.Due.After(now) }

// Schedule returns the card's state after it is answered with g at time now.
func Schedule(s State, g Grade, now time.Time) State {
	if g == Again {
		return State{Ease: math.Max(minEase, s.Ease+againEase), Due: now}
	}
	next := State{Ease: s.Ease, Interval: intervalFor(s, g)}
	switch g {
	case Hard:
		next.Ease = math.Max(minEase, s.Ease+hardEase)
	case Easy:
		next.Ease += easyEase
	}
	next.Due = startOfDay(now).AddDate(0, 0, next.Interval)
	return next
}

// intervalFor returns the days until the next review if s is answered with
// Hard, Good or Easy.
func intervalFor(s State, g Grade) int {
	hard, good, easy := learnHardGood, learnHardGood, learnEasy
	if s.Interval > 0 {
		hard = max(s.Interval+1, scale(s.Interval, hardFactor))
		good = max(hard+1, scale(s.Interval, s.Ease))
		easy = max(good+1, scale(s.Interval, s.Ease*easyBonus))
	}
	switch g {
	case Hard:
		return hard
	case Easy:
		return easy
	default:
		return good
	}
}

// IntervalLabel is the short "when will I see this again" text shown next to
// each grade, e.g. "now", "3d", "1.5mo", "2.1y".
func IntervalLabel(s State, g Grade) string {
	if g == Again {
		return "now"
	}
	d := intervalFor(s, g)
	switch {
	case d < 30:
		return fmt.Sprintf("%dd", d)
	case d < 365:
		return fmt.Sprintf("%.1fmo", float64(d)/30)
	default:
		return fmt.Sprintf("%.1fy", float64(d)/365)
	}
}

func scale(days int, f float64) int { return int(math.Round(float64(days) * f)) }

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}
