// Package refreshpolicy defines conservative polling cadences shared by the
// terminal and graphical frontends.
package refreshpolicy

import "time"

type Class uint8

const (
	Operational Class = iota
	Inventory
	Metered
	Metrics
	Manual
)

func Interval(base time.Duration, class Class) time.Duration {
	if base <= 0 {
		base = 5 * time.Second
	}
	minimum := time.Duration(0)
	switch class {
	case Inventory:
		minimum = 30 * time.Second
	case Metered, Metrics:
		minimum = time.Minute
	case Manual:
		return 0
	default:
		minimum = 5 * time.Second
	}
	if base < minimum {
		return minimum
	}
	return base
}

func Due(last, now time.Time, base time.Duration, class Class) bool {
	interval := Interval(base, class)
	return interval > 0 && (last.IsZero() || !now.Before(last.Add(interval)))
}
