package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"time-tracker/pkg/db"
	"time-tracker/pkg/timer"
	"time-tracker/pkg/ui"
	"time-tracker/pkg/window"

	_ "github.com/gogpu/gg/gpu" // Register the renderer used by desktop.Run's GPU compositor.
	"github.com/gogpu/gogpu"
	"github.com/gogpu/ui/app"
	"github.com/gogpu/ui/desktop"
	"github.com/gogpu/ui/theme"
	"github.com/gogpu/ui/widget"
)

var (
	version = "1.0.6"
)

func resolveDefaultDBPath(homeDir string) string {
	if homeDir == "" {
		return "simple-time-tracker.db"
	}
	dir := filepath.Join(homeDir, "Library", "Application Support", "simple-time-tracker")
	_ = os.MkdirAll(dir, 0755)
	dbPath := filepath.Join(dir, "data.db")

	migrateLegacyDB(dbPath, []string{
		filepath.Join(homeDir, "Library", "Application Support", "time-tracker", "data.db"),
		filepath.Join(homeDir, "Library", "Application Support", "yokto-time", "data.db"),
	})

	return dbPath
}

func defaultDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "simple-time-tracker.db"
	}
	return resolveDefaultDBPath(home)
}

func migrateLegacyDB(destPath string, legacyPaths []string) bool {
	if _, err := os.Stat(destPath); err == nil {
		return false // Destination already exists
	}
	for _, legacyPath := range legacyPaths {
		if data, err := os.ReadFile(legacyPath); err == nil {
			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err == nil {
				if err := os.WriteFile(destPath, data, 0644); err == nil {
					fmt.Printf("📦 Migrating existing database from %s to %s...\n", legacyPath, destPath)
					for _, ext := range []string{"-wal", "-shm"} {
						legacyExtra := legacyPath + ext
						if extraData, err := os.ReadFile(legacyExtra); err == nil {
							_ = os.WriteFile(destPath+ext, extraData, 0644)
						}
					}
					return true
				}
			}
		}
	}
	return false
}

func main() {
	dbFlag := flag.String("db", "", "Path to SQLite database file")
	demoFlag := flag.Bool("demo", false, "Auto-seed demo clients and projects if database is empty")
	flag.Parse()

	dbPath := *dbFlag
	if dbPath == "" {
		dbPath = defaultDBPath()
	}

	fmt.Printf("⏱️ Starting Simple Time Tracker (stt)...\n")
	if version != "" {
		fmt.Printf("ℹ️  Version: %s\n", version)
	}
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
	gogpuApp := gogpu.NewApp(gogpu.DefaultConfig().
		WithTitle("Simple Time Tracker").
		WithAppName("Simple Time Tracker").
		WithSize(420, 580).
		WithResizable(false))

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

	// 8. Mount the UI and invalidate its retained scene, not just the OS window.
	updates := &desktopUpdates{wake: gogpuApp.RequestRedraw}
	rootView := ui.NewAppView(repo, timerSvc, winMgr, updates.requestRedraw)
	uiApp.SetRoot(rootView)
	rootView.SetOnResize(func(w, h int) {
		uiApp.Window().HandleResize(w, h)
		updates.requestRedraw()
	})

	// 9. Initialize native controls after the window actually exists. Native
	// menu callbacks run on Cocoa's thread; widgets belong to the render thread.
	statusInitialized := false
	gogpuApp.OnUpdate(func(float64) {
		if !statusInitialized {
			statusInitialized = true
			winMgr.InitStatusItem(window.StatusCallbacks{
				OnToggleWindow: func() {
					// Bounds() is synchronized by WidgetBase. Show the native
					// window before relying on a render callback to run.
					size := rootView.Bounds().Size()
					if size.Width <= 0 || size.Height <= 0 {
						winMgr.TogglePopover(420, 580)
					} else {
						winMgr.TogglePopover(int(size.Width), int(size.Height))
					}
					updates.requestRedraw()
				},
				OnDayView: func() {
					winMgr.ShowPopover(420, 640)
					updates.post(rootView.SwitchToCalendar)
				},
				OnExport: func() {
					winMgr.ShowPopover(420, 580)
					updates.post(rootView.SwitchToExport)
				},
				OnQuickShift: func() {
					winMgr.ShowPopover(420, 580)
					updates.post(rootView.TriggerQuickShift)
				},
				OnQuit: func() {
					gogpuApp.Quit()
					os.Exit(0)
				},
			})
			if primary := gogpuApp.PrimaryWindow(); runtime.GOOS == "darwin" && primary != nil {
				primary.SetOnClose(func() bool {
					winMgr.HidePopover()
					return false // Keep the timer and status item alive.
				})
			}
			fmt.Println("🚀 Simple Time Tracker is ready. Click '⏱️ Time' to show/hide; right-click for actions.")
		}
	})
	uiApp.SetFrameCallback(func(app.FrameStats) {
		updates.flush(func() {
			rootView.SetNeedsRedraw(true)
			uiApp.Window().Context().Invalidate()
		})
	})

	// 10. Hook status title ticker to live duration
	timerSvc.OnTick(func(elapsed time.Duration, state timer.TimerState, entry *db.TimeEntry) {
		if rootView.IsShieldActive() {
			winMgr.UpdateStatusTitle("⏱️ Time")
			return
		}
		switch state {
		case timer.StateRunning:
			winMgr.UpdateStatusTitle(fmt.Sprintf("⏱️ %s", timer.FormatDurationHHMMSS(elapsed)))
		case timer.StateQuickShift:
			winMgr.UpdateStatusTitle(fmt.Sprintf("⚡ %s", timer.FormatDurationHHMMSS(elapsed)))
		case timer.StatePaused:
			winMgr.UpdateStatusTitle(fmt.Sprintf("⏸️ %s", timer.FormatDurationHHMMSS(elapsed)))
		default:
			winMgr.UpdateStatusTitle("⏱️ Time")
		}
	})

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
