package timer

import (
	"math"
	"strconv"
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

	const maxDuration = time.Duration(1<<63 - 1)
	if intervalMin > int(maxDuration/time.Minute) {
		return d // The requested interval cannot be represented.
	}
	interval := time.Duration(intervalMin) * time.Minute
	if d <= 0 {
		return 0
	}

	switch mode {
	case RoundCeil:
		if remainder := d % interval; remainder != 0 {
			delta := interval - remainder
			if d > maxDuration-delta {
				return maxDuration
			}
			return d + delta
		}
		return d
	case RoundFloor:
		return d.Truncate(interval)
	case RoundNearest:
		return d.Round(interval)
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
	if n >= 0 && n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}
