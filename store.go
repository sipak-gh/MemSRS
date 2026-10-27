// store.go — persisting card state in SQLite (srs.db next to the CSV).
//
// One database can hold several decks (all CSVs in the same directory). A
// card is identified by (deck file name, card front). Cards that have never
// been reviewed have no row; they are "new".

package main

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver, registers "sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS cards (
	deck          TEXT    NOT NULL, -- CSV file name, e.g. "spanish.csv"
	front         TEXT    NOT NULL,
	ease          REAL    NOT NULL,
	interval_days INTEGER NOT NULL,
	due           INTEGER NOT NULL, -- unix seconds
	introduced    INTEGER NOT NULL, -- unix seconds of first review (for the daily new-card limit)
	PRIMARY KEY (deck, front)
);`

// Store reads and writes card state.
type Store struct{ db *sql.DB }

// OpenStore opens (creating if needed) the database at path.
func OpenStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// Load returns the state of every reviewed card in deck, keyed by front.
func (s *Store) Load(deck string) (map[string]State, error) {
	rows, err := s.db.Query(`SELECT front, ease, interval_days, due FROM cards WHERE deck = ?`, deck)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	states := map[string]State{}
	for rows.Next() {
		var front string
		var st State
		var due int64
		if err := rows.Scan(&front, &st.Ease, &st.Interval, &due); err != nil {
			return nil, err
		}
		st.Due = time.Unix(due, 0)
		states[front] = st
	}
	return states, rows.Err()
}

// NewIntroducedSince counts cards in deck whose first review was at or after t.
func (s *Store) NewIntroducedSince(deck string, t time.Time) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM cards WHERE deck = ? AND introduced >= ?`,
		deck, t.Unix()).Scan(&n)
	return n, err
}

// Save stores a card's state after a review made at time now. The first save
// of a card also records now as its "introduced" time; later saves keep it.
func (s *Store) Save(deck, front string, st State, now time.Time) error {
	_, err := s.db.Exec(`INSERT INTO cards (deck, front, ease, interval_days, due, introduced)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (deck, front) DO UPDATE SET
			ease = excluded.ease, interval_days = excluded.interval_days, due = excluded.due`,
		deck, front, st.Ease, st.Interval, st.Due.Unix(), now.Unix())
	return err
}
