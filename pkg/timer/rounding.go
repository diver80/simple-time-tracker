package timer

import (
	"math"
	"time"
)

type RoundMode int

const (
	RoundNone RoundMode = iota
	RoundNearest
	RoundCeil
	RoundFloor
)

// RoundDuration rounds a given duration to the specified interval in minutes
// according to the RoundMode (RoundNone, RoundNearest, RoundCeil, RoundFloor).
func RoundDuration(d time.Duration, intervalMin int, mode RoundMode) time.Duration {
	if intervalMin <= 0 || mode == RoundNone {
		return d
	}

	interval := time.Duration(intervalMin) * time.Minute
	if d <= 0 {
		return 0
	}

	switch mode {
	case RoundCeil:
		return ((d + interval - 1) / interval) * interval
	case RoundFloor:
		return (d / interval) * interval
	case RoundNearest:
		half := interval / 2
		return ((d + half) / interval) * interval
	default:
		return d
	}
}

// FormatDurationHHMM formats a duration as HH:MM.
func FormatDurationHHMM(d time.Duration) string {
	totalMinutes := int(math.Round(d.Minutes()))
	h := totalMinutes / 60
	m := totalMinutes % 60
	return formatTwoDigits(h) + ":" + formatTwoDigits(m)
}

// FormatDurationHHMMSS formats a duration as HH:MM:SS.
func FormatDurationHHMMSS(d time.Duration) string {
	totalSec := int(d.Seconds())
	h := totalSec / 3600
	m := (totalSec % 3600) / 60
	s := totalSec % 60
	return formatTwoDigits(h) + ":" + formatTwoDigits(m) + ":" + formatTwoDigits(s)
}

// DecimalHours returns the duration in fractional hours (e.g. 1h 30m -> 1.50).
func DecimalHours(d time.Duration) float64 {
	return math.Round((d.Hours())*100) / 100
}

func formatTwoDigits(n int) string {
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	return string(rune('0'+(n/10)%10)) + string(rune('0'+n%10))
}
