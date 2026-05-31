package render

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/color/palette"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestComposePageAndBMPSize(t *testing.T) {
	album := image.NewRGBA(image.Rect(0, 0, 200, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 200; x++ {
			album.SetRGBA(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 80, A: 255})
		}
	}

	r := New("../../../icons")
	keys, err := r.ComposePage(testPage(), album, 1, "Playing", Weather{}, time.Date(2026, 5, 21, 20, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 6 {
		t.Fatalf("got %d keys, want 6", len(keys))
	}
	for i, key := range keys {
		if key.Bounds().Dx() != KeyWidth || key.Bounds().Dy() != KeyHeight {
			t.Fatalf("key %d size = %v, want 80x80", i, key.Bounds())
		}
		bmp := r.KeyToBMP(key)
		if len(bmp) != 54+KeyWidth*KeyHeight*3 {
			t.Fatalf("key %d BMP size = %d", i, len(bmp))
		}
		if string(bmp[:2]) != "BM" {
			t.Fatalf("key %d BMP header = %q", i, string(bmp[:2]))
		}
	}
}

func TestComposePageWithClassicLayoutAndJPEG(t *testing.T) {
	r := NewWithLayout("../../../icons", Layout{Columns: 5, Rows: 3, KeyWidth: 72, KeyHeight: 72})

	keys, err := r.ComposePage(testPage(), nil, 0, "Paused", Weather{}, time.Date(2026, 5, 21, 20, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 15 {
		t.Fatalf("got %d keys, want 15", len(keys))
	}
	for i, key := range keys {
		if key.Bounds().Dx() != 72 || key.Bounds().Dy() != 72 {
			t.Fatalf("key %d size = %v, want 72x72", i, key.Bounds())
		}
	}

	data, err := r.KeyToJPEG(keys[0])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := jpeg.Decode(bytes.NewReader(data)); err != nil {
		t.Fatalf("decode JPEG: %v", err)
	}
}

func TestComposePageWithoutAlbumArtUsesBlackBackground(t *testing.T) {
	r := New("../../../icons")
	keys, err := r.ComposePage(testPage(), nil, 1, "Paused", Weather{}, time.Date(2026, 5, 21, 20, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatal(err)
	}

	for keyIndex, key := range keys {
		for y := 0; y < KeyHeight; y++ {
			for x := 0; x < KeyWidth; x++ {
				if got := key.RGBAAt(x, y); got != (color.RGBA{0, 0, 0, 255}) {
					t.Fatalf("key %d pixel %d,%d = %v, want black", keyIndex, x, y, got)
				}
			}
		}
	}
}

func TestComposePageUsesColorLayerWithoutAlbumArt(t *testing.T) {
	r := New("../../../icons")
	page := Page{
		Background: Background{Type: "media_art", Mode: "fill"},
		Buttons: []Button{
			{Layers: []Layer{{Type: "color", Color: "#123456"}}},
			{Layers: []Layer{{Type: "color", Color: "ABCDEF"}}},
		},
	}

	keys, err := r.ComposePage(page, nil, 0, "Paused", Weather{}, time.Date(2026, 5, 21, 20, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatal(err)
	}

	if got, want := keys[0].RGBAAt(0, 0), (color.RGBA{0x12, 0x34, 0x56, 0xff}); got != want {
		t.Fatalf("key 0 background = %v, want %v", got, want)
	}
	if got, want := keys[1].RGBAAt(0, 0), (color.RGBA{0xab, 0xcd, 0xef, 0xff}); got != want {
		t.Fatalf("key 1 background = %v, want %v", got, want)
	}
}

func TestComposePageUsesPageBackgroundColor(t *testing.T) {
	r := New("../../../icons")
	page := Page{
		Background: Background{Type: "solid", Color: "#224466"},
	}

	keys, err := r.ComposePage(page, nil, 0, "Paused", Weather{}, r.started)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := keys[0].RGBAAt(0, 0), (color.RGBA{0x22, 0x44, 0x66, 0xff}); got != want {
		t.Fatalf("page background = %v, want %v", got, want)
	}
}

func TestPageBackgroundBlinkEffect(t *testing.T) {
	r := New("../../../icons")
	page := Page{
		Background: Background{
			Type:  "solid",
			Color: "#000000",
			Effect: Effect{
				Type:    "blink",
				Color:   "#ffffff",
				BlinkMS: 100,
			},
		},
	}

	onKeys, err := r.ComposePage(page, nil, 0, "Paused", Weather{}, r.started)
	if err != nil {
		t.Fatal(err)
	}
	offKeys, err := r.ComposePage(page, nil, 0, "Paused", Weather{}, r.started.Add(100*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	if got := onKeys[0].RGBAAt(0, 0); got != (color.RGBA{255, 255, 255, 255}) {
		t.Fatalf("blink on color = %v, want white", got)
	}
	if got := offKeys[0].RGBAAt(0, 0); got != (color.RGBA{0, 0, 0, 255}) {
		t.Fatalf("blink off color = %v, want black", got)
	}
}

func TestColorLayerBlinkStopsAfterRepeatDuration(t *testing.T) {
	r := New("../../../icons")
	page := Page{
		Background: Background{Type: "solid", Color: "#000000"},
		Buttons: []Button{
			{
				Layers: []Layer{
					{
						Type:  "color",
						Color: "#111111",
						Effect: Effect{
							Type:       "blink",
							Color:      "#eeeeee",
							BlinkMS:    100,
							DurationMS: 200,
							Repeat:     1,
						},
					},
				},
			},
		},
	}

	onKeys, err := r.ComposePage(page, nil, 0, "Paused", Weather{}, r.started)
	if err != nil {
		t.Fatal(err)
	}
	afterKeys, err := r.ComposePage(page, nil, 0, "Paused", Weather{}, r.started.Add(250*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	if got := onKeys[0].RGBAAt(0, 0); got != (color.RGBA{0xee, 0xee, 0xee, 0xff}) {
		t.Fatalf("button blink on color = %v, want effect color", got)
	}
	if got := afterKeys[0].RGBAAt(0, 0); got != (color.RGBA{0x11, 0x11, 0x11, 0xff}) {
		t.Fatalf("button blink after repeat = %v, want base color", got)
	}
}

func TestColorLayerPulseEffect(t *testing.T) {
	r := New("../../../icons")
	page := Page{
		Background: Background{Type: "solid", Color: "#000000"},
		Buttons: []Button{
			{
				Layers: []Layer{
					{
						Type:  "color",
						Color: "#000000",
						Effect: Effect{
							Type:       "pulse",
							Color:      "#646464",
							DurationMS: 1000,
						},
					},
				},
			},
		},
	}

	baseKeys, err := r.ComposePage(page, nil, 0, "Paused", Weather{}, r.started)
	if err != nil {
		t.Fatal(err)
	}
	peakKeys, err := r.ComposePage(page, nil, 0, "Paused", Weather{}, r.started.Add(500*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	if got := baseKeys[0].RGBAAt(0, 0); got != (color.RGBA{0, 0, 0, 255}) {
		t.Fatalf("pulse start = %v, want base color", got)
	}
	if got := peakKeys[0].RGBAAt(0, 0); got != (color.RGBA{100, 100, 100, 255}) {
		t.Fatalf("pulse peak = %v, want effect color", got)
	}
}

func TestColorLayerFlashEffect(t *testing.T) {
	r := New("../../../icons")
	page := Page{
		Background: Background{Type: "solid", Color: "#000000"},
		Buttons: []Button{
			{
				Layers: []Layer{
					{
						Type:  "color",
						Color: "#111111",
						Effect: Effect{
							Type:       "flash",
							Color:      "#eeeeee",
							DurationMS: 100,
							Repeat:     1,
						},
					},
				},
			},
		},
	}

	onKeys, err := r.ComposePage(page, nil, 0, "Paused", Weather{}, r.started.Add(50*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	offKeys, err := r.ComposePage(page, nil, 0, "Paused", Weather{}, r.started.Add(150*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	afterKeys, err := r.ComposePage(page, nil, 0, "Paused", Weather{}, r.started.Add(250*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	if got := onKeys[0].RGBAAt(0, 0); got != (color.RGBA{0xee, 0xee, 0xee, 0xff}) {
		t.Fatalf("flash on = %v, want effect color", got)
	}
	if got := offKeys[0].RGBAAt(0, 0); got != (color.RGBA{0x11, 0x11, 0x11, 0xff}) {
		t.Fatalf("flash off = %v, want base color", got)
	}
	if got := afterKeys[0].RGBAAt(0, 0); got != (color.RGBA{0x11, 0x11, 0x11, 0xff}) {
		t.Fatalf("flash after repeat = %v, want base color", got)
	}
}

func TestColorLayerUsesHexColorAndBlinkEffect(t *testing.T) {
	r := New("../../../icons")
	page := Page{
		Background: Background{Type: "solid", Color: "#000000"},
		Buttons: []Button{
			{
				Layers: []Layer{
					{
						Type:  "color",
						Color: "#111111",
						Effect: Effect{
							Type:    "blink",
							Color:   "#333333",
							BlinkMS: 100,
						},
					},
				},
			},
		},
	}

	onKeys, err := r.ComposePage(page, nil, 0, "Paused", Weather{}, r.started)
	if err != nil {
		t.Fatal(err)
	}
	offKeys, err := r.ComposePage(page, nil, 0, "Paused", Weather{}, r.started.Add(100*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	if got := onKeys[0].RGBAAt(0, 0); got != (color.RGBA{0x33, 0x33, 0x33, 0xff}) {
		t.Fatalf("color layer blink on = %v, want effect color", got)
	}
	if got := offKeys[0].RGBAAt(0, 0); got != (color.RGBA{0x11, 0x11, 0x11, 0xff}) {
		t.Fatalf("color layer blink off = %v, want base color", got)
	}
}

func TestComposePageUsesColorLayerOverAlbumArt(t *testing.T) {
	album := image.NewRGBA(image.Rect(0, 0, 200, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 200; x++ {
			album.SetRGBA(x, y, color.RGBA{R: 200, G: 10, B: 20, A: 255})
		}
	}

	r := New("../../../icons")
	page := Page{
		Background: Background{Type: "media_art", Mode: "fill"},
		Buttons: []Button{
			{Layers: []Layer{{Type: "color", Color: "#123456"}}},
		},
	}

	keys, err := r.ComposePage(page, album, 0, "Paused", Weather{}, time.Date(2026, 5, 21, 20, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatal(err)
	}

	if got, want := keys[0].RGBAAt(0, 0), (color.RGBA{0x12, 0x34, 0x56, 0xff}); got != want {
		t.Fatalf("key 0 background = %v, want button background", got)
	}
}

func TestMediaPlayPauseIconFollowsPlaybackState(t *testing.T) {
	iconDir := t.TempDir()
	writeTestIcon(t, iconDir, "play.png", color.RGBA{255, 0, 0, 255})
	writeTestIcon(t, iconDir, "pause.png", color.RGBA{0, 255, 0, 255})

	r := New(iconDir)
	page := Page{
		Background: Background{Type: "media_art", Mode: "fill"},
		Buttons: []Button{
			{Layers: []Layer{{Type: "empty"}}},
			{Layers: []Layer{{Type: "media_play_pause"}}},
		},
	}
	playingKeys, err := r.ComposePage(page, nil, 0, "Playing", Weather{}, time.Date(2026, 5, 21, 20, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatal(err)
	}
	pausedKeys, err := r.ComposePage(page, nil, 0, "Paused", Weather{}, time.Date(2026, 5, 21, 20, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatal(err)
	}

	playingCenter := playingKeys[1].RGBAAt(KeyWidth/2, KeyHeight/2)
	pausedCenter := pausedKeys[1].RGBAAt(KeyWidth/2, KeyHeight/2)
	if playingCenter == pausedCenter {
		t.Fatalf("play/pause key center did not change: %v", playingCenter)
	}
	if playingCenter.G <= playingCenter.R {
		t.Fatalf("playing state center = %v, want pause icon color", playingCenter)
	}
	if pausedCenter.R <= pausedCenter.G {
		t.Fatalf("paused state center = %v, want play icon color", pausedCenter)
	}
}

func TestImageViewRendersRelativePath(t *testing.T) {
	iconDir := t.TempDir()
	writeTestIcon(t, iconDir, "tile.png", color.RGBA{10, 80, 180, 255})

	r := New(iconDir)
	page := Page{
		Background: Background{Type: "solid"},
		Buttons: []Button{
			{Layers: []Layer{{Type: "image", Path: "tile.png", Mode: "stretch"}}},
		},
	}

	keys, err := r.ComposePage(page, nil, 0, "Paused", Weather{}, time.Date(2026, 5, 21, 20, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if got := keys[0].RGBAAt(KeyWidth/2, KeyHeight/2); got != (color.RGBA{10, 80, 180, 255}) {
		t.Fatalf("image center = %v, want image color", got)
	}
}

func TestImageViewFitKeepsAspectRatio(t *testing.T) {
	iconDir := t.TempDir()
	img := image.NewRGBA(image.Rect(0, 0, 20, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 20; x++ {
			img.SetRGBA(x, y, color.RGBA{200, 40, 30, 255})
		}
	}
	writePNG(t, filepath.Join(iconDir, "wide.png"), img)

	r := New(iconDir)
	page := Page{
		Background: Background{Type: "solid"},
		Buttons: []Button{
			{Layers: []Layer{{Type: "image", Path: "wide.png", Mode: "fit"}}},
		},
	}

	keys, err := r.ComposePage(page, nil, 0, "Paused", Weather{}, time.Date(2026, 5, 21, 20, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if got := keys[0].RGBAAt(KeyWidth/2, KeyHeight/2); got != (color.RGBA{200, 40, 30, 255}) {
		t.Fatalf("image center = %v, want image color", got)
	}
	if got := keys[0].RGBAAt(KeyWidth/2, 0); got != (color.RGBA{0, 0, 0, 255}) {
		t.Fatalf("letterbox pixel = %v, want black", got)
	}
}

func TestPageBackgroundImageSpansKeys(t *testing.T) {
	iconDir := t.TempDir()
	img := image.NewRGBA(image.Rect(0, 0, 252, 174))
	for y := 0; y < 174; y++ {
		for x := 0; x < 252; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 90, A: 255})
		}
	}
	writePNG(t, filepath.Join(iconDir, "background.png"), img)

	r := New(iconDir)
	page := Page{
		Background: Background{Type: "image", Path: "background.png", Mode: "stretch"},
		Buttons: []Button{
			{Layers: []Layer{{Type: "empty"}}},
			{Layers: []Layer{{Type: "empty"}}},
		},
	}

	keys, err := r.ComposePage(page, nil, 0, "Paused", Weather{}, time.Date(2026, 5, 21, 20, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if keys[0].RGBAAt(0, 0) == keys[1].RGBAAt(0, 0) {
		t.Fatal("page background did not span across key crops")
	}
}

func TestButtonImageOverridesPageBackgroundImage(t *testing.T) {
	iconDir := t.TempDir()
	writeTestIcon(t, iconDir, "background.png", color.RGBA{10, 20, 30, 255})
	writeTestIcon(t, iconDir, "button.png", color.RGBA{200, 80, 40, 255})

	r := New(iconDir)
	page := Page{
		Background: Background{Type: "image", Path: "background.png", Mode: "stretch"},
		Buttons: []Button{
			{Layers: []Layer{{Type: "image", Path: "button.png", Mode: "stretch"}}},
		},
	}

	keys, err := r.ComposePage(page, nil, 0, "Paused", Weather{}, time.Date(2026, 5, 21, 20, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if got := keys[0].RGBAAt(KeyWidth/2, KeyHeight/2); got != (color.RGBA{200, 80, 40, 255}) {
		t.Fatalf("button image pixel = %v, want button image", got)
	}
}

func TestButtonLayersComposeImageIconAndText(t *testing.T) {
	iconDir := t.TempDir()
	writeTestIcon(t, iconDir, "background.png", color.RGBA{20, 40, 60, 255})
	writeTestIcon(t, iconDir, "button.png", color.RGBA{220, 40, 30, 255})

	r := New(iconDir)
	page := Page{
		Background: Background{Type: "solid"},
		Buttons: []Button{
			{
				Layers: []Layer{
					{Type: "image", Path: "background.png", Mode: "stretch"},
					{Type: "icon", Icon: "button.png", Size: 24, Position: "center"},
					{Type: "text", Text: "M", FontSize: 14, Position: "lower"},
				},
			},
		},
	}

	keys, err := r.ComposePage(page, nil, 0, "Paused", Weather{}, time.Date(2026, 5, 21, 20, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if got := keys[0].RGBAAt(0, 0); got != (color.RGBA{20, 40, 60, 255}) {
		t.Fatalf("background pixel = %v, want image background", got)
	}
	if got := keys[0].RGBAAt(KeyWidth/2, KeyHeight/2); got.R < 150 {
		t.Fatalf("center pixel = %v, want icon layer", got)
	}
	if !hasNonBackgroundPixelInBand(keys[0], color.RGBA{20, 40, 60, 255}, 60, KeyHeight) {
		t.Fatal("lower text layer did not render")
	}
}

func TestTextLayerDefaultsToCenter(t *testing.T) {
	r := New("../../../icons")
	page := Page{
		Background: Background{Type: "solid"},
		Buttons: []Button{
			{
				Layers: []Layer{
					{Type: "text", Text: "M", FontSize: 18},
				},
			},
		},
	}

	keys, err := r.ComposePage(page, nil, 0, "Paused", Weather{}, time.Date(2026, 5, 21, 20, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	_, minY, _, maxY, ok := nonBlackBounds(keys[0])
	if !ok {
		t.Fatal("text layer rendered blank")
	}
	centerY := (minY + maxY) / 2
	if centerY < KeyHeight/2-6 || centerY > KeyHeight/2+6 {
		t.Fatalf("text vertical center = %d, want near %d (bounds y=%d..%d)", centerY, KeyHeight/2, minY, maxY)
	}
}

func TestTextLayerUsesConfiguredHexColors(t *testing.T) {
	r := New("../../../icons")
	page := Page{
		Background: Background{Type: "solid"},
		Buttons: []Button{
			{
				Layers: []Layer{
					{Type: "text", Text: "M", FontSize: 18, Color: "#00ff00", OutlineColor: "#ff0000", OutlineWidth: intPtr(2)},
				},
			},
		},
	}

	keys, err := r.ComposePage(page, nil, 0, "Paused", Weather{}, time.Date(2026, 5, 21, 20, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !hasPixel(keys[0], color.RGBA{0, 255, 0, 255}) {
		t.Fatal("text fill color #00ff00 was not rendered")
	}
	if !hasPixel(keys[0], color.RGBA{255, 0, 0, 255}) {
		t.Fatal("text outline color #ff0000 was not rendered")
	}
}

func TestTextLayerAllowsZeroOutlineWidth(t *testing.T) {
	r := New("../../../icons")
	page := Page{
		Background: Background{Type: "solid"},
		Buttons: []Button{
			{
				Layers: []Layer{
					{Type: "text", Text: "M", FontSize: 18, Color: "#00ff00", OutlineColor: "#ff0000", OutlineWidth: intPtr(0)},
				},
			},
		},
	}

	keys, err := r.ComposePage(page, nil, 0, "Paused", Weather{}, time.Date(2026, 5, 21, 20, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if hasPixel(keys[0], color.RGBA{255, 0, 0, 255}) {
		t.Fatal("text outline rendered even though outline_width was 0")
	}
}

func TestLayerJSONPreservesExplicitZeroOutlineWidth(t *testing.T) {
	data, err := json.Marshal(Layer{Type: "text", Text: "M", OutlineWidth: intPtr(0)})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`"outline_width":0`)) {
		t.Fatalf("encoded layer = %s, want outline_width:0", data)
	}
}

func TestIconLayerUsesConfiguredOutlineColor(t *testing.T) {
	iconDir := t.TempDir()
	writeCenteredIcon(t, filepath.Join(iconDir, "shape.png"), color.RGBA{0, 255, 0, 255})

	r := New(iconDir)
	page := Page{
		Background: Background{Type: "solid"},
		Buttons: []Button{
			{
				Layers: []Layer{
					{Type: "icon", Icon: "shape.png", Size: 24, OutlineColor: "#ff0000", OutlineWidth: intPtr(2)},
				},
			},
		},
	}

	keys, err := r.ComposePage(page, nil, 0, "Paused", Weather{}, time.Date(2026, 5, 21, 20, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !hasPixel(keys[0], color.RGBA{255, 0, 0, 255}) {
		t.Fatal("icon outline color #ff0000 was not rendered")
	}
}

func TestIconLayerAllowsZeroOutlineWidth(t *testing.T) {
	iconDir := t.TempDir()
	writeCenteredIcon(t, filepath.Join(iconDir, "shape.png"), color.RGBA{0, 255, 0, 255})

	r := New(iconDir)
	page := Page{
		Background: Background{Type: "solid"},
		Buttons: []Button{
			{
				Layers: []Layer{
					{Type: "icon", Icon: "shape.png", Size: 24, OutlineColor: "#ff0000", OutlineWidth: intPtr(0)},
				},
			},
		},
	}

	keys, err := r.ComposePage(page, nil, 0, "Paused", Weather{}, time.Date(2026, 5, 21, 20, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if hasPixel(keys[0], color.RGBA{255, 0, 0, 255}) {
		t.Fatal("icon outline rendered even though outline_width was 0")
	}
}

func TestTextLayerAppliesOffsets(t *testing.T) {
	r := New("../../../icons")
	page := Page{
		Background: Background{Type: "solid"},
		Buttons: []Button{
			{Layers: []Layer{{Type: "text", Text: "M", FontSize: 18}}},
			{Layers: []Layer{{Type: "text", Text: "M", FontSize: 18, OffsetX: 7, OffsetY: -5}}},
		},
	}

	keys, err := r.ComposePage(page, nil, 0, "Paused", Weather{}, time.Date(2026, 5, 21, 20, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	baseMinX, baseMinY, _, _, ok := nonBlackBounds(keys[0])
	if !ok {
		t.Fatal("base text rendered blank")
	}
	offsetMinX, offsetMinY, _, _, ok := nonBlackBounds(keys[1])
	if !ok {
		t.Fatal("offset text rendered blank")
	}
	if offsetMinX-baseMinX != 7 {
		t.Fatalf("text offset-x moved %d pixels, want 7", offsetMinX-baseMinX)
	}
	if offsetMinY-baseMinY != -5 {
		t.Fatalf("text offset-y moved %d pixels, want -5", offsetMinY-baseMinY)
	}
}

func TestIconLayerAppliesOffsets(t *testing.T) {
	iconDir := t.TempDir()
	writeCenteredIcon(t, filepath.Join(iconDir, "shape.png"), color.RGBA{0, 255, 0, 255})

	r := New(iconDir)
	page := Page{
		Background: Background{Type: "solid"},
		Buttons: []Button{
			{Layers: []Layer{{Type: "icon", Icon: "shape.png", Size: 24}}},
			{Layers: []Layer{{Type: "icon", Icon: "shape.png", Size: 24, OffsetX: -6, OffsetY: 8}}},
		},
	}

	keys, err := r.ComposePage(page, nil, 0, "Paused", Weather{}, time.Date(2026, 5, 21, 20, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	baseMinX, baseMinY, _, _, ok := nonBlackBounds(keys[0])
	if !ok {
		t.Fatal("base icon rendered blank")
	}
	offsetMinX, offsetMinY, _, _, ok := nonBlackBounds(keys[1])
	if !ok {
		t.Fatal("offset icon rendered blank")
	}
	if offsetMinX-baseMinX != -6 {
		t.Fatalf("icon offset-x moved %d pixels, want -6", offsetMinX-baseMinX)
	}
	if offsetMinY-baseMinY != 8 {
		t.Fatalf("icon offset-y moved %d pixels, want 8", offsetMinY-baseMinY)
	}
}

func TestAnimationLayerUsesGIFFrames(t *testing.T) {
	iconDir := t.TempDir()
	writeTestGIF(t, filepath.Join(iconDir, "blink.gif"))

	r := New(iconDir)
	page := Page{
		Background: Background{Type: "solid"},
		Buttons: []Button{
			{Layers: []Layer{{Type: "animation", Path: "blink.gif", Mode: "stretch"}}},
		},
	}

	firstKeys, err := r.ComposePage(page, nil, 0, "Paused", Weather{}, time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	secondKeys, err := r.ComposePage(page, nil, 0, "Paused", Weather{}, time.Unix(0, int64(110*time.Millisecond)))
	if err != nil {
		t.Fatal(err)
	}
	if got := firstKeys[0].RGBAAt(KeyWidth/2, KeyHeight/2); got.R <= got.G {
		t.Fatalf("first animation frame center = %v, want red", got)
	}
	if got := secondKeys[0].RGBAAt(KeyWidth/2, KeyHeight/2); got.G <= got.R {
		t.Fatalf("second animation frame center = %v, want green", got)
	}
}

func TestFormatDateTimeUsesReadableTokens(t *testing.T) {
	now := time.Date(2026, 5, 21, 20, 5, 9, 0, time.UTC)

	got := formatDateTime("ddd DD\nHH:mm", now)
	want := "Thu 21\n20:05"
	if got != want {
		t.Fatalf("formatDateTime = %q, want %q", got, want)
	}
}

func TestComposePageUsesSystemFontWhenConfigured(t *testing.T) {
	fontPath := "/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf"
	if _, err := os.Stat(fontPath); err != nil {
		t.Skip(err)
	}

	r := New("../../../icons", fontPath)
	if r.font == nil {
		t.Fatal("system font was not loaded")
	}

	page := Page{
		Background: Background{Type: "solid"},
		Buttons: []Button{
			{
				Layers: []Layer{{Type: "datetime", Format: "HH:mm", FontSize: 18}},
			},
		},
	}
	keys, err := r.ComposePage(page, nil, 0, "Paused", Weather{}, time.Date(2026, 5, 21, 20, 5, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !hasNonBlackPixel(keys[0]) {
		t.Fatal("datetime rendered with system font is blank")
	}
}

func TestDateTimeWithSystemFontIsVerticallyCentered(t *testing.T) {
	fontPath := "/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf"
	if _, err := os.Stat(fontPath); err != nil {
		t.Skip(err)
	}

	r := New("../../../icons", fontPath)
	page := Page{
		Background: Background{Type: "solid"},
		Buttons: []Button{
			{
				Layers: []Layer{{Type: "datetime", Format: "ddd DD\nHH:mm", FontSize: 18}},
			},
		},
	}
	keys, err := r.ComposePage(page, nil, 0, "Paused", Weather{}, time.Date(2026, 5, 21, 20, 5, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}

	_, minY, _, maxY, ok := nonBlackBounds(keys[0])
	if !ok {
		t.Fatal("datetime rendered with system font is blank")
	}
	centerY := (minY + maxY) / 2
	if centerY < KeyHeight/2-3 || centerY > KeyHeight/2+3 {
		t.Fatalf("datetime vertical center = %d, want near %d (bounds y=%d..%d)", centerY, KeyHeight/2, minY, maxY)
	}
}

func testPage() Page {
	return Page{
		Background: Background{Type: "media_art", Mode: "fill"},
		Buttons: []Button{
			{Layers: []Layer{{Type: "empty"}}},
			{Layers: []Layer{{Type: "empty"}}},
			{Layers: []Layer{{Type: "empty"}}},
			{Layers: []Layer{{Type: "empty"}}},
			{Layers: []Layer{{Type: "empty"}}},
			{Layers: []Layer{{Type: "empty"}}},
		},
	}
}

func hasNonBlackPixel(img *image.RGBA) bool {
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			if img.RGBAAt(x, y) != (color.RGBA{0, 0, 0, 255}) {
				return true
			}
		}
	}
	return false
}

func hasPixel(img *image.RGBA, needle color.RGBA) bool {
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			if img.RGBAAt(x, y) == needle {
				return true
			}
		}
	}
	return false
}

func nonBlackBounds(img *image.RGBA) (int, int, int, int, bool) {
	minX, minY := img.Bounds().Dx(), img.Bounds().Dy()
	maxX, maxY := -1, -1
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			if img.RGBAAt(x, y) == (color.RGBA{0, 0, 0, 255}) {
				continue
			}
			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x > maxX {
				maxX = x
			}
			if y > maxY {
				maxY = y
			}
		}
	}
	return minX, minY, maxX, maxY, maxX >= 0
}

func hasNonBackgroundPixelInBand(img *image.RGBA, background color.RGBA, minY int, maxY int) bool {
	for y := minY; y < maxY; y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			if img.RGBAAt(x, y) != background {
				return true
			}
		}
	}
	return false
}

func writeTestIcon(t *testing.T, dir, name string, c color.RGBA) {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	writePNG(t, filepath.Join(dir, name), img)
}

func intPtr(value int) *int {
	return &value
}

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()

	buf := &bytes.Buffer{}
	if err := png.Encode(buf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeCenteredIcon(t *testing.T, path string, c color.RGBA) {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 3; y < 7; y++ {
		for x := 3; x < 7; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	writePNG(t, path, img)
}

func writeTestGIF(t *testing.T, path string) {
	t.Helper()

	red := image.NewPaletted(image.Rect(0, 0, 10, 10), palette.Plan9)
	green := image.NewPaletted(image.Rect(0, 0, 10, 10), palette.Plan9)
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			red.Set(x, y, color.RGBA{255, 0, 0, 255})
			green.Set(x, y, color.RGBA{0, 255, 0, 255})
		}
	}
	buf := &bytes.Buffer{}
	err := gif.EncodeAll(buf, &gif.GIF{
		Image: []*image.Paletted{red, green},
		Delay: []int{10, 10},
		Config: image.Config{
			ColorModel: color.Palette(palette.Plan9),
			Width:      10,
			Height:     10,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}
