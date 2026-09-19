package export

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"
	"unicode"

	"time-tracker/pkg/db"
	"time-tracker/pkg/timer"
)

type ExportOptions struct {
	Year        int
	Month       time.Month
	CustomerID  *int64
	ProjectID   *int64
	RoundingMin int             // 0, 5, 15
	RoundMode   timer.RoundMode // RoundNone, RoundNearest, RoundCeil, RoundFloor
}

type MonthlySummary struct {
	TotalEntries   int
	TotalHours     float64
	BillableHours  float64
	TotalRevenue   float64
	FormattedTotal string
}

// sanitizeForSpreadsheet guards against formula injection in spreadsheet applications.
// It checks for formula-trigger characters (=, +, -, @, tab, CR, LF) after leading spaces,
// and prefixes the original (non-trimmed) string with a single quote if found.
// Ordinary leading-space text is kept unchanged.
func sanitizeForSpreadsheet(s string) string {
	for _, ch := range s {
		switch ch {
		case '=', '+', '-', '@', '\t', '\r', '\n':
			return "'" + s
		}
		if !unicode.IsSpace(ch) {
			break
		}
	}
	return s
}

// GenerateMonthlyCSV generates a CSV representation of the monthly time bookings.
func GenerateMonthlyCSV(repo db.Repository, opts ExportOptions) (string, *MonthlySummary, error) {
	entries, err := repo.ListEntriesForMonth(opts.Year, opts.Month, opts.CustomerID, opts.ProjectID)
	if err != nil {
		return "", nil, fmt.Errorf("failed to list entries for month: %w", err)
	}

	// Cache project hourly rates
	projectRates := make(map[int64]float64)
	allProjects, err := repo.ListProjects(nil)
	if err != nil {
		return "", nil, fmt.Errorf("failed to list projects: %w", err)
	}
	for _, p := range allProjects {
		projectRates[p.ID] = p.HourlyRate
	}

	var buf bytes.Buffer
	// Write UTF-8 BOM for Excel / Numbers compatibility
	buf.WriteString("\xEF\xBB\xBF")

	writer := csv.NewWriter(&buf)
	writer.Comma = ';' // Semicolon for German/European spreadsheet compatibility

	// Header row
	header := []string{
		"Datum",
		"Kunde",
		"Projekt",
		"Aufgabe",
		"Buchungstext",
		"Start",
		"Ende",
		"Dauer (HH:MM)",
		"Stunden (Dezimal)",
		"Abrechenbar",
		"Stundensatz (€)",
		"Gesamt (€)",
	}
	if err := writer.Write(header); err != nil {
		return "", nil, err
	}

	summary := &MonthlySummary{}
	var totalRoundedDuration time.Duration
	var totalBillableRoundedDuration time.Duration
	var totalRevenueCents int64

	for _, e := range entries {
		summary.TotalEntries++

		// Use local display timestamps matching local monthly boundaries
		localStarted := e.StartedAt.Local()
		dateStr := localStarted.Format("2006-01-02")
		startStr := localStarted.Format("15:04")
		endStr := ""
		if e.EndedAt != nil {
			endStr = e.EndedAt.Local().Format("15:04")
		}

		rawDur := time.Duration(e.DurationSec) * time.Second
		roundedDur := timer.RoundDuration(rawDur, opts.RoundingMin, opts.RoundMode)
		decHours := timer.DecimalHours(roundedDur)

		billableStr := "Nein"
		rate := 0.0
		lineTotal := 0.0

		// Track total durations using rounded values to avoid drift
		totalRoundedDuration += roundedDur
		if e.IsBillable {
			billableStr = "Ja"
			totalBillableRoundedDuration += roundedDur
			if e.ProjectID != nil {
				rate = projectRates[*e.ProjectID]
				// Compute revenue from roundedDur.Hours() * rate, rounded to cents
				lineRevenue := roundedDur.Hours() * rate
				lineCents := int64(math.Round(lineRevenue * 100))
				lineTotal = float64(lineCents) / 100
				totalRevenueCents += lineCents
			}
		}

		custName := sanitizeForSpreadsheet(e.CustomerName)
		if custName == "" {
			custName = "-"
		}
		projName := sanitizeForSpreadsheet(e.ProjectName)
		if projName == "" {
			projName = "-"
		}

		row := []string{
			dateStr,
			custName,
			projName,
			sanitizeForSpreadsheet(e.TaskName),
			sanitizeForSpreadsheet(e.BookingText),
			startStr,
			endStr,
			timer.FormatDurationHHMM(roundedDur),
			fmt.Sprintf("%.2f", decHours),
			billableStr,
			fmt.Sprintf("%.2f", rate),
			fmt.Sprintf("%.2f", lineTotal),
		}
		if err := writer.Write(row); err != nil {
			return "", nil, err
		}
	}

	summary.TotalRevenue = float64(totalRevenueCents) / 100
	// Calculate totals from accumulated rounded durations
	summary.TotalHours = timer.DecimalHours(totalRoundedDuration)
	summary.BillableHours = timer.DecimalHours(totalBillableRoundedDuration)

	// Total row - use accumulated rounded duration for consistency
	summaryRow := []string{
		"SUMME",
		"",
		"",
		"",
		"",
		"",
		"",
		timer.FormatDurationHHMM(totalRoundedDuration),
		fmt.Sprintf("%.2f", summary.TotalHours),
		"",
		"",
		fmt.Sprintf("%.2f", summary.TotalRevenue),
	}
	if err := writer.Write(summaryRow); err != nil {
		return "", nil, err
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", nil, err
	}

	summary.FormattedTotal = timer.FormatDurationHHMM(totalRoundedDuration)
	return buf.String(), summary, nil
}

// ExportMonthlyCSVToFile writes the generated CSV directly to a target file path.
func ExportMonthlyCSVToFile(repo db.Repository, opts ExportOptions, targetPath string) (*MonthlySummary, error) {
	content, summary, err := GenerateMonthlyCSV(repo, opts)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return nil, err
	}

	if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
		return nil, err
	}
	return summary, nil
}
