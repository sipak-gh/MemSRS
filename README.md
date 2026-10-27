# memsrs

Terminal flashcards with spaced repetition (think minimal Anki).

Requires Go 1.26 or newer.

```
go build -o memsrs .
./memsrs [-new 20] deck.csv
```

`-new` limits how many new cards are introduced per day (default 20, `-1` for no limit).

## Cards

The CSV has `front,back` rows:

- An optional header row (`front,back`, `question,answer` or `term,definition`) is skipped.
- Extra columns are ignored.
- Quoted fields may contain commas and newlines.

## Keys

| Key     | Action                                    |
|---------|-------------------------------------------|
| `space` | show answer                               |
| `q`     | again (re-shown 3 cards later)            |
| `w`     | hard                                      |
| `e`     | good                                      |
| `r`     | easy                                      |
| `esc`   | quit (progress is saved after every card) |

## Scheduling

Each card has an interval (days) and an ease (growth rate, starting at 2.5):

- **Good** multiplies the interval by the ease.
- **Hard** multiplies it by 1.2 and lowers the ease.
- **Easy** grows it more than Good and raises the ease.
- **Again** resets the interval and lowers the ease.

New or forgotten cards come back after 1 day (Hard/Good) or 4 days (Easy). Cards become due at midnight, so "1d" means any time tomorrow. The tuning constants and full rules are at the top of `srs.go`.

## Storage

Progress is stored in `srs.db` (SQLite) next to the CSV. Delete it to start over.

- One row per reviewed card, keyed by CSV file name and card front. Cards with no row are new.
- Renaming the CSV or changing a card's front starts that card over.
- Editing a card's back keeps its progress.

## Code

| File       | Purpose                                         |
|------------|-------------------------------------------------|
| `main.go`  | flags, terminal setup, end-of-session summary   |
| `deck.go`  | CSV parsing                                     |
| `srs.go`   | scheduling algorithm                            |
| `store.go` | SQLite persistence                              |
| `ui.go`    | study queue and interactive review loop         |

## Testing

Run `go test ./...`. Each file has a matching `_test.go`. The review loop is tested with scripted key presses, so no terminal is needed.

`testdata/` has sample decks:

- `spanish.csv` is a clean deck to try the app with: `./memsrs testdata/spanish.csv`. This creates `testdata/srs.db`.
- `messy.csv` is used by the CSV tests and covers edge cases.
