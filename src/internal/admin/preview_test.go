package admin

import (
	"strings"
	"testing"

	"streamdeck-go/internal/app"
	"streamdeck-go/internal/render"
)

func TestRenderPreviewUsesConfiguredModelLayout(t *testing.T) {
	resp, err := renderPreview(PreviewRequest{
		Config: FileConfig{
			Settings: Settings{
				Model:   "classic",
				IconDir: t.TempDir(),
			},
			Pages: Pages{
				Items: map[string]app.Page{
					"main": {
						Background: render.Background{Type: "solid", Color: "#111827"},
					},
				},
			},
		},
		PageID: "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Columns != 5 || resp.Rows != 3 {
		t.Fatalf("layout = %dx%d, want 5x3", resp.Columns, resp.Rows)
	}
	if len(resp.Keys) != 15 {
		t.Fatalf("keys = %d, want 15", len(resp.Keys))
	}
	if !strings.HasPrefix(resp.Keys[0], "data:image/png;base64,") {
		t.Fatalf("preview key does not look like a data URL: %q", resp.Keys[0])
	}
}
