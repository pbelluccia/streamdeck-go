package app

import (
	"context"
	"testing"
	"time"

	"streamdeck-go/internal/render"
)

func TestHoldButtonReleaseBeforeThresholdRunsPress(t *testing.T) {
	app := testInputApp(50)

	app.HandleKeyEvent(context.Background(), 0, true)
	app.HandleKeyEvent(context.Background(), 0, false)

	if got := testCurrentPage(app); got != "press" {
		t.Fatalf("currentPage = %q, want press", got)
	}
}

func TestHoldButtonAfterThresholdRunsHoldOnly(t *testing.T) {
	app := testInputApp(10)

	app.HandleKeyEvent(context.Background(), 0, true)
	time.Sleep(30 * time.Millisecond)
	app.HandleKeyEvent(context.Background(), 0, false)

	if got := testCurrentPage(app); got != "hold" {
		t.Fatalf("currentPage = %q, want hold", got)
	}
}

func TestPageTimeoutReturnsToStartPage(t *testing.T) {
	app := testInputApp(50)
	app.setCurrentPage("temporary")

	app.resetPageTimeout(context.Background())
	time.Sleep(1100 * time.Millisecond)

	if got := testCurrentPage(app); got != "main" {
		t.Fatalf("currentPage = %q, want main", got)
	}
}

func TestPageTimeoutDoesNotRunOnStartPage(t *testing.T) {
	app := testInputApp(50)

	app.resetPageTimeout(context.Background())
	time.Sleep(1100 * time.Millisecond)

	if got := testCurrentPage(app); got != "main" {
		t.Fatalf("currentPage = %q, want main", got)
	}
}

func TestPageTimeoutResetsOnSecondaryPageActivity(t *testing.T) {
	app := testInputApp(50)
	app.setCurrentPage("temporary")

	app.resetPageTimeout(context.Background())
	time.Sleep(500 * time.Millisecond)
	app.handleActionAndRefresh(context.Background(), Action{Type: "empty"})
	time.Sleep(700 * time.Millisecond)
	if got := testCurrentPage(app); got != "temporary" {
		t.Fatalf("currentPage = %q, want temporary before reset timeout expires", got)
	}
	time.Sleep(500 * time.Millisecond)
	if got := testCurrentPage(app); got != "main" {
		t.Fatalf("currentPage = %q, want main after reset timeout expires", got)
	}
}

func TestBrightnessActionUpDownSetAndClamp(t *testing.T) {
	app := testInputApp(50)
	app.brightness = 20

	app.handleAction(context.Background(), Action{Type: "brightness", Command: "up", Step: 15})
	if got := app.currentBrightness(); got != 35 {
		t.Fatalf("brightness after up = %d, want 35", got)
	}

	app.handleAction(context.Background(), Action{Type: "brightness", Command: "down", Step: 100})
	if got := app.currentBrightness(); got != 0 {
		t.Fatalf("brightness after down clamp = %d, want 0", got)
	}

	value := 140
	app.handleAction(context.Background(), Action{Type: "brightness", Command: "set", Value: &value})
	if got := app.currentBrightness(); got != 100 {
		t.Fatalf("brightness after set clamp = %d, want 100", got)
	}
}

func TestBrightnessActionWhileDisplayOffOnlyUpdatesStoredBrightness(t *testing.T) {
	app := testInputApp(50)
	app.brightness = 20
	app.mode = 2
	value := 70

	app.handleAction(context.Background(), Action{Type: "brightness", Command: "set", Value: &value})

	if got := app.currentBrightness(); got != 70 {
		t.Fatalf("stored brightness = %d, want 70", got)
	}
	if got := app.displayMode(); got != 2 {
		t.Fatalf("display mode = %d, want 2", got)
	}
}

func TestStopTimersStopsPageAndHoldTimers(t *testing.T) {
	app := testInputApp(50)
	app.pageTimer = time.NewTimer(time.Hour)
	app.keyStates[0].timer = time.NewTimer(time.Hour)
	app.keyStates[1].timer = time.NewTimer(time.Hour)

	app.stopTimers()

	if app.pageTimer != nil {
		t.Fatal("page timer was not cleared")
	}
	for key, state := range app.keyStates {
		if state.timer != nil {
			t.Fatalf("key %d timer was not cleared", key)
		}
	}
}

func testCurrentPage(app *App) string {
	pageID, _ := app.currentPageSnapshot()
	return pageID
}

func testInputApp(holdMS int) *App {
	pages := map[string]Page{
		"main": {
			Buttons: []Button{
				{
					Press: Action{Type: "page", Page: "press"},
					Hold:  Action{Type: "page", Page: "hold"},
				},
			},
		},
		"press": {},
		"hold":  {},
		"temporary": {
			TimeoutSeconds: 1,
		},
	}
	app := &App{
		config:      Config{HoldMS: holdMS, StartPage: "main", Pages: pages},
		layout:      render.DefaultLayout(),
		currentPage: "main",
	}
	app.ensureKeyBuffers()
	return app
}
