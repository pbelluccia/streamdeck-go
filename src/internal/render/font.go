package render

import (
	"image"
	"image/color"
	"math"
	"os"
	"strings"

	xfont "golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const DefaultTextFontSize = 18

type FontRenderer struct {
	font *opentype.Font
}

func NewFontRenderer(path string) (*FontRenderer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	font, err := opentype.Parse(data)
	if err != nil {
		return nil, err
	}
	return &FontRenderer{font: font}, nil
}

var glyphs = map[rune][7]string{
	'0': {"111", "101", "101", "101", "101", "101", "111"},
	'1': {"010", "110", "010", "010", "010", "010", "111"},
	'2': {"111", "001", "001", "111", "100", "100", "111"},
	'3': {"111", "001", "001", "111", "001", "001", "111"},
	'4': {"101", "101", "101", "111", "001", "001", "001"},
	'5': {"111", "100", "100", "111", "001", "001", "111"},
	'6': {"111", "100", "100", "111", "101", "101", "111"},
	'7': {"111", "001", "001", "010", "010", "010", "010"},
	'8': {"111", "101", "101", "111", "101", "101", "111"},
	'9': {"111", "101", "101", "111", "001", "001", "111"},
	':': {"0", "1", "0", "0", "1", "0", "0"},
	'-': {"000", "000", "000", "111", "000", "000", "000"},
	'C': {"111", "100", "100", "100", "100", "100", "111"},
	' ': {"0", "0", "0", "0", "0", "0", "0"},
	'M': {"10001", "11011", "10101", "10101", "10001", "10001", "10001"},
	'o': {"000", "000", "111", "101", "101", "101", "111"},
	'n': {"000", "000", "110", "101", "101", "101", "101"},
	'T': {"11111", "00100", "00100", "00100", "00100", "00100", "00100"},
	'u': {"000", "000", "101", "101", "101", "101", "111"},
	'e': {"000", "000", "111", "100", "111", "100", "111"},
	'W': {"10001", "10001", "10001", "10101", "10101", "10101", "01010"},
	'd': {"001", "001", "111", "101", "101", "101", "111"},
	'h': {"100", "100", "110", "101", "101", "101", "101"},
	'F': {"111", "100", "100", "111", "100", "100", "100"},
	'r': {"000", "000", "110", "101", "100", "100", "100"},
	'i': {"1", "0", "1", "1", "1", "1", "1"},
	'S': {"111", "100", "100", "111", "001", "001", "111"},
	'a': {"000", "000", "111", "001", "111", "101", "111"},
	't': {"010", "010", "111", "010", "010", "010", "011"},
	'°': {"11", "11", "00", "00", "00", "00", "00"},
}

func TextSize(text string, scale int) (int, int) {
	maxW := 0
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		w := 0
		for _, r := range line {
			g := glyphFor(r)
			w += len(g[0])*scale + scale
		}
		if w > 0 {
			w -= scale
		}
		if w > maxW {
			maxW = w
		}
	}
	return maxW, len(lines)*7*scale + (len(lines)-1)*scale
}

func DrawText(img *image.RGBA, x, y int, text string, scale int, fill color.RGBA, stroke color.RGBA, strokeWidth int) {
	for dy := -strokeWidth; dy <= strokeWidth; dy++ {
		for dx := -strokeWidth; dx <= strokeWidth; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			drawTextNoStroke(img, x+dx, y+dy, text, scale, stroke)
		}
	}
	drawTextNoStroke(img, x, y, text, scale, fill)
}

func defaultFontSize(fontSize int) int {
	if fontSize <= 0 {
		return DefaultTextFontSize
	}
	return fontSize
}

func bitmapScaleForFontSize(fontSize int) int {
	scale := (defaultFontSize(fontSize) + 6) / 7
	if scale < 1 {
		return 1
	}
	return scale
}

func (r *Renderer) textSize(text string, fontSize int) (int, int) {
	if r.font == nil {
		return TextSize(text, bitmapScaleForFontSize(fontSize))
	}
	w, h, err := r.font.textSize(text, fontSize)
	if err != nil {
		return TextSize(text, bitmapScaleForFontSize(fontSize))
	}
	return w, h
}

func (r *Renderer) drawText(img *image.RGBA, x, y int, text string, fontSize int, fill color.RGBA, stroke color.RGBA, strokeWidth int) {
	if r.font == nil {
		DrawText(img, x, y, text, bitmapScaleForFontSize(fontSize), fill, stroke, strokeWidth)
		return
	}
	if err := r.font.drawText(img, x, y, text, fontSize, fill, stroke, strokeWidth); err != nil {
		DrawText(img, x, y, text, bitmapScaleForFontSize(fontSize), fill, stroke, strokeWidth)
	}
}

func (f *FontRenderer) textSize(text string, fontSize int) (int, int, error) {
	face, err := f.face(fontSize)
	if err != nil {
		return 0, 0, err
	}
	defer face.Close()

	metrics := face.Metrics()
	lineHeight := ceilFixed(metrics.Height)
	if lineHeight <= 0 {
		lineHeight = int(math.Ceil(float64(fontSize) * 1.2))
	}

	maxW := 0
	lines := strings.Split(text, "\n")
	drawer := &xfont.Drawer{Face: face}
	for _, line := range lines {
		w := ceilFixed(drawer.MeasureString(line))
		if w > maxW {
			maxW = w
		}
	}
	return maxW, lineHeight * len(lines), nil
}

func (f *FontRenderer) drawText(img *image.RGBA, x, y int, text string, fontSize int, fill color.RGBA, stroke color.RGBA, strokeWidth int) error {
	if strokeWidth > 0 {
		for dy := -strokeWidth; dy <= strokeWidth; dy++ {
			for dx := -strokeWidth; dx <= strokeWidth; dx++ {
				if dx == 0 && dy == 0 {
					continue
				}
				if err := f.drawTextNoStroke(img, x+dx, y+dy, text, fontSize, stroke); err != nil {
					return err
				}
			}
		}
	}
	return f.drawTextNoStroke(img, x, y, text, fontSize, fill)
}

func (f *FontRenderer) drawTextNoStroke(img *image.RGBA, x, y int, text string, fontSize int, fill color.RGBA) error {
	face, err := f.face(fontSize)
	if err != nil {
		return err
	}
	defer face.Close()

	metrics := face.Metrics()
	ascent := ceilFixed(metrics.Ascent)
	lineHeight := ceilFixed(metrics.Height)
	if lineHeight <= 0 {
		lineHeight = int(math.Ceil(float64(fontSize) * 1.2))
	}

	drawer := &xfont.Drawer{
		Dst:  img,
		Src:  image.NewUniform(fill),
		Face: face,
	}
	for i, line := range strings.Split(text, "\n") {
		drawer.Dot = fixed.P(x, y+ascent+i*lineHeight)
		drawer.DrawString(line)
	}
	return nil
}

func ceilFixed(value fixed.Int26_6) int {
	return int((value + 63) >> 6)
}

func (f *FontRenderer) face(fontSize int) (xfont.Face, error) {
	return opentype.NewFace(f.font, &opentype.FaceOptions{
		Size:    float64(defaultFontSize(fontSize)),
		DPI:     72,
		Hinting: xfont.HintingFull,
	})
}

func drawTextNoStroke(img *image.RGBA, x, y int, text string, scale int, fill color.RGBA) {
	startX := x
	for _, r := range text {
		if r == '\n' {
			x = startX
			y += 8 * scale
			continue
		}
		g := glyphFor(r)
		for gy, row := range g {
			for gx, bit := range row {
				if bit != '1' {
					continue
				}
				fillRect(img, x+gx*scale, y+gy*scale, scale, scale, fill)
			}
		}
		x += (len(g[0]) + 1) * scale
	}
}

func glyphFor(r rune) [7]string {
	if g, ok := glyphs[r]; ok {
		return g
	}
	return [7]string{"111", "001", "010", "010", "000", "010", "000"}
}

func fillRect(img *image.RGBA, x, y, w, h int, c color.RGBA) {
	b := img.Bounds()
	for yy := y; yy < y+h; yy++ {
		for xx := x; xx < x+w; xx++ {
			if xx >= b.Min.X && xx < b.Max.X && yy >= b.Min.Y && yy < b.Max.Y {
				img.SetRGBA(xx, yy, c)
			}
		}
	}
}
