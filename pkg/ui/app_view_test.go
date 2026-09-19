package ui

import (
	"strings"
	"testing"
	"time"

	"time-tracker/pkg/db"
	"time-tracker/pkg/timer"
	"time-tracker/pkg/window"
)

type dummyWinMgr struct {
	hiddenDuration time.Duration
	shieldEnabled  bool
}

func (d *dummyWinMgr) InitStatusItem(cb window.StatusCallbacks) {}
func (d *dummyWinMgr) UpdateStatusTitle(title string)           {}
func (d *dummyWinMgr) SetWindowSize(w, h int)                   {}
func (d *dummyWinMgr) TogglePopover(w, h int)                   {}
func (d *dummyWinMgr) ShowPopover(w, h int)                     {}
func (d *dummyWinMgr) HidePopover()                             {}
func (d *dummyWinMgr) IsVisible() bool                          { return true }
func (d *dummyWinMgr) SetScreenShareShield(enable bool)         { d.shieldEnabled = enable }
func (d *dummyWinMgr) IsScreenShareShieldEnabled() bool         { return d.shieldEnabled }
func (d *dummyWinMgr) TempHide(duration time.Duration)          { d.hiddenDuration = duration }

func TestAppViewNavBarTimerVisibility(t *testing.T) {
	repo, err := db.NewRepository(":memory:")
	if err != nil {
		t.Fatalf("Failed to create memory repo: %v", err)
	}
	defer repo.Close()

	timerSvc := timer.NewTimerService(repo)
	defer timerSvc.Close()

	appView := NewAppView(repo, timerSvc, nil, func() {})

	// 1. Initial idle state: tab button should say "⏱️ Tracker"
	if appView.tabTrackerBtn.text != "⏱️ Tracker" {
		t.Errorf("Expected idle label '⏱️ Tracker', got '%s'", appView.tabTrackerBtn.text)
	}

	// 2. Start timer
	entry, err := timerSvc.Start(nil, "Nav Bar Test Task", true)
	if err != nil || entry == nil {
		t.Fatalf("Failed to start timer: %v", err)
	}

	// Switch to Calendar tab (Tag)
	appView.SwitchTab(TabCalendar)
	if appView.activeTab != TabCalendar {
		t.Errorf("Expected active tab TabCalendar, got %v", appView.activeTab)
	}

	// Refresh timer state on UI thread (simulating Draw)
	appView.refreshTimerState()

	// In the nav bar, the tracker tab button must now display the running time (e.g. "⏱️ 00:00:00" or "⏱️ 00:00:01")
	if !strings.HasPrefix(appView.tabTrackerBtn.text, "⏱️ 00:") {
		t.Errorf("Expected nav bar button to display running time, got '%s'", appView.tabTrackerBtn.text)
	}

	// 3. Trigger QuickShift
	_, err = timerSvc.StartQuickShift("Urgent call")
	if err != nil {
		t.Fatalf("Failed to start QuickShift: %v", err)
	}

	// Refresh timer state on UI thread (simulating Draw)
	appView.refreshTimerState()

	// Button must now show QuickShift lightning indicator
	if !strings.HasPrefix(appView.tabTrackerBtn.text, "⚡ 00:") {
		t.Errorf("Expected nav bar button to display QuickShift lightning '⚡', got '%s'", appView.tabTrackerBtn.text)
	}

	// Finish QuickShift
	_ = timerSvc.FinishQuickShift()

	// Refresh timer state on UI thread (simulating Draw)
	appView.refreshTimerState()

	if !strings.HasPrefix(appView.tabTrackerBtn.text, "⏱️ 00:") {
		t.Errorf("Expected nav bar button to resume running time, got '%s'", appView.tabTrackerBtn.text)
	}

	// 4. Stop timer
	_, _ = timerSvc.Stop()

	// Refresh timer state on UI thread (simulating Draw)
	appView.refreshTimerState()

	if appView.tabTrackerBtn.text != "⏱️ Tracker" {
		t.Errorf("Expected nav bar button to reset to '⏱️ Tracker', got '%s'", appView.tabTrackerBtn.text)
	}
}

func TestAppViewPrivacyAndScreenShareShield(t *testing.T) {
	repo, err := db.NewRepository(":memory:")
	if err != nil {
		t.Fatalf("Failed to create memory repo: %v", err)
	}
	defer repo.Close()

	timerSvc := timer.NewTimerService(repo)
	defer timerSvc.Close()

	winMgr := &dummyWinMgr{}
	appView := NewAppView(repo, timerSvc, winMgr, func() {})

	// 1. Initial state: overlay should be closed, privacy not masked
	if appView.showPrivacyOverlay {
		t.Error("Expected privacy overlay to be closed initially")
	}
	if appView.privacyMasked {
		t.Error("Expected privacyMasked to be false initially")
	}

	// 2. Click Shield button -> overlay opens
	appView.tabShieldBtn.onClick()
	if !appView.showPrivacyOverlay {
		t.Error("Expected privacy overlay to be open after clicking tabShieldBtn")
	}

	// 3. Test TempHide for 15m Call
	appView.hide15Btn.onClick()
	if appView.showPrivacyOverlay {
		t.Error("Expected overlay to close after clicking hide15Btn")
	}
	if winMgr.hiddenDuration != 15*time.Minute {
		t.Errorf("Expected winMgr.hiddenDuration == 15m, got %v", winMgr.hiddenDuration)
	}

	// 4. Test TempHide for 30m Call
	appView.tabShieldBtn.onClick()
	appView.hide30Btn.onClick()
	if winMgr.hiddenDuration != 30*time.Minute {
		t.Errorf("Expected winMgr.hiddenDuration == 30m, got %v", winMgr.hiddenDuration)
	}

	// 5. Test Mask Data toggle
	appView.tabShieldBtn.onClick()
	appView.maskToggleBtn.onClick()
	if !appView.privacyMasked {
		t.Error("Expected privacyMasked to be true after clicking maskToggleBtn")
	}
	if !appView.hudView.privacyMasked {
		t.Error("Expected hudView.privacyMasked to be true after clicking maskToggleBtn")
	}

	// Unmask
	appView.maskToggleBtn.onClick()
	if appView.privacyMasked {
		t.Error("Expected privacyMasked to be false after toggling off")
	}

	// 6. Test Close button
	appView.closeOverlayBtn.onClick()
	if appView.showPrivacyOverlay {
		t.Error("Expected overlay to close after clicking closeOverlayBtn")
	}
}

func TestAppViewChildren(t *testing.T) {
	repo, err := db.NewRepository(":memory:")
	if err != nil {
		t.Fatalf("Failed to create memory repo: %v", err)
	}
	defer repo.Close()

	timerSvc := timer.NewTimerService(repo)
	defer timerSvc.Close()

	appView := NewAppView(repo, timerSvc, nil, func() {})

	// Default tab: TabTracker
	chTracker := appView.Children()
	if len(chTracker) != 6 {
		t.Fatalf("expected 6 children for TabTracker, got %d", len(chTracker))
	}
	if chTracker[5] != appView.hudView {
		t.Errorf("expected last child to be hudView, got %v", chTracker[5])
	}

	// TabCalendar
	appView.SwitchTab(TabCalendar)
	chCalendar := appView.Children()
	if len(chCalendar) != 6 || chCalendar[5] != appView.calendarView {
		t.Errorf("expected calendarView child for TabCalendar, got %v", chCalendar)
	}

	// TabExport
	appView.SwitchTab(TabExport)
	chExport := appView.Children()
	if len(chExport) != 6 || chExport[5] != appView.exportView {
		t.Errorf("expected exportView child for TabExport, got %v", chExport)
	}

	// TabProjects
	appView.SwitchTab(TabProjects)
	chProjects := appView.Children()
	if len(chProjects) != 6 || chProjects[5] != appView.projectView {
		t.Errorf("expected projectView child for TabProjects, got %v", chProjects)
	}
}

