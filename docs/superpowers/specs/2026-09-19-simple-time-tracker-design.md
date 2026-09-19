# Design Specification: Simple Time Tracker (stt)

**Date**: 2026-09-19  
**Status**: Approved  
**Author**: Frank Hess & Antigravity  
**Application Name**: Simple Time Tracker (`stt`)  
**Bundle ID**: `com.avono.simple-time-tracker`  

---

## 1. Overview & Goals

Simple Time Tracker (`stt`) is a lightweight, keyboard-friendly macOS and cross-platform desktop time tracker built with Go and `gogpu/ui`. This specification covers:
1. Resolving input handling failures where text fields and project selectors in the HUD and modals were not receiving focus or keystrokes.
2. Enabling intuitive calendar manual editing and click-to-create workflows for time bookings.
3. Designing an official macOS app icon and generating `AppIcon.icns`.
4. Establishing a multi-platform build, DMG packaging, and Homebrew tap distribution workflow modeled after `jira-quick-access`.

---

## 2. Architecture & Input System Fixes

### 2.1 Problem Analysis
* **Event Dispatch Omission**: `HUDView.Event()` intercepted button clicks and a fallback switch appending runes to `inputBookingText`. It never forwarded `MouseEvent`, `KeyEvent`, or `FocusEvent` to child controls (`h.taskInput`, `h.notesInput`, and `h.projectInput`).
* **Focus Lifecycle**: `TextInput` lacked `SetFocused(bool)` and `IsFocused() bool` methods, meaning `gogpu/ui`'s `ContextImpl.RequestFocus()` could not blur previously focused widgets. Clicking between fields left all clicked inputs marked as focused, so the first one in the list swallowed all subsequent key events.
* **Widget Tree Traversal**: `AppView.Children()` and `HUDView.Children()` returned `nil`, isolating inputs from the framework's focus traversal and focus manager.

### 2.2 Technical Solution
* **`TextInput` Enhancements**:
  * Implement `SetFocused(focused bool)`: sets `isFocused` state and clears selection on blur (`focused == false`).
  * Implement `IsFocused() bool`: returns current focus status.
  * In `handleMouseEvent`: on `MousePress` within bounds, call `ctx.RequestFocus(t)`. On `MousePress` outside bounds, blur.
  * Implement Tab / Shift+Tab navigation to cycle between adjacent inputs.
* **`HUDView` Event Routing**:
  * Forward events sequentially:
    ```go
    if h.projectInput.Event(ctx, e) {
        return true
    }
    if h.taskInput.Event(ctx, e) {
        return true
    }
    if h.notesInput.Event(ctx, e) {
        return true
    }
    ```
  * In `refreshState()`, only sync text from the active timer entry if the input is not currently focused by the user (`!h.taskInput.IsFocused()`).
* **Tree Hierarchy**:
  * Implement `Children()` in `AppView` returning active tab view and header buttons.
  * Implement `Children()` in `HUDView` returning all buttons, picker, and text inputs.

---

## 3. Calendar View & Manual Bookings Workflow

### 3.1 Timeline Click-to-Create
* **Hour Slot Detection**: In timeline mode (07:00 to 21:00), calculate clicked hour based on mouse Y coordinate:
  $$\text{Hour} = \text{dayStartHour} + \lfloor \frac{Y - \text{contentTop}}{\text{hourHeight}} \rfloor$$
* **Quick Modal Initialization**: Clicking an empty slot automatically opens `EntryEditor` initialized on `currentDay` with:
  * `StartedAt`: `HH:00`
  * `EndedAt`: `(HH+1):00`
  * Default project and empty task description.

### 3.2 Existing Entry Editing
* Clicking an existing entry block opens `EntryEditor` populated with the entry's task name, booking text, start/end timestamps, project ID, and billability toggle.
* Supports saving updates or deleting with a 2-step confirmation.

### 3.3 `EntryEditor` UX Enhancements
* **Quick Duration Extenders**: Add quick buttons:
  * `+15m`, `+30m`, `+1h` to increment `endDTInput` by 15, 30, or 60 minutes with one click.
* **Date & Time Formats**: Accept flexible time formats (`15:04`, `15:04:05`, `02.01.2006 15:04`).
* **Focus Traversal**: Tab moves from Task $\rightarrow$ Notes $\rightarrow$ Startzeit $\rightarrow$ Endzeit $\rightarrow$ Projekt.
* **Immediate Feedback**: After saving/deleting, refresh `CalendarView` entries, recalculate totals, and trigger immediate redraw.

---

## 4. App Icon & Asset Generation

### 4.1 Icon Generator (`assets/generate_icon.py`)
* Python script using Pillow with supersampling (2048×2048 rendered, downscaled to 1024×1024 master `icon.png`).
* **Visual Elements**:
  * Apple macOS Human Interface Guidelines (HIG) squircle tile.
  * Deep obsidian and slate gradient background.
  * Luminous emerald (`#10B981`) and electric cyan (`#06B6D4`) stopwatch dial with glowing circular progress ring and fine tick marks.
  * Minimalist glass card accents.
* **Output Artifacts**:
  * `assets/icon.png` (1024×1024 master)
  * `assets/icon.ico` (multi-resolution Windows icon)
  * `assets/AppIcon.icns` (macOS bundle icon generated via `iconutil -c icns`)

---

## 5. Build, Packaging & Homebrew Tap

### 5.1 Build System (`build.sh`)
* **Identifiers**:
  * `APP_NAME="stt"`
  * `APP_DISPLAY_NAME="Simple Time Tracker"`
  * `BUNDLE_ID="com.avono.simple-time-tracker"`
  * `VERSION="1.0.0"`
* **Targets**:
  * `osx`: Builds ARM64 and AMD64, stitches into Universal 2 binary with `lipo`, copies `AppIcon.icns`, writes `Info.plist`, and builds `.dmg` with `/Applications` symlink.
  * `install`: Installs directly into `/Applications/Simple Time Tracker.app` and removes macOS quarantine attribute.
  * `all`: Builds macOS Universal, Windows AMD64/ARM64, and Linux AMD64/ARM64.
  * `clean`: Removes `dist/`.

### 5.2 Homebrew Release (`./build.sh release [version]`)
* Computes SHA-256 of the generated DMG.
* Tags git commit (e.g. `v1.0.0`).
* Creates GitHub release on `diver80/simple-time-tracker` with the DMG attached using `gh release create`.
* Clones `diver80/homebrew-tap`, writes/updates `Casks/simple-time-tracker.rb`, and pushes to main.

#### Cask Formula (`Casks/simple-time-tracker.rb`)
```ruby
cask "simple-time-tracker" do
  version "1.0.0"
  sha256 "<checksum>"

  url "https://github.com/diver80/simple-time-tracker/releases/download/v#{version}/Simple.Time.Tracker-v#{version}-macOS-Universal.dmg"
  name "Simple Time Tracker"
  desc "Lightweight, keyboard-friendly desktop time tracker"
  homepage "https://github.com/diver80/simple-time-tracker"

  app "Simple Time Tracker.app"
  binary "#{appdir}/Simple Time Tracker.app/Contents/MacOS/stt", target: "stt"

  zap trash: [
    "~/Library/Application Support/time-tracker",
    "~/Library/Application Support/simple-time-tracker",
  ]
end
```

---

## 6. Testing & Verification

1. **Unit Tests**:
   * Text input focus, selection, typing, and blur behavior in `pkg/ui/text_input_test.go`.
   * Timeline click-to-hour math and editor opening in `pkg/ui/calendar_edit_test.go`.
   * Entry CRUD validation and quick duration buttons in `pkg/ui/entry_editor_test.go`.
2. **Integration Verification**:
   * Verify `./build.sh osx` generates valid Universal 2 binary and `.app` bundle with `AppIcon.icns`.
   * Verify all Go unit tests pass cleanly: `go test ./...`.
