//go:build race

package variables

// raceEnabled reports whether the suite is built with the race detector, which
// together with coverage costs roughly 20x and so needs a different time budget.
const raceEnabled = true
