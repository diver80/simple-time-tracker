package ui

import (
	"strings"
	"testing"
	"time"

	"time-tracker/pkg/db"
	"time-tracker/pkg/timer"
	"time-tracker/pkg/window"

	"github.com/gogpu/ui/geometry"
)

type dummyWinMgr struct {
	hiddenDuration time.Duration
	shieldEnabled  bool
	statusTitle    string
}

func (d *dummyWinMgr) InitStatusItem(cb window.StatusCallbacks) {}
func (d *dummyWinMgr) UpdateStatusTitle(title string)           { d.statusTitle = title }
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

	// 1. Initial idle state: tab button should say "Tracker"
	if appView.tabTrackerBtn.text != "Tracker" {
		t.Errorf("Expected idle label 'Tracker', got '%s'", appView.tabTrackerBtn.text)
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

	// In the nav bar, the tracker tab button must now display the running time (e.g. "00:00:00" or "00:00:01") without emoji
	if !strings.HasPrefix(appView.tabTrackerBtn.text, "00:") {
		t.Errorf("Expected nav bar button to display running time, got '%s'", appView.tabTrackerBtn.text)
	}
	if strings.ContainsAny(appView.tabTrackerBtn.text, "⏱️⚡⏸️") {
		t.Errorf("Nav bar button label should not contain emojis, got '%s'", appView.tabTrackerBtn.text)
	}

	// 3. Trigger QuickShift
	_, err = timerSvc.StartQuickShift("Urgent call")
	if err != nil {
		t.Fatalf("Failed to start QuickShift: %v", err)
	}

	// Refresh timer state on UI thread (simulating Draw)
	appView.refreshTimerState()

	// Button must now show QuickShift duration without emoji prefix
	if !strings.HasPrefix(appView.tabTrackerBtn.text, "00:") {
		t.Errorf("Expected nav bar button to display QuickShift duration, got '%s'", appView.tabTrackerBtn.text)
	}
	if strings.ContainsAny(appView.tabTrackerBtn.text, "⏱️⚡⏸️") {
		t.Errorf("Nav bar button label should not contain emojis, got '%s'", appView.tabTrackerBtn.text)
	}

	// Finish QuickShift
	_ = timerSvc.FinishQuickShift()

	// Refresh timer state on UI thread (simulating Draw)
	appView.refreshTimerState()

	if !strings.HasPrefix(appView.tabTrackerBtn.text, "00:") {
		t.Errorf("Expected nav bar button to resume running time, got '%s'", appView.tabTrackerBtn.text)
	}
	if strings.ContainsAny(appView.tabTrackerBtn.text, "⏱️⚡⏸️") {
		t.Errorf("Nav bar button label should not contain emojis, got '%s'", appView.tabTrackerBtn.text)
	}

	// 4. Stop timer
	_, _ = timerSvc.Stop()

	// Refresh timer state on UI thread (simulating Draw)
	appView.refreshTimerState()

	if appView.tabTrackerBtn.text != "Tracker" {
		t.Errorf("Expected nav bar button to reset to 'Tracker', got '%s'", appView.tabTrackerBtn.text)
	}
}

func TestAppViewTabLabelsAndLayout(t *testing.T) {
	repo, err := db.NewRepository(":memory:")
	if err != nil {
		t.Fatalf("Failed to create memory repo: %v", err)
	}
	defer repo.Close()

	timerSvc := timer.NewTimerService(repo)
	defer timerSvc.Close()

	appView := NewAppView(repo, timerSvc, nil, func() {})
	appView.SetBounds(geometry.NewRect(0, 0, 420, 580))

	// Verify idle tab button labels: clean text, no emojis
	expectedLabels := map[string]string{
		"Tracker":  appView.tabTrackerBtn.text,
		"Tag":      appView.tabCalendarBtn.text,
		"Export":   appView.tabExportBtn.text,
		"Budgets":  appView.tabProjectsBtn.text,
		"Schutz":   appView.tabShieldBtn.text,
	}
	for expected, actual := range expectedLabels {
		if actual != expected {
			t.Errorf("Expected tab label %q, got %q", expected, actual)
		}
	}

	// Call Draw to calculate and set bounds
	mockCtx := &testWidgetContext{}
	appView.Draw(mockCtx, &testCanvas{})

	// Verify shield button width is exactly 54px
	shieldBounds := appView.tabShieldBtn.Bounds()
	if shieldBounds.Width() != 54 {
		t.Errorf("Expected shield button width 54, got %v", shieldBounds.Width())
	}

	// Verify all tab bounds across 420px container
	trackerBounds := appView.tabTrackerBtn.Bounds()
	calendarBounds := appView.tabCalendarBtn.Bounds()
	exportBounds := appView.tabExportBtn.Bounds()
	projectsBounds := appView.tabProjectsBtn.Bounds()

	if trackerBounds.Min.X != 16 || trackerBounds.Width() != 84 {
		t.Errorf("Tracker tab bounds mismatch: got %+v, expected X=16 W=84", trackerBounds)
	}
	if calendarBounds.Min.X != 106 || calendarBounds.Width() != 64 {
		t.Errorf("Calendar tab bounds mismatch: got %+v, expected X=106 W=64", calendarBounds)
	}
	if exportBounds.Min.X != 176 || exportBounds.Width() != 78 {
		t.Errorf("Export tab bounds mismatch: got %+v, expected X=176 W=78", exportBounds)
	}
	if projectsBounds.Min.X != 260 || projectsBounds.Width() != 84 {
		t.Errorf("Projects tab bounds mismatch: got %+v, expected X=260 W=84", projectsBounds)
	}
	if shieldBounds.Min.X != 350 || shieldBounds.Width() != 54 {
		t.Errorf("Shield tab bounds mismatch: got %+v, expected X=350 W=54", shieldBounds)
	}

	// Check uniform gap between adjacent buttons
	gap1 := calendarBounds.Min.X - trackerBounds.Max.X
	gap2 := exportBounds.Min.X - calendarBounds.Max.X
	gap3 := projectsBounds.Min.X - exportBounds.Max.X
	gap4 := shieldBounds.Min.X - projectsBounds.Max.X
	if gap1 != 6 || gap2 != 6 || gap3 != 6 || gap4 != 6 {
		t.Errorf("Expected uniform 6px gaps, got %v, %v, %v, %v", gap1, gap2, gap3, gap4)
	}

	// Check margins
	if trackerBounds.Min.X != 16 {
		t.Errorf("Expected left margin 16, got %v", trackerBounds.Min.X)
	}
	rightMargin := float32(420) - shieldBounds.Max.X
	if rightMargin != 16 {
		t.Errorf("Expected right margin 16, got %v", rightMargin)
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

	// Test overlay children
	appView.tabShieldBtn.onClick()
	chOverlay := appView.Children()
	if len(chOverlay) != 12 {
		t.Fatalf("expected 12 children when showPrivacyOverlay is true (5 tabs + 7 overlay buttons), got %d", len(chOverlay))
	}
	if chOverlay[5] != appView.telkoToggleBtn {
		t.Errorf("expected chOverlay[5] to be telkoToggleBtn, got %v", chOverlay[5])
	}
	if chOverlay[11] != appView.closeOverlayBtn {
		t.Errorf("expected chOverlay[11] to be closeOverlayBtn, got %v", chOverlay[11])
	}
}

func TestAppViewTelkoMode(t *testing.T) {
	repo, err := db.NewRepository(":memory:")
	if err != nil {
		t.Fatalf("Failed to create memory repo: %v", err)
	}
	defer repo.Close()

	timerSvc := timer.NewTimerService(repo)
	defer timerSvc.Close()

	winMgr := &dummyWinMgr{}
	appView := NewAppView(repo, timerSvc, winMgr, func() {})

	// 1. Initially shield and telko mode should be inactive
	if appView.IsShieldActive() {
		t.Error("Expected IsShieldActive to be false initially")
	}

	// 2. Start timer
	_, err = timerSvc.Start(nil, "Telko Test", true)
	if err != nil {
		t.Fatalf("Failed to start timer: %v", err)
	}
	appView.refreshTimerState()

	// Tracker tab button should show duration
	if !strings.HasPrefix(appView.tabTrackerBtn.text, "00:") {
		t.Errorf("Expected tabTrackerBtn to show elapsed time, got %q", appView.tabTrackerBtn.text)
	}

	// 3. Toggle TelkoMode ON
	appView.ToggleTelkoMode()
	if !appView.IsShieldActive() {
		t.Error("Expected IsShieldActive to be true after ToggleTelkoMode")
	}
	if appView.tabTrackerBtn.text != "Tracker" {
		t.Errorf("Expected tabTrackerBtn to display 'Tracker' while TelkoMode is active, got %q", appView.tabTrackerBtn.text)
	}
	if winMgr.statusTitle != "⏱️ Time" {
		t.Errorf("Expected status title '⏱️ Time', got %q", winMgr.statusTitle)
	}

	// Calling refreshTimerState while shield is active must keep "Tracker"
	appView.refreshTimerState()
	if appView.tabTrackerBtn.text != "Tracker" {
		t.Errorf("Expected tabTrackerBtn to remain 'Tracker', got %q", appView.tabTrackerBtn.text)
	}

	// 4. Toggle TelkoMode OFF
	appView.ToggleTelkoMode()
	if appView.IsShieldActive() {
		t.Error("Expected IsShieldActive to be false after toggling off")
	}
	if !strings.HasPrefix(appView.tabTrackerBtn.text, "00:") {
		t.Errorf("Expected tabTrackerBtn to resume showing duration, got %q", appView.tabTrackerBtn.text)
	}
}

