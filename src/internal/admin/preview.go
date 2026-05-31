package admin

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"time"

	"streamdeck-go/internal/app"
	"streamdeck-go/internal/deck"
	"streamdeck-go/internal/render"
)

type PreviewRequest struct {
	Config FileConfig `json:"config"`
	PageID string     `json:"page_id"`
}

type PreviewResponse struct {
	Keys    []string `json:"keys"`
	Columns int      `json:"columns"`
	Rows    int      `json:"rows"`
}

func renderPreview(req PreviewRequest) (PreviewResponse, error) {
	page, ok := req.Config.Pages.Items[req.PageID]
	if !ok {
		page = app.Page{}
	}
	layout := previewLayout(req.Config.Settings.Model)
	renderer := render.NewWithLayout(req.Config.Settings.IconDir, layout, req.Config.Settings.Font.Path)
	keys, err := renderer.ComposePage(renderPage(page), placeholderAlbum(), 0, "Paused", render.Weather{
		Condition:   "clear",
		Temperature: 22,
		OK:          true,
	}, time.Now())
	if err != nil {
		return PreviewResponse{}, err
	}
	encoded := make([]string, len(keys))
	for i, key := range keys {
		var buf bytes.Buffer
		if err := png.Encode(&buf, key); err != nil {
			return PreviewResponse{}, err
		}
		encoded[i] = "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
	}
	return PreviewResponse{Keys: encoded, Columns: layout.Columns, Rows: layout.Rows}, nil
}

func previewLayout(modelID string) render.Layout {
	model, ok := deck.ModelByID(modelID)
	if !ok {
		model, _ = deck.ModelByID("mini")
	}
	return model.Layout()
}

func renderPage(page app.Page) render.Page {
	buttons := make([]render.Button, len(page.Buttons))
	for i, button := range page.Buttons {
		layers := make([]render.Layer, len(button.Layers))
		copy(layers, button.Layers)
		buttons[i] = render.Button{
			Layers: layers,
		}
	}
	return render.Page{
		Background: page.Background,
		Buttons:    buttons,
	}
}

func placeholderAlbum() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 240, 240))
	for y := 0; y < 240; y++ {
		for x := 0; x < 240; x++ {
			img.SetRGBA(x, y, color.RGBA{
				R: uint8(20 + x/3),
				G: uint8(30 + y/4),
				B: uint8(80 + (x+y)/8),
				A: 255,
			})
		}
	}
	return img
}
