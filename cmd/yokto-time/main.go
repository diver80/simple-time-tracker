package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"yokto-time/pkg/db"
	"yokto-time/pkg/timer"
	"yokto-time/pkg/ui"
	"yokto-time/pkg/window"

	"github.com/gogpu/gogpu"
	"github.com/gogpu/ui/app"
	"github.com/gogpu/ui/desktop"
	"github.com/gogpu/ui/theme"
	"github.com/gogpu/ui/widget"
)

var (
	version = "1.0.0"
)

func main() {
	dbFlag := flag.String("db", "", "Path to SQLite database file")
	demoFlag := flag.Bool("demo", true, "Auto-seed demo clients and projects if database is empty")
	flag.Parse()

	dbPath := *dbFlag
	if dbPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			dbPath = "yokto-time.db"
		} else {
			dbPath = filepath.Join(home, "Library", "Application Support", "yokto-time", "data.db")
		}
	}

	fmt.Printf("⏱️ Starting Yokto Time Tracker v%s...\n", version)
	fmt.Printf("📦 Database: %s\n", dbPath)

	// 1. Initialize SQLite Database
	repo, err := db.NewRepository(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database at %s: %v", dbPath, err)
	}
	defer repo.Close()

	// 2. Seed initial sample data if empty and demo mode enabled
	if *demoFlag {
		custs, err := repo.ListCustomers()
		if err == nil && len(custs) == 0 {
			seedInitialData(repo)
		}
	}

	// 3. Initialize Timer Service
	timerSvc := timer.NewTimerService(repo)
	defer timerSvc.Close()

	// 4. Initialize Window & Menu Bar Manager
	winMgr := window.NewWindowManager()

	// 5. Initialize Gogpu engine
	gogpuApp := gogpu.NewApp(gogpu.Config{
		Title:  "Yokto Time Tracker",
		Width:  420,
		Height: 580,
	})

	// 6. Transparent theme so macOS window has rounded corners with zero white artifact
	transparentTheme := theme.DefaultDark()
	transparentTheme.Colors.Background = widget.RGBA8(0, 0, 0, 0)
	transparentTheme.Colors.Surface = widget.RGBA8(0, 0, 0, 0)

	// 7. Create UI Application
	uiApp := app.New(
		app.WithWindowProvider(gogpuApp),
		app.WithPlatformProvider(gogpuApp),
		app.WithEventSource(gogpuApp.EventSource()),
		app.WithTheme(transparentTheme),
		app.WithRenderMode(app.RenderModeFrameworkManaged),
	)

	// 8. Create Root App View
	rootView := ui.NewAppView(
		repo,
		timerSvc,
		winMgr,
		func() {
			gogpuApp.RequestRedraw()
		},
	)
	rootView.SetOnResize(func(w, h int) {
		uiApp.Window().HandleResize(w, h)
		gogpuApp.RequestRedraw()
	})

	// 9. Wire Menu Bar Status Item Callbacks
	go func() {
		time.Sleep(300 * time.Millisecond)
		winMgr.InitStatusItem(window.StatusCallbacks{
			OnToggleWindow: func() {
				winMgr.TogglePopover(420, 580)
				gogpuApp.RequestRedraw()
			},
			OnDayView: func() {
				winMgr.ShowPopover(420, 640)
				rootView.SwitchToCalendar()
				gogpuApp.RequestRedraw()
			},
			OnExport: func() {
				winMgr.ShowPopover(420, 580)
				rootView.SwitchToExport()
				gogpuApp.RequestRedraw()
			},
			OnQuickShift: func() {
				winMgr.ShowPopover(420, 580)
				rootView.TriggerQuickShift()
				gogpuApp.RequestRedraw()
			},
			OnQuit: func() {
				os.Exit(0)
			},
		})
	}()

	// 10. Hook status title ticker to live duration
	timerSvc.OnTick(func(elapsed time.Duration, state timer.TimerState, entry *db.TimeEntry) {
		switch state {
		case timer.StateRunning:
			winMgr.UpdateStatusTitle(fmt.Sprintf("⏱️ %s", timer.FormatDurationHHMMSS(elapsed)))
		case timer.StateQuickShift:
			winMgr.UpdateStatusTitle(fmt.Sprintf("⚡ %s", timer.FormatDurationHHMMSS(elapsed)))
		case timer.StatePaused:
			winMgr.UpdateStatusTitle(fmt.Sprintf("⏸️ %s", timer.FormatDurationHHMMSS(elapsed)))
		default:
			winMgr.UpdateStatusTitle("⏱️ Yokto")
		}
	})

	fmt.Println("🚀 Yokto Time is now active in your macOS Menu Bar!")
	fmt.Println("👉 Click the '⏱️ Yokto' icon in the top right menu bar to open.")

	// 11. Run desktop pipeline
	if err := desktop.Run(gogpuApp, uiApp); err != nil {
		log.Fatalf("Desktop execution stopped: %v", err)
	}
}

func seedInitialData(repo db.Repository) {
	fmt.Println("🌱 Seeding initial demo customers and projects...")
	cust1, err := repo.CreateCustomer("Acme Corporation")
	if err == nil && cust1 != nil {
		p1, _ := repo.CreateProject(cust1.ID, "Web Platform Redesign", 125.0, 40.0, 5000.0, "#3B82F6")
		p2, _ := repo.CreateProject(cust1.ID, "Security & Performance Audit", 140.0, 15.0, 2100.0, "#10B981")

		// Add sample historical entries for today's calendar view
		now := time.Now().UTC()
		start1 := time.Date(now.Year(), now.Month(), now.Day(), 9, 30, 0, 0, time.UTC)
		end1 := start1.Add(90 * time.Minute)
		if p1 != nil {
			e1, _ := repo.StartTimeEntry(&p1.ID, "Figma Wireframing & Layout", true)
			if e1 != nil {
				e1.StartedAt = start1
				e1.EndedAt = &end1
				e1.DurationSec = 5400
				e1.BookingText = "- Wireframed dashboard components\n- Approved typography tokens"
				_ = repo.UpdateEntry(e1)
			}
		}

		start2 := time.Date(now.Year(), now.Month(), now.Day(), 11, 15, 0, 0, time.UTC)
		end2 := start2.Add(45 * time.Minute)
		if p2 != nil {
			e2, _ := repo.StartTimeEntry(&p2.ID, "Cloudflare SSL & DNS check", true)
			if e2 != nil {
				e2.StartedAt = start2
				e2.EndedAt = &end2
				e2.DurationSec = 2700
				e2.BookingText = "Verified SSL renewals and HSTS headers"
				_ = repo.UpdateEntry(e2)
			}
		}
	}

	cust2, err := repo.CreateCustomer("Starlight Media")
	if err == nil && cust2 != nil {
		_, _ = repo.CreateProject(cust2.ID, "iOS & Mac Mobile App", 150.0, 60.0, 9000.0, "#8B5CF6")
	}
}
