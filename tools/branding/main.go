// Command branding reads a source PNG and writes a multi-size Windows .ico plus a lean favicon
// PNG. Stdlib only (hand-rolled bilinear resize + PNG-in-ICO), so no deps/network. Driven by
// branding.ps1, which also runs rsrc to turn the .ico into the .syso resources.
//
//	go run . <src.png> <out.ico> <favicon.png>
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: mkico <src.png> <out.ico> <favicon.png>")
		os.Exit(2)
	}
	src := decode(os.Args[1])

	// favicon: a single 128px PNG.
	writePNG(os.Args[3], resize(src, 128, 128))

	// ico: the sizes Windows shells actually use.
	sizes := []int{256, 64, 48, 32, 16}
	var imgs [][]byte
	for _, s := range sizes {
		var buf bytes.Buffer
		if err := png.Encode(&buf, resize(src, s, s)); err != nil {
			fail(err)
		}
		imgs = append(imgs, buf.Bytes())
	}
	writeICO(os.Args[2], sizes, imgs)
	fmt.Println("wrote", os.Args[2], "and", os.Args[3])
}

func decode(path string) image.Image {
	f, err := os.Open(path)
	if err != nil {
		fail(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		fail(err)
	}
	return img
}

func writePNG(path string, img image.Image) {
	f, err := os.Create(path)
	if err != nil {
		fail(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		fail(err)
	}
}

// resize does a straightforward bilinear scale into an NRGBA of w×h.
func resize(src image.Image, w, h int) *image.NRGBA {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		fy := (float64(y)+0.5)*float64(sh)/float64(h) - 0.5
		y0 := int(math.Floor(fy))
		dy := fy - float64(y0)
		for x := 0; x < w; x++ {
			fx := (float64(x)+0.5)*float64(sw)/float64(w) - 0.5
			x0 := int(math.Floor(fx))
			dx := fx - float64(x0)
			r00, g00, b00, a00 := at(src, b, x0, y0)
			r10, g10, b10, a10 := at(src, b, x0+1, y0)
			r01, g01, b01, a01 := at(src, b, x0, y0+1)
			r11, g11, b11, a11 := at(src, b, x0+1, y0+1)
			lerp := func(c00, c10, c01, c11 float64) uint8 {
				top := c00*(1-dx) + c10*dx
				bot := c01*(1-dx) + c11*dx
				return uint8(math.Round(top*(1-dy) + bot*dy))
			}
			i := dst.PixOffset(x, y)
			dst.Pix[i+0] = lerp(r00, r10, r01, r11)
			dst.Pix[i+1] = lerp(g00, g10, g01, g11)
			dst.Pix[i+2] = lerp(b00, b10, b01, b11)
			dst.Pix[i+3] = lerp(a00, a10, a01, a11)
		}
	}
	return dst
}

func at(src image.Image, b image.Rectangle, x, y int) (r, g, bb, a float64) {
	if x < 0 {
		x = 0
	} else if x >= b.Dx() {
		x = b.Dx() - 1
	}
	if y < 0 {
		y = 0
	} else if y >= b.Dy() {
		y = b.Dy() - 1
	}
	cr, cg, cb, ca := src.At(b.Min.X+x, b.Min.Y+y).RGBA()
	return float64(cr >> 8), float64(cg >> 8), float64(cb >> 8), float64(ca >> 8)
}

// writeICO writes a PNG-payload .ico (Vista+). Each dir entry points at its PNG blob.
func writeICO(path string, sizes []int, imgs [][]byte) {
	var buf bytes.Buffer
	n := len(imgs)
	binary.Write(&buf, binary.LittleEndian, uint16(0)) // reserved
	binary.Write(&buf, binary.LittleEndian, uint16(1)) // type: icon
	binary.Write(&buf, binary.LittleEndian, uint16(n))
	offset := 6 + 16*n
	for i, s := range sizes {
		dim := byte(s)
		if s >= 256 {
			dim = 0 // 0 means 256
		}
		buf.WriteByte(dim) // width
		buf.WriteByte(dim) // height
		buf.WriteByte(0)   // palette
		buf.WriteByte(0)   // reserved
		binary.Write(&buf, binary.LittleEndian, uint16(1))              // planes
		binary.Write(&buf, binary.LittleEndian, uint16(32))             // bpp
		binary.Write(&buf, binary.LittleEndian, uint32(len(imgs[i])))   // bytesInRes
		binary.Write(&buf, binary.LittleEndian, uint32(offset))         // offset
		offset += len(imgs[i])
	}
	for _, im := range imgs {
		buf.Write(im)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		fail(err)
	}
}

func fail(err error) { fmt.Fprintln(os.Stderr, "mkico:", err); os.Exit(1) }
