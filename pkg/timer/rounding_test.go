package timer

import (
	"testing"
	"time"
)

func TestRoundDuration(t *testing.T) {
	cases := []struct {
		duration time.Duration
		interval int
		mode     RoundMode
		expected time.Duration
	}{
		{duration: 7 * time.Minute, interval: 15, mode: RoundCeil, expected: 15 * time.Minute},
		{duration: 16 * time.Minute, interval: 15, mode: RoundCeil, expected: 30 * time.Minute},
		{duration: 7 * time.Minute, interval: 15, mode: RoundFloor, expected: 0},
		{duration: 16 * time.Minute, interval: 15, mode: RoundFloor, expected: 15 * time.Minute},
		{duration: 7 * time.Minute, interval: 15, mode: RoundNearest, expected: 0},
		{duration: 8 * time.Minute, interval: 15, mode: RoundNearest, expected: 15 * time.Minute},
		{duration: 4 * time.Minute, interval: 5, mode: RoundCeil, expected: 5 * time.Minute},
		{duration: 4 * time.Minute, interval: 5, mode: RoundNone, expected: 4 * time.Minute},
	}

	for _, c := range cases {
		got := RoundDuration(c.duration, c.interval, c.mode)
		if got != c.expected {
			t.Errorf("RoundDuration(%v, %d, %v) = %v, expected %v", c.duration, c.interval, c.mode, got, c.expected)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	d := 1*time.Hour + 23*time.Minute + 45*time.Second
	if got := FormatDurationHHMMSS(d); got != "01:23:45" {
		t.Errorf("FormatDurationHHMMSS = %s, expected 01:23:45", got)
	}
	if got := FormatDurationHHMM(d); got != "01:24" { // 23m45s rounds to 24m
		t.Errorf("FormatDurationHHMM = %s, expected 01:24", got)
	}
	if got := DecimalHours(1*time.Hour + 30*time.Minute); got != 1.5 {
		t.Errorf("DecimalHours = %f, expected 1.50", got)
	}
}
