package ui

import (
	"testing"

	"time-tracker/pkg/db"

	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
)

func TestProjectViewDeleteCard(t *testing.T) {
	repo, err := db.NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	cust, err := repo.CreateCustomer("Acme Corp")
	if err != nil {
		t.Fatal(err)
	}
	proj, err := repo.CreateProject(cust.ID, "Test", 100, 10, 1000, "#3B82F6")
	if err != nil {
		t.Fatal(err)
	}

	pv := NewProjectView(repo, nil)
	pv.SetBounds(geometry.NewRect(0, 0, 420, 560))

	mockCtx := &testWidgetContext{}
	pv.Draw(mockCtx, &testCanvas{})

	// Trigger delete for the project
	pv.requestDeleteProject(proj.ID)
	if pv.confirmDeleteID != proj.ID {
		t.Fatalf("expected confirmDeleteID to be %d, got %d", proj.ID, pv.confirmDeleteID)
	}

	pv.confirmDelete()
	pv.Refresh()

	projs, err := repo.ListProjects(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(projs) != 0 {
		t.Fatalf("expected project to be deleted from repo, found: %d", len(projs))
	}
}

func TestProjectViewCancelDelete(t *testing.T) {
	repo, err := db.NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	cust, _ := repo.CreateCustomer("Test Customer")
	proj, _ := repo.CreateProject(cust.ID, "Cancel Me", 100, 10, 1000, "#3B82F6")

	pv := NewProjectView(repo, nil)
	pv.SetBounds(geometry.NewRect(0, 0, 420, 560))

	mockCtx := &testWidgetContext{}
	pv.Draw(mockCtx, &testCanvas{})

	pv.requestDeleteProject(proj.ID)
	if pv.confirmDeleteID != proj.ID {
		t.Fatalf("expected confirmDeleteID to be %d, got %d", proj.ID, pv.confirmDeleteID)
	}

	pv.cancelDelete()
	if pv.confirmDeleteID != 0 {
		t.Fatalf("expected confirmDeleteID to be reset to 0, got %d", pv.confirmDeleteID)
	}

	projs, _ := repo.ListProjects(nil)
	if len(projs) != 1 {
		t.Fatalf("expected project to remain in repo, found %d", len(projs))
	}
}

func TestProjectViewClearDemoData(t *testing.T) {
	repo, err := db.NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	cust, _ := repo.CreateCustomer("Acme Corporation")
	repo.CreateProject(cust.ID, "Demo Project", 120, 20, 2400, "#3B82F6")

	pv := NewProjectView(repo, nil)
	pv.SetBounds(geometry.NewRect(0, 0, 420, 560))

	if !pv.hasDemoData() {
		t.Fatalf("expected hasDemoData() to be true for Acme Corporation")
	}

	pv.handleClearDemo()
	pv.Refresh()

	if pv.hasDemoData() {
		t.Fatalf("expected hasDemoData() to be false after clearing demo data")
	}

	custs, _ := repo.ListCustomers()
	if len(custs) != 0 {
		t.Fatalf("expected 0 customers remaining, got %d", len(custs))
	}
}

func TestProjectEditorSaveAndValidate(t *testing.T) {
	repo, err := db.NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	savedCalled := false
	canceledCalled := false

	editor := NewProjectEditor(repo, func() {
		savedCalled = true
	}, func() {
		canceledCalled = true
	})

	// Validation check: empty project name
	editor.projectInput.SetText("")
	editor.handleSave()
	if savedCalled {
		t.Fatalf("handleSave should not succeed with empty project name")
	}
	if editor.errorMessage == "" {
		t.Fatalf("expected error message for empty project name")
	}

	// Successful save
	editor.customerInput.SetText("New Client Inc")
	editor.projectInput.SetText("Mobile App")
	editor.rateInput.SetText("150")
	editor.hoursInput.SetText("50")

	editor.handleSave()
	if !savedCalled {
		t.Fatalf("expected saved callback to be called")
	}

	projs, _ := repo.ListProjects(nil)
	if len(projs) != 1 {
		t.Fatalf("expected 1 project in repo, got %d", len(projs))
	}
	if projs[0].Name != "Mobile App" {
		t.Fatalf("expected project name 'Mobile App', got '%s'", projs[0].Name)
	}
	if projs[0].HourlyRate != 150.0 {
		t.Fatalf("expected rate 150.0, got %f", projs[0].HourlyRate)
	}
	if projs[0].BudgetHours != 50.0 {
		t.Fatalf("expected budget hours 50.0, got %f", projs[0].BudgetHours)
	}
	if projs[0].BudgetCost != 7500.0 {
		t.Fatalf("expected budget cost 7500.0, got %f", projs[0].BudgetCost)
	}

	// Test cancel
	editor.handleCancel()
	if !canceledCalled {
		t.Fatalf("expected canceled callback to be called")
	}
}

func TestProjectViewOpenEditorModal(t *testing.T) {
	repo, err := db.NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	redrawCalled := false
	pv := NewProjectView(repo, func() {
		redrawCalled = true
	})
	pv.SetBounds(geometry.NewRect(0, 0, 420, 560))

	if pv.projectEditor != nil {
		t.Fatalf("expected projectEditor to be nil initially")
	}

	pv.openProjectEditor()
	if pv.projectEditor == nil {
		t.Fatalf("expected projectEditor to be open")
	}
	if !redrawCalled {
		t.Fatalf("expected redraw to be requested")
	}

	// Draw while editor is open
	mockCtx := &testWidgetContext{}
	pv.Draw(mockCtx, &testCanvas{})

	// Verify redraw is requested on validation error (wires SetOnRequestRedraw)
	redrawCalled = false
	pv.projectEditor.projectInput.SetText("")
	pv.projectEditor.handleSave()
	if !redrawCalled {
		t.Fatalf("expected redraw to be requested on validation error in editor")
	}

	// Cancel editor
	pv.projectEditor.handleCancel()
	if pv.projectEditor != nil {
		t.Fatalf("expected projectEditor to be nil after cancel")
	}
}

func TestProjectViewDeleteCardInteractivity(t *testing.T) {
	repo, err := db.NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	cust, _ := repo.CreateCustomer("Acme Corp")
	proj, _ := repo.CreateProject(cust.ID, "UI Interactivity Test", 100, 10, 1000, "#3B82F6")

	pv := NewProjectView(repo, nil)
	pv.SetBounds(geometry.NewRect(0, 0, 420, 560))

	mockCtx := &testWidgetContext{}
	pv.Draw(mockCtx, &testCanvas{})

	delBounds, ok := pv.deleteBtnBounds[proj.ID]
	if !ok {
		t.Fatalf("expected delete button bounds for project %d", proj.ID)
	}

	// Click delete button
	clickPt := geometry.Pt(delBounds.Min.X+2, delBounds.Min.Y+2)
	pressEv := event.NewMouseEvent(event.MousePress, event.ButtonLeft, 0, clickPt, clickPt, event.ModNone)
	handled := pv.Event(mockCtx, pressEv)
	if !handled {
		t.Fatalf("expected delete button click to be handled")
	}
	if pv.confirmDeleteID != proj.ID {
		t.Fatalf("expected confirmDeleteID to be %d, got %d", proj.ID, pv.confirmDeleteID)
	}

	// Draw confirmation state
	pv.Draw(mockCtx, &testCanvas{})

	// Click Ja button
	yesBounds := pv.confirmYesBtn.Bounds()
	clickYes := geometry.Pt(yesBounds.Min.X+5, yesBounds.Min.Y+5)
	yesEv := event.NewMouseEvent(event.MousePress, event.ButtonLeft, 0, clickYes, clickYes, event.ModNone)
	handledYes := pv.Event(mockCtx, yesEv)
	if !handledYes {
		t.Fatalf("expected Yes button click to be handled")
	}

	projs, _ := repo.ListProjects(nil)
	if len(projs) != 0 {
		t.Fatalf("expected project to be deleted, found %d", len(projs))
	}
}

func TestProjectEditorValidationAndDefaults(t *testing.T) {
	repo, err := db.NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	editor := NewProjectEditor(repo, nil, nil)

	// Auto-focus on customer input
	if !editor.customerInput.IsFocused() {
		t.Fatalf("expected customerInput to be auto-focused on modal creation")
	}

	// Invalid rate should fail and NOT create a customer in DB
	editor.customerInput.SetText("Orphan Check Customer")
	editor.projectInput.SetText("Test Proj")
	editor.rateInput.SetText("not-a-number")
	editor.handleSave()
	if editor.errorMessage == "" {
		t.Fatalf("expected error for non-numeric rate")
	}

	custsAfterFail, _ := repo.ListCustomers()
	if len(custsAfterFail) != 0 {
		t.Fatalf("expected no customer created when rate validation fails, found %d", len(custsAfterFail))
	}

	// Negative rate
	editor.errorMessage = ""
	editor.rateInput.SetText("-50")
	editor.handleSave()
	if editor.errorMessage == "" {
		t.Fatalf("expected error for negative rate")
	}

	// Invalid hours should not create customer either
	editor.errorMessage = ""
	editor.rateInput.SetText("100")
	editor.hoursInput.SetText("invalid")
	editor.handleSave()
	if editor.errorMessage == "" {
		t.Fatalf("expected error for non-numeric hours")
	}
	custsAfterHoursFail, _ := repo.ListCustomers()
	if len(custsAfterHoursFail) != 0 {
		t.Fatalf("expected no customer created when hours validation fails, found %d", len(custsAfterHoursFail))
	}

	// Comma decimal support (German formatting: 125,50 €/h and 37,5 h)
	editor.errorMessage = ""
	editor.customerInput.SetText("Acme Comma Corp")
	editor.projectInput.SetText("Comma Project")
	editor.rateInput.SetText("125,50")
	editor.hoursInput.SetText("40,0")

	saved := false
	editor.onSaved = func() { saved = true }
	editor.handleSave()

	if !saved {
		t.Fatalf("expected save to succeed with comma decimals, got error: %s", editor.errorMessage)
	}

	projs, _ := repo.ListProjects(nil)
	if len(projs) != 1 || projs[0].HourlyRate != 125.5 || projs[0].BudgetHours != 40.0 {
		t.Fatalf("expected project rate 125.5 and hours 40.0, got rate=%f, hours=%f", projs[0].HourlyRate, projs[0].BudgetHours)
	}

	// Default customer name to "Standard" when blank
	editor.errorMessage = ""
	editor.customerInput.SetText("")
	editor.projectInput.SetText("Internal Project")
	editor.rateInput.SetText("0")
	editor.hoursInput.SetText("0")

	saved = false
	editor.handleSave()

	if !saved {
		t.Fatalf("expected save to succeed with empty customer, got error: %s", editor.errorMessage)
	}

	custs, _ := repo.ListCustomers()
	hasStandard := false
	for _, c := range custs {
		if c.Name == "Standard" {
			hasStandard = true
		}
	}
	if !hasStandard {
		t.Fatalf("expected customer 'Standard', got %v", custs)
	}
}

func TestProjectEditorTabCycling(t *testing.T) {
	repo, err := db.NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	mockCtx := &testWidgetContext{}
	editor := NewProjectEditor(repo, nil, nil)

	// customerInput is auto-focused initially
	if !editor.customerInput.IsFocused() {
		t.Fatalf("expected customerInput to be auto-focused initially")
	}

	tabEv := event.NewKeyEvent(event.KeyPress, event.KeyTab, 0, event.ModNone)

	// First tab should move to projectInput
	handled := editor.Event(mockCtx, tabEv)
	if !handled || !editor.projectInput.IsFocused() {
		t.Fatalf("expected first tab to focus projectInput, handled=%v, focused=%v", handled, editor.projectInput.IsFocused())
	}

	// Next tab should focus rateInput
	editor.Event(mockCtx, tabEv)
	if !editor.rateInput.IsFocused() {
		t.Fatalf("expected second tab to focus rateInput")
	}

	// Next tab should focus hoursInput
	editor.Event(mockCtx, tabEv)
	if !editor.hoursInput.IsFocused() {
		t.Fatalf("expected third tab to focus hoursInput")
	}

	// Next tab should cycle back to customerInput
	editor.Event(mockCtx, tabEv)
	if !editor.customerInput.IsFocused() {
		t.Fatalf("expected tab to cycle back to customerInput")
	}
}

func TestProjectViewChildren(t *testing.T) {
	repo, err := db.NewRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	pv := NewProjectView(repo, nil)

	// Empty state children: addProjectBtn, addSampleBtn
	kids := pv.Children()
	if len(kids) < 2 {
		t.Fatalf("expected at least 2 children in empty state, got %d", len(kids))
	}

	// Confirmation mode children: addProjectBtn, confirmYesBtn, confirmNoBtn
	pv.confirmDeleteID = 42
	kidsConf := pv.Children()
	hasYes := false
	for _, k := range kidsConf {
		if k == pv.confirmYesBtn {
			hasYes = true
		}
	}
	if !hasYes {
		t.Fatalf("expected confirmYesBtn in Children during delete confirmation")
	}

	// Editor mode: projectEditor is child
	pv.confirmDeleteID = 0
	pv.openProjectEditor()
	kidsEditor := pv.Children()
	if len(kidsEditor) != 1 || kidsEditor[0] != pv.projectEditor {
		t.Fatalf("expected only projectEditor in Children when editor open")
	}
}

