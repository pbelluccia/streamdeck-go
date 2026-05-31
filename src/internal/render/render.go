package render

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	KeyWidth      = 80
	KeyHeight     = 80
	GapMM         = 6.0
	DPI           = 72.0
	MediaIconSize = 50
	WeatherSize   = 50
	OutlineWidth  = 2
)

type Layout struct {
	Columns   int
	Rows      int
	KeyWidth  int
	KeyHeight int
}

func DefaultLayout() Layout {
	return Layout{
		Columns:   3,
		Rows:      2,
		KeyWidth:  KeyWidth,
		KeyHeight: KeyHeight,
	}
}

func (l Layout) KeyCount() int {
	l = l.withDefaults()
	return l.Columns * l.Rows
}

func (l Layout) withDefaults() Layout {
	defaults := DefaultLayout()
	if l.Columns <= 0 {
		l.Columns = defaults.Columns
	}
	if l.Rows <= 0 {
		l.Rows = defaults.Rows
	}
	if l.KeyWidth <= 0 {
		l.KeyWidth = defaults.KeyWidth
	}
	if l.KeyHeight <= 0 {
		l.KeyHeight = defaults.KeyHeight
	}
	return l
}

type Weather struct {
	Condition   string
	Temperature int
	OK          bool
}

type Background struct {
	Type   string `json:"type"`
	Path   string `json:"path,omitempty"`
	Player string `json:"player,omitempty"`
	Mode   string `json:"mode,omitempty"`
	Color  string `json:"color,omitempty"`
	Effect Effect `json:"effect,omitempty"`
}

type Effect struct {
	Type       string `json:"type,omitempty"`
	Color      string `json:"color,omitempty"`
	BlinkMS    int    `json:"blink_ms,omitempty"`
	DurationMS int    `json:"duration_ms,omitempty"`
	Repeat     int    `json:"repeat,omitempty"`
}

type Layer struct {
	Type         string `json:"type"`
	Icon         string `json:"icon,omitempty"`
	Path         string `json:"path,omitempty"`
	Mode         string `json:"mode,omitempty"`
	Format       string `json:"format,omitempty"`
	Text         string `json:"text,omitempty"`
	Position     string `json:"position,omitempty"`
	Player       string `json:"player,omitempty"`
	FontSize     int    `json:"font_size,omitempty"`
	Size         int    `json:"size,omitempty"`
	OffsetX      int    `json:"offset-x,omitempty"`
	OffsetY      int    `json:"offset-y,omitempty"`
	Color        string `json:"color,omitempty"`
	OutlineColor string `json:"outline_color,omitempty"`
	OutlineWidth *int   `json:"outline_width,omitempty"`
	Effect       Effect `json:"effect,omitempty"`
	Playback     string `json:"-"`
}

type Button struct {
	Layers []Layer
}

type Page struct {
	Background Background
	Buttons    []Button
}

type Renderer struct {
	layout     Layout
	iconDir    string
	icons      map[string]*image.RGBA
	images     map[string]image.Image
	animations map[string]*Animation
	font       *FontRenderer
	started    time.Time
}

type Animation struct {
	Frames   []image.Image
	Delays   []time.Duration
	Duration time.Duration
}

func New(iconDir string, fontPath ...string) *Renderer {
	return NewWithLayout(iconDir, DefaultLayout(), fontPath...)
}

func NewWithLayout(iconDir string, layout Layout, fontPath ...string) *Renderer {
	r := &Renderer{
		layout:     layout.withDefaults(),
		iconDir:    iconDir,
		icons:      map[string]*image.RGBA{},
		images:     map[string]image.Image{},
		animations: map[string]*Animation{},
		started:    time.Now(),
	}
	if len(fontPath) > 0 && strings.TrimSpace(fontPath[0]) != "" {
		font, err := NewFontRenderer(fontPath[0])
		if err != nil {
			fmt.Println("Error loading font:", err)
		} else {
			r.font = font
		}
	}
	return r
}

func DecodeImage(data []byte) (image.Image, error) {
	if img, err := jpeg.Decode(bytes.NewReader(data)); err == nil {
		return img, nil
	}
	if img, err := png.Decode(bytes.NewReader(data)); err == nil {
		return img, nil
	}
	return nil, fmt.Errorf("unsupported image data")
}

func (r *Renderer) ComposePage(page Page, album image.Image, mode int, playback string, weather Weather, now time.Time) ([]*image.RGBA, error) {
	layout := r.layout.withDefaults()
	gapMM := float64(GapMM)
	gapPixels := int((gapMM / 25.4) * float64(DPI))
	compositeW := layout.Columns*layout.KeyWidth + (layout.Columns-1)*gapPixels
	compositeH := layout.Rows*layout.KeyHeight + (layout.Rows-1)*gapPixels
	composite, _ := r.composeBackground(page.Background, album, compositeW, compositeH, mode, now)

	keys := make([]*image.RGBA, layout.KeyCount())
	for key := 0; key < len(keys); key++ {
		row := key / layout.Columns
		col := key % layout.Columns
		x := col * (layout.KeyWidth + gapPixels)
		y := row * (layout.KeyHeight + gapPixels)
		keys[key] = crop(composite, x, y, layout.KeyWidth, layout.KeyHeight)
		if mode == 0 || mode == 3 {
			r.overlayLayers(keys[key], page.buttonForKey(key).Layers, key, playback, weather, now)
		}
		if mode == 2 {
			keys[key] = solid(layout.KeyWidth, layout.KeyHeight, color.RGBA{0, 0, 0, 255})
		}
	}
	return keys, nil
}

func (r *Renderer) composeBackground(background Background, album image.Image, w int, h int, displayMode int, now time.Time) (*image.RGBA, bool) {
	if displayMode == 2 || displayMode == 3 {
		return solid(w, h, color.RGBA{0, 0, 0, 255}), false
	}

	switch background.Type {
	case "solid", "color":
		c := color.RGBA{0, 0, 0, 255}
		if parsed, err := parseOptionalColor(background.Color, c); err == nil {
			c = parsed
		} else {
			fmt.Println("Invalid background color:", err)
		}
		c = applyColorEffect(c, background.Effect, now, r.started)
		return solid(w, h, c), false
	case "media_art":
		if album == nil {
			c := color.RGBA{0, 0, 0, 255}
			c = applyColorEffect(c, background.Effect, now, r.started)
			return solid(w, h, c), false
		}
		return resizeFill(album, w, h), true
	case "image":
		img, err := r.loadImage(background.Path)
		if err != nil {
			fmt.Println("Error loading background image", background.Path+":", err)
			return solid(w, h, color.RGBA{0, 0, 0, 255}), false
		}
		return renderImageToSize(img, w, h, background.Mode), true
	default:
		c := color.RGBA{0, 0, 0, 255}
		c = applyColorEffect(c, background.Effect, now, r.started)
		return solid(w, h, c), false
	}
}

func (r *Renderer) KeyToBMP(img image.Image) []byte {
	return EncodeBMP24(FlipVertical(RotateCounterClockwise(img)))
}

func (r *Renderer) KeyToJPEG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, Rotate180(img), &jpeg.Options{Quality: 95}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (p Page) buttonForKey(key int) Button {
	if key < 0 || key >= len(p.Buttons) {
		return Button{}
	}
	return p.Buttons[key]
}

func (r *Renderer) overlayLayers(dst *image.RGBA, layers []Layer, key int, playback string, weather Weather, now time.Time) {
	for _, layer := range layers {
		r.overlayLayer(dst, layer, key, playback, weather, now)
	}
}

func (r *Renderer) overlayLayer(dst *image.RGBA, layer Layer, key int, playback string, weather Weather, now time.Time) {
	switch layer.Type {
	case "", "empty":
		return
	case "icon":
		r.overlayIcon(dst, layer.Icon, defaultLayerSize(layer.Size, MediaIconSize, r.keyWidth()), layer.Position, layer.OffsetX, layer.OffsetY, layer.OutlineColor, layer.OutlineWidth)
	case "image":
		r.overlayImage(dst, layer.Path, layer.Mode)
	case "color":
		r.overlayColor(dst, layer.Color, layer.Effect, now)
	case "animation":
		r.overlayAnimation(dst, layer.Path, layer.Mode, layer.OffsetX, layer.OffsetY, now)
	case "media_play_pause":
		if layer.Playback != "" {
			playback = layer.Playback
		}
		r.overlayMediaPlayPause(dst, playback, defaultLayerSize(layer.Size, MediaIconSize, r.keyWidth()), layer.Position, layer.OffsetX, layer.OffsetY, layer.OutlineColor, layer.OutlineWidth)
	case "weather":
		r.overlayWeather(dst, weather, layer.FontSize, layer.Position, layer.OffsetX, layer.OffsetY, layer.Color, layer.OutlineColor, layer.OutlineWidth)
	case "datetime":
		r.overlayDateTime(dst, layer.Format, layer.FontSize, layer.Position, layer.OffsetX, layer.OffsetY, layer.Color, layer.OutlineColor, layer.OutlineWidth, now)
	case "text":
		r.overlayText(dst, layer.Text, layer.FontSize, layer.Position, layer.OffsetX, layer.OffsetY, layer.Color, layer.OutlineColor, layer.OutlineWidth)
	default:
		fmt.Println("Unknown layer type:", layer.Type, "on key", key)
	}
}

func (r *Renderer) overlayColor(dst *image.RGBA, value string, effect Effect, now time.Time) {
	c := color.RGBA{0, 0, 0, 255}
	if parsed, err := parseOptionalColor(value, c); err == nil {
		c = parsed
	} else {
		fmt.Println("Invalid color layer color:", err)
	}
	c = applyColorEffect(c, effect, now, r.started)
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: c}, image.Point{}, draw.Src)
}

func (r *Renderer) overlayMediaPlayPause(dst *image.RGBA, playback string, size int, position string, offsetX int, offsetY int, outlineValue string, outlineWidth *int) {
	name := ""
	if playback == "Playing" {
		name = "pause.png"
	} else {
		name = "play.png"
	}
	r.overlayIcon(dst, name, size, position, offsetX, offsetY, outlineValue, outlineWidth)
}

func (r *Renderer) overlayIcon(dst *image.RGBA, name string, size int, position string, offsetX int, offsetY int, outlineValue string, outlineWidth *int) {
	outlineColor := color.RGBA{0, 0, 0, 255}
	if parsed, err := parseOptionalColor(outlineValue, outlineColor); err == nil {
		outlineColor = parsed
	} else {
		fmt.Println("Invalid icon outline_color:", err)
	}
	icon, err := r.loadIcon(name, size, defaultOutlineWidth(outlineWidth), outlineColor)
	if err != nil {
		fmt.Println("Error loading icon", name+":", err)
		return
	}
	alphaOver(dst, icon, (r.keyWidth()-size)/2+offsetX, positionedY(size, position, 4, r.keyHeight())+offsetY)
}

func (r *Renderer) overlayImage(dst *image.RGBA, path string, mode string) {
	img, err := r.loadImage(path)
	if err != nil {
		fmt.Println("Error loading image", path+":", err)
		return
	}
	r.overlayImageSource(dst, img, mode, 0, 0)
}

func (r *Renderer) overlayAnimation(dst *image.RGBA, path string, mode string, offsetX int, offsetY int, now time.Time) {
	animation, err := r.loadAnimation(path)
	if err != nil {
		fmt.Println("Error loading animation", path+":", err)
		return
	}
	frame := animation.Frame(now)
	if frame == nil {
		return
	}
	r.overlayImageSource(dst, frame, mode, offsetX, offsetY)
}

func (r *Renderer) overlayImageSource(dst *image.RGBA, img image.Image, mode string, offsetX int, offsetY int) {
	mode = strings.TrimSpace(mode)
	if mode == "" {
		mode = "fit"
	}

	var src image.Image
	switch mode {
	case "fit":
		src = resizeFit(img, r.keyWidth(), r.keyHeight())
	case "fill":
		src = resizeFill(img, r.keyWidth(), r.keyHeight())
	case "center":
		src = img
	case "stretch":
		src = resizeStretch(img, r.keyWidth(), r.keyHeight())
	default:
		fmt.Println("Unknown image mode:", mode)
		src = resizeFit(img, r.keyWidth(), r.keyHeight())
	}

	sb := src.Bounds()
	alphaOver(dst, src, (r.keyWidth()-sb.Dx())/2+offsetX, (r.keyHeight()-sb.Dy())/2+offsetY)
}

func renderImageToSize(img image.Image, w int, h int, mode string) *image.RGBA {
	mode = strings.TrimSpace(mode)
	if mode == "" {
		mode = "fill"
	}
	dst := solid(w, h, color.RGBA{0, 0, 0, 255})
	var src image.Image
	switch mode {
	case "fit":
		src = resizeFit(img, w, h)
	case "fill":
		return resizeFill(img, w, h)
	case "center":
		src = img
	case "stretch":
		return resizeStretch(img, w, h)
	default:
		fmt.Println("Unknown background image mode:", mode)
		return resizeFill(img, w, h)
	}
	sb := src.Bounds()
	alphaOver(dst, src, (w-sb.Dx())/2, (h-sb.Dy())/2)
	return dst
}

func (r *Renderer) overlayWeather(dst *image.RGBA, weather Weather, fontSize int, position string, offsetX int, offsetY int, fillValue string, outlineValue string, outlineWidth *int) {
	temp := "--\u00b0C"
	icon := "weather_cloudy.png"
	if weather.OK {
		temp = fmt.Sprintf("%d\u00b0C", weather.Temperature)
		condition := strings.ToLower(weather.Condition)
		switch {
		case strings.Contains(condition, "clear"):
			icon = "weather_sunny.png"
		case strings.Contains(condition, "cloud"):
			icon = "weather_cloudy.png"
		case strings.Contains(condition, "rain"), strings.Contains(condition, "drizzle"), strings.Contains(condition, "thunderstorm"):
			icon = "weather_rain.png"
		case strings.Contains(condition, "snow"):
			icon = "weather_snow.png"
		}
	}
	fontSize = defaultFontSize(fontSize)
	tw, th := r.textSize(temp, fontSize)
	weatherSize := defaultLayerSize(0, WeatherSize, r.keyWidth())
	totalH := weatherSize + 2 + th
	y := positionedY(totalH, position, 2, r.keyHeight()) + offsetY
	outline := color.RGBA{0, 0, 0, 255}
	if parsed, err := parseOptionalColor(outlineValue, outline); err == nil {
		outline = parsed
	} else {
		fmt.Println("Invalid weather outline_color:", err)
	}
	if img, err := r.loadIcon(icon, weatherSize, defaultOutlineWidth(outlineWidth), outline); err == nil {
		alphaOver(dst, img, (r.keyWidth()-weatherSize)/2+offsetX, y)
	}
	fill := color.RGBA{255, 255, 255, 255}
	if parsed, err := parseOptionalColor(fillValue, fill); err == nil {
		fill = parsed
	} else {
		fmt.Println("Invalid weather text color:", err)
	}
	r.drawText(dst, (r.keyWidth()-tw)/2+offsetX, y+weatherSize+2, temp, fontSize, fill, outline, defaultOutlineWidth(outlineWidth))
}

func (r *Renderer) overlayDay(dst *image.RGBA, now time.Time) {
	r.overlayDateTime(dst, "ddd DD\nHH:mm", 0, "center", 0, 0, "", "", nil, now)
}

func (r *Renderer) overlayDateTime(dst *image.RGBA, format string, fontSize int, position string, offsetX int, offsetY int, fillValue string, outlineValue string, outlineWidth *int, now time.Time) {
	text := formatDateTime(format, now)
	r.overlayText(dst, text, fontSize, position, offsetX, offsetY, fillValue, outlineValue, outlineWidth)
}

func (r *Renderer) overlayText(dst *image.RGBA, text string, fontSize int, position string, offsetX int, offsetY int, fillValue string, outlineValue string, outlineWidth *int) {
	fontSize = defaultFontSize(fontSize)
	tw, th := r.textSize(text, fontSize)
	fill := color.RGBA{255, 255, 255, 255}
	if parsed, err := parseOptionalColor(fillValue, fill); err == nil {
		fill = parsed
	} else {
		fmt.Println("Invalid text color:", err)
	}
	outline := color.RGBA{0, 0, 0, 255}
	if parsed, err := parseOptionalColor(outlineValue, outline); err == nil {
		outline = parsed
	} else {
		fmt.Println("Invalid text outline_color:", err)
	}
	r.drawText(dst, (r.keyWidth()-tw)/2+offsetX, positionedY(th, position, 4, r.keyHeight())+offsetY, text, fontSize, fill, outline, defaultOutlineWidth(outlineWidth))
}

func formatDateTime(format string, now time.Time) string {
	if format == "" {
		return ""
	}

	tokens := []struct {
		token string
		value string
	}{
		{"YYYY", now.Format("2006")},
		{"YY", now.Format("06")},
		{"dddd", now.Format("Monday")},
		{"ddd", now.Format("Mon")},
		{"MMMM", now.Format("January")},
		{"MMM", now.Format("Jan")},
		{"MM", now.Format("01")},
		{"M", fmt.Sprintf("%d", int(now.Month()))},
		{"DD", now.Format("02")},
		{"D", fmt.Sprintf("%d", now.Day())},
		{"HH", now.Format("15")},
		{"H", fmt.Sprintf("%d", now.Hour())},
		{"hh", now.Format("03")},
		{"h", strings.TrimLeft(now.Format("03"), "0")},
		{"mm", now.Format("04")},
		{"m", fmt.Sprintf("%d", now.Minute())},
		{"ss", now.Format("05")},
		{"s", fmt.Sprintf("%d", now.Second())},
		{"A", now.Format("PM")},
		{"a", strings.ToLower(now.Format("PM"))},
	}

	var result strings.Builder
	for i := 0; i < len(format); {
		matched := false
		for _, token := range tokens {
			if strings.HasPrefix(format[i:], token.token) {
				result.WriteString(token.value)
				i += len(token.token)
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		result.WriteByte(format[i])
		i++
	}
	return result.String()
}

func (r *Renderer) loadIcon(name string, size int, outlineWidth int, outlineColor color.RGBA) (*image.RGBA, error) {
	key := fmt.Sprintf("%s:%d:%d:%02x%02x%02x", name, size, outlineWidth, outlineColor.R, outlineColor.G, outlineColor.B)
	if img, ok := r.icons[key]; ok {
		return cloneRGBA(img), nil
	}
	data, err := os.ReadFile(filepath.Join(r.iconDir, name))
	if err != nil {
		return nil, err
	}
	src, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	img := resizeFit(src, size, size)
	img = outline(img, outlineWidth, outlineColor)
	r.icons[key] = img
	return cloneRGBA(img), nil
}

func (r *Renderer) loadImage(path string) (image.Image, error) {
	path = r.resolveAssetPath(path)
	if img, ok := r.images[path]; ok {
		return img, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	img, err := DecodeImage(data)
	if err != nil {
		return nil, err
	}
	r.images[path] = img
	return img, nil
}

func (r *Renderer) loadAnimation(path string) (*Animation, error) {
	path = r.resolveAssetPath(path)
	if animation, ok := r.animations[path]; ok {
		return animation, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	gifImage, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	animation := animationFromGIF(gifImage)
	r.animations[path] = animation
	return animation, nil
}

func (r *Renderer) resolveAssetPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(r.iconDir, path)
}

func animationFromGIF(gifImage *gif.GIF) *Animation {
	if gifImage == nil || len(gifImage.Image) == 0 {
		return &Animation{}
	}
	bounds := image.Rect(0, 0, gifImage.Config.Width, gifImage.Config.Height)
	canvas := image.NewRGBA(bounds)
	frames := make([]image.Image, 0, len(gifImage.Image))
	delays := make([]time.Duration, 0, len(gifImage.Image))
	total := time.Duration(0)
	for i, frame := range gifImage.Image {
		draw.Draw(canvas, frame.Bounds(), frame, frame.Bounds().Min, draw.Over)
		frames = append(frames, cloneRGBA(canvas))
		delay := 100 * time.Millisecond
		if i < len(gifImage.Delay) && gifImage.Delay[i] > 0 {
			delay = time.Duration(gifImage.Delay[i]) * 10 * time.Millisecond
		}
		delays = append(delays, delay)
		total += delay
	}
	return &Animation{Frames: frames, Delays: delays, Duration: total}
}

func (a *Animation) Frame(now time.Time) image.Image {
	if a == nil || len(a.Frames) == 0 {
		return nil
	}
	if len(a.Frames) == 1 || a.Duration <= 0 {
		return a.Frames[0]
	}
	elapsed := time.Duration(now.UnixNano() % int64(a.Duration))
	accumulated := time.Duration(0)
	for i, delay := range a.Delays {
		accumulated += delay
		if elapsed < accumulated {
			return a.Frames[i]
		}
	}
	return a.Frames[len(a.Frames)-1]
}

func solid(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: c}, image.Point{}, draw.Src)
	return img
}

func parseHexColor(value string) (color.RGBA, error) {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "#")
	if len(value) != 6 {
		return color.RGBA{}, fmt.Errorf("expected #RRGGBB")
	}
	n, err := strconv.ParseUint(value, 16, 32)
	if err != nil {
		return color.RGBA{}, err
	}
	return color.RGBA{
		R: uint8(n >> 16),
		G: uint8(n >> 8),
		B: uint8(n),
		A: 255,
	}, nil
}

func parseOptionalColor(value string, fallback color.RGBA) (color.RGBA, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	return parseHexColor(value)
}

func applyColorEffect(base color.RGBA, effect Effect, now time.Time, origin time.Time) color.RGBA {
	effectType := strings.TrimSpace(effect.Type)
	if effectType == "" {
		return base
	}
	alt := color.RGBA{255, 255, 255, 255}
	if parsed, err := parseOptionalColor(effect.Color, alt); err == nil {
		alt = parsed
	} else {
		fmt.Println("Invalid effect color:", err)
	}

	elapsed := now.Sub(origin)
	if elapsed < 0 {
		elapsed = 0
	}

	switch effectType {
	case "blink":
		if !effectActive(effect, elapsed) {
			return base
		}
		blink := time.Duration(effect.BlinkMS) * time.Millisecond
		if blink <= 0 {
			blink = 500 * time.Millisecond
		}
		if (elapsed/blink)%2 == 0 {
			return alt
		}
		return base
	case "pulse":
		cycle := effectDuration(effect, 1000*time.Millisecond)
		if !effectRepeating(effect, elapsed, cycle) {
			return base
		}
		progress := float64(elapsed%cycle) / float64(cycle)
		if progress > 0.5 {
			progress = 1 - progress
		}
		return mixColor(base, alt, progress*2)
	case "flash":
		on := effectDuration(effect, 250*time.Millisecond)
		cycle := on * 2
		if !effectRepeating(effect, elapsed, cycle) {
			return base
		}
		if elapsed%cycle < on {
			return alt
		}
		return base
	default:
		return base
	}
}

func effectActive(effect Effect, elapsed time.Duration) bool {
	duration := time.Duration(effect.DurationMS) * time.Millisecond
	if duration <= 0 {
		return true
	}
	if effect.Repeat <= 0 {
		return true
	}
	total := duration * time.Duration(effect.Repeat)
	if total <= 0 {
		return true
	}
	return elapsed < total
}

func effectDuration(effect Effect, fallback time.Duration) time.Duration {
	duration := time.Duration(effect.DurationMS) * time.Millisecond
	if duration <= 0 {
		return fallback
	}
	return duration
}

func effectRepeating(effect Effect, elapsed time.Duration, cycle time.Duration) bool {
	if cycle <= 0 || effect.Repeat <= 0 {
		return true
	}
	return elapsed < cycle*time.Duration(effect.Repeat)
}

func mixColor(a color.RGBA, b color.RGBA, amount float64) color.RGBA {
	if amount < 0 {
		amount = 0
	}
	if amount > 1 {
		amount = 1
	}
	return color.RGBA{
		R: uint8(float64(a.R)*(1-amount) + float64(b.R)*amount + 0.5),
		G: uint8(float64(a.G)*(1-amount) + float64(b.G)*amount + 0.5),
		B: uint8(float64(a.B)*(1-amount) + float64(b.B)*amount + 0.5),
		A: 255,
	}
}

func defaultLayerSize(size int, fallback int, max int) int {
	if size <= 0 {
		return fallback
	}
	if size > max {
		return max
	}
	return size
}

func defaultOutlineWidth(width *int) int {
	if width == nil {
		return OutlineWidth
	}
	if *width < 0 {
		return 0
	}
	return *width
}

func positionedY(height int, position string, margin int, keyHeight int) int {
	if height > keyHeight {
		return (keyHeight - height) / 2
	}
	switch strings.ToLower(strings.TrimSpace(position)) {
	case "upper", "top":
		return margin
	case "lower", "bottom":
		return keyHeight - height - margin
	default:
		return (keyHeight - height) / 2
	}
}

func (r *Renderer) keyWidth() int {
	return r.layout.withDefaults().KeyWidth
}

func (r *Renderer) keyHeight() int {
	return r.layout.withDefaults().KeyHeight
}

func crop(src image.Image, x, y, w, h int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), src, image.Point{X: x, Y: y}, draw.Src)
	return dst
}

func resizeFill(src image.Image, w, h int) *image.RGBA {
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	if sw == 0 || sh == 0 {
		return solid(w, h, color.RGBA{0, 0, 0, 255})
	}
	scaleW := float64(w) / float64(sw)
	scaleH := float64(h) / float64(sh)
	scale := scaleW
	if scaleH > scale {
		scale = scaleH
	}
	nw := int(float64(sw)*scale + 0.5)
	nh := int(float64(sh)*scale + 0.5)
	resized := resizeNearest(src, nw, nh)
	return crop(resized, (nw-w)/2, (nh-h)/2, w, h)
}

func resizeFit(src image.Image, w, h int) *image.RGBA {
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	scaleW := float64(w) / float64(sw)
	scaleH := float64(h) / float64(sh)
	scale := scaleW
	if scaleH < scale {
		scale = scaleH
	}
	nw := int(float64(sw)*scale + 0.5)
	nh := int(float64(sh)*scale + 0.5)
	resized := resizeNearest(src, nw, nh)
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	alphaOver(dst, resized, (w-nw)/2, (h-nh)/2)
	return dst
}

func resizeStretch(src image.Image, w, h int) *image.RGBA {
	return resizeNearest(src, w, h)
}

func resizeNearest(src image.Image, w, h int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	for y := 0; y < h; y++ {
		sy := sb.Min.Y + y*sh/h
		for x := 0; x < w; x++ {
			sx := sb.Min.X + x*sw/w
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}

func outline(src *image.RGBA, width int, c color.RGBA) *image.RGBA {
	if width <= 0 {
		return src
	}
	dst := image.NewRGBA(src.Bounds())
	b := src.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			_, _, _, a := src.At(x, y).RGBA()
			if a == 0 {
				continue
			}
			for dy := -width; dy <= width; dy++ {
				for dx := -width; dx <= width; dx++ {
					if dx == 0 && dy == 0 {
						continue
					}
					px, py := x+dx, y+dy
					if px >= b.Min.X && px < b.Max.X && py >= b.Min.Y && py < b.Max.Y {
						dst.SetRGBA(px, py, c)
					}
				}
			}
		}
	}
	alphaOver(dst, src, 0, 0)
	return dst
}

func alphaOver(dst *image.RGBA, src image.Image, ox, oy int) {
	sb := src.Bounds()
	for y := 0; y < sb.Dy(); y++ {
		for x := 0; x < sb.Dx(); x++ {
			dx, dy := ox+x, oy+y
			if !image.Pt(dx, dy).In(dst.Bounds()) {
				continue
			}
			sr, sg, sbv, sa := src.At(sb.Min.X+x, sb.Min.Y+y).RGBA()
			if sa == 0 {
				continue
			}
			dr, dg, db, da := dst.At(dx, dy).RGBA()
			a := float64(sa) / 65535.0
			ia := 1.0 - a
			outA := uint32(float64(sa) + float64(da)*ia)
			dst.SetRGBA(dx, dy, color.RGBA{
				R: uint8((float64(sr)*a + float64(dr)*ia) / 257.0),
				G: uint8((float64(sg)*a + float64(dg)*ia) / 257.0),
				B: uint8((float64(sbv)*a + float64(db)*ia) / 257.0),
				A: uint8(outA / 257),
			})
		}
	}
}

func cloneRGBA(src *image.RGBA) *image.RGBA {
	dst := image.NewRGBA(src.Bounds())
	copy(dst.Pix, src.Pix)
	return dst
}
