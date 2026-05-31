package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInjectPageAddsPageAndDisplaysIt(t *testing.T) {
	app := testInputApp(50)
	defer stopTestPageTimer(app)
	page := Page{
		TimeoutSeconds: 3,
		Buttons: Buttons{
			{Press: Action{Type: "page", Page: "main"}},
		},
	}

	if err := app.InjectPage(context.Background(), "notice", page); err != nil {
		t.Fatal(err)
	}

	if got := testCurrentPage(app); got != "notice" {
		t.Fatalf("currentPage = %q, want notice", got)
	}
	if got := app.config.Pages["notice"].TimeoutSeconds; got != 3 {
		t.Fatalf("injected page timeout = %d, want 3", got)
	}
	if got := app.config.Pages["notice"].Buttons[0].Press.Page; got != "main" {
		t.Fatalf("injected page button target = %q, want main", got)
	}
}

func TestInjectedPageHTTPUsesPageJSONShape(t *testing.T) {
	app := testInputApp(50)
	defer stopTestPageTimer(app)
	body := strings.NewReader(`{
		"timeout_seconds": 5,
		"background": { "type": "solid", "color": "#111827" },
		"buttons": {
			"0": {
				"color": "#dc2626",
				"layers": [
					{ "type": "text", "text": "Alert", "font_size": 18 }
				],
				"press": { "type": "page", "page": "main" }
			}
		}
	}`)
	req := httptest.NewRequest(http.MethodPut, "/pages/notification", body)
	rec := httptest.NewRecorder()

	app.handleInjectedPage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := testCurrentPage(app); got != "notification" {
		t.Fatalf("currentPage = %q, want notification", got)
	}
	injected := app.config.Pages["notification"]
	if injected.Background.Color != "#111827" {
		t.Fatalf("background color = %q, want #111827", injected.Background.Color)
	}
	if got := injected.Buttons[0].Layers[0].Text; got != "Alert" {
		t.Fatalf("layer text = %q, want Alert", got)
	}
}

func TestInjectedPageHTTPRejectsButtonArray(t *testing.T) {
	app := testInputApp(50)
	req := httptest.NewRequest(http.MethodPut, "/pages/bad", strings.NewReader(`{
		"buttons": [
			{ "layers": [{ "type": "empty" }] }
		]
	}`))
	rec := httptest.NewRecorder()

	app.handleInjectedPage(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestInjectedPageHTTPClearReturnsToStartPage(t *testing.T) {
	app := testInputApp(50)
	defer stopTestPageTimer(app)
	if err := app.InjectPage(context.Background(), "notification", Page{
		Buttons: Buttons{
			{Press: Action{Type: "page", Page: "main"}},
		},
	}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/pages/notification/clear", nil)
	rec := httptest.NewRecorder()

	app.handleInjectedPage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := testCurrentPage(app); got != "main" {
		t.Fatalf("currentPage = %q, want main", got)
	}
	if _, ok := app.config.Pages["notification"]; ok {
		t.Fatal("notification page was not cleared")
	}
}

func stopTestPageTimer(app *App) {
	app.pageTimerMu.Lock()
	defer app.pageTimerMu.Unlock()
	if app.pageTimer != nil {
		app.pageTimer.Stop()
		app.pageTimer = nil
	}
}
