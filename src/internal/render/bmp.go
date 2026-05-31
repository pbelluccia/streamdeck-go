package render

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
)

func RotateClockwise(src image.Image) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, h, w))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(h-1-y, x, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

func RotateCounterClockwise(src image.Image) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, h, w))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(y, w-1-x, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

func Rotate180(src image.Image) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(w-1-x, h-1-y, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

func FlipVertical(src image.Image) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(x, h-1-y, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

func EncodeBMP24(img image.Image) []byte {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	rowSize := ((24*w + 31) / 32) * 4
	pixelSize := rowSize * h
	fileSize := 54 + pixelSize

	buf := bytes.NewBuffer(make([]byte, 0, fileSize))
	buf.Write([]byte{'B', 'M'})
	writeLE(buf, uint32(fileSize))
	writeLE(buf, uint16(0))
	writeLE(buf, uint16(0))
	writeLE(buf, uint32(54))
	writeLE(buf, uint32(40))
	writeLE(buf, int32(w))
	writeLE(buf, int32(h))
	writeLE(buf, uint16(1))
	writeLE(buf, uint16(24))
	writeLE(buf, uint32(0))
	writeLE(buf, uint32(pixelSize))
	writeLE(buf, int32(2835))
	writeLE(buf, int32(2835))
	writeLE(buf, uint32(0))
	writeLE(buf, uint32(0))

	padding := make([]byte, rowSize-w*3)
	for y := h - 1; y >= 0; y-- {
		for x := 0; x < w; x++ {
			c := color.RGBAModel.Convert(img.At(b.Min.X+x, b.Min.Y+y)).(color.RGBA)
			buf.WriteByte(c.B)
			buf.WriteByte(c.G)
			buf.WriteByte(c.R)
		}
		buf.Write(padding)
	}
	return buf.Bytes()
}

func writeLE(buf *bytes.Buffer, value any) {
	_ = binary.Write(buf, binary.LittleEndian, value)
}
