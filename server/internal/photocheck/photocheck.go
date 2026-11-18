// Package photocheck does cheap image checks on attendance photos: how dark the
// photo is, and a perceptual hash for spotting reused photos. It does not
// detect, identify or match faces.
package photocheck

import (
	"bytes"
	"image"
	_ "image/jpeg" // decoders for the formats the client sends
	_ "image/png"
	"math/bits"
)

type Result struct {
	Decoded bool
	Luma    float64 // mean brightness, 0 to 255
	DHash   uint64
}

// Analyze returns zero Result{Decoded:false} for formats it cannot decode
// (for example WebP). Callers must treat that as "no opinion".
func Analyze(data []byte) Result {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return Result{}
	}
	b := img.Bounds()
	if b.Dx() < 9 || b.Dy() < 8 {
		return Result{}
	}
	return Result{Decoded: true, Luma: meanLuma(img), DHash: dhash(img)}
}

func luma(img image.Image, x, y int) float64 {
	r, g, b, _ := img.At(x, y).RGBA()
	return (0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(b)) / 257
}

func meanLuma(img image.Image) float64 {
	b := img.Bounds()
	const n = 32
	var sum float64
	for j := 0; j < n; j++ {
		for i := 0; i < n; i++ {
			sum += luma(img, b.Min.X+i*b.Dx()/n, b.Min.Y+j*b.Dy()/n)
		}
	}
	return sum / (n * n)
}

// blockMean averages the brightness of one cell of a cols x rows grid.
func blockMean(img image.Image, col, row, cols, rows int) float64 {
	b := img.Bounds()
	x0, x1 := b.Min.X+col*b.Dx()/cols, b.Min.X+(col+1)*b.Dx()/cols
	y0, y1 := b.Min.Y+row*b.Dy()/rows, b.Min.Y+(row+1)*b.Dy()/rows
	stepX, stepY := max(1, (x1-x0)/6), max(1, (y1-y0)/6)
	var sum float64
	var n int
	for y := y0; y < y1; y += stepY {
		for x := x0; x < x1; x += stepX {
			sum += luma(img, x, y)
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

// dhash compares each cell of a 9x8 grid with its right neighbour: 64 bits.
func dhash(img image.Image) uint64 {
	var grid [8][9]float64
	for r := 0; r < 8; r++ {
		for c := 0; c < 9; c++ {
			grid[r][c] = blockMean(img, c, r, 9, 8)
		}
	}
	var h uint64
	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			h <<= 1
			if grid[r][c] > grid[r][c+1] {
				h |= 1
			}
		}
	}
	return h
}

// Distance is the number of differing bits between two hashes.
func Distance(a, b uint64) int { return bits.OnesCount64(a ^ b) }

// Informative reports whether a hash carries enough structure to compare.
// Flat or near-flat images hash to almost all zeros or ones and would match
// each other, so callers must not draw conclusions from them.
func Informative(h uint64) bool {
	n := bits.OnesCount64(h)
	return n >= 8 && n <= 56
}
