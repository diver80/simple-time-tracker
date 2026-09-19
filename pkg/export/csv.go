package export

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"yokto-time/pkg/db"
	"yokto-time/pkg/timer"
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

// GenerateMonthlyCSV generates a CSV representation of the monthly time bookings.
func GenerateMonthlyCSV(repo db.Repository, opts ExportOptions) (string, *MonthlySummary, error) {
	entries, err := repo.ListEntriesForMonth(opts.Year, opts.Month, opts.CustomerID, opts.ProjectID)
	if err != nil {
		return "", nil, fmt.Errorf("failed to list entries for month: %w", err)
	}

	// Cache project hourly rates
	projectRates := make(map[int64]float64)
	allProjects, _ := repo.ListProjects(nil)
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

	for _, e := range entries {
		summary.TotalEntries++

		dateStr := e.StartedAt.Format("2006-01-02")
		startStr := e.StartedAt.Format("15:04")
		endStr := ""
		if e.EndedAt != nil {
			endStr = e.EndedAt.Format("15:04")
		}

		rawDur := time.Duration(e.DurationSec) * time.Second
		roundedDur := timer.RoundDuration(rawDur, opts.RoundingMin, opts.RoundMode)
		decHours := timer.DecimalHours(roundedDur)

		billableStr := "Nein"
		rate := 0.0
		lineTotal := 0.0

		summary.TotalHours += decHours
		if e.IsBillable {
			billableStr = "Ja"
			summary.BillableHours += decHours
			if e.ProjectID != nil {
				rate = projectRates[*e.ProjectID]
				lineTotal = decHours * rate
				summary.TotalRevenue += lineTotal
			}
		}

		custName := e.CustomerName
		if custName == "" {
			custName = "-"
		}
		projName := e.ProjectName
		if projName == "" {
			projName = "-"
		}

		row := []string{
			dateStr,
			custName,
			projName,
			e.TaskName,
			e.BookingText,
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

	// Total row
	summaryRow := []string{
		"SUMME",
		"",
		"",
		"",
		"",
		"",
		"",
		timer.FormatDurationHHMM(time.Duration(summary.TotalHours * float64(time.Hour))),
		fmt.Sprintf("%.2f", summary.TotalHours),
		"",
		"",
		fmt.Sprintf("%.2f", summary.TotalRevenue),
	}
	_ = writer.Write(summaryRow)

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", nil, err
	}

	summary.FormattedTotal = timer.FormatDurationHHMM(time.Duration(summary.TotalHours * float64(time.Hour)))
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
