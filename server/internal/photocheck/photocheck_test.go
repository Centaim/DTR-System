package photocheck

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

func encode(t *testing.T, f func(x, y int) uint8) []byte {
	t.Helper()
	img := image.NewGray(image.Rect(0, 0, 160, 120))
	for y := 0; y < 120; y++ {
		for x := 0; x < 160; x++ {
			img.SetGray(x, y, color.Gray{Y: f(x, y)})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestDarkAndBrightLuma(t *testing.T) {
	dark := Analyze(encode(t, func(x, y int) uint8 { return 12 }))
	bright := Analyze(encode(t, func(x, y int) uint8 { return 190 }))
	if !dark.Decoded || dark.Luma > 30 {
		t.Errorf("dark photo luma %v", dark.Luma)
	}
	if bright.Luma < 150 {
		t.Errorf("bright photo luma %v", bright.Luma)
	}
}

func TestSameImageSameHashDifferentImageFar(t *testing.T) {
	ramp := func(x, y int) uint8 { return uint8(x * 255 / 159) }
	flip := func(x, y int) uint8 { return uint8(255 - x*255/159) }
	a, b, c := Analyze(encode(t, ramp)), Analyze(encode(t, ramp)), Analyze(encode(t, flip))
	if Distance(a.DHash, b.DHash) != 0 {
		t.Errorf("identical images differ by %d bits", Distance(a.DHash, b.DHash))
	}
	if d := Distance(a.DHash, c.DHash); d < 40 {
		t.Errorf("opposite gradients only %d bits apart", d)
	}
}

func TestSmallChangeStaysClose(t *testing.T) {
	base := func(x, y int) uint8 { return uint8((x*3 + y) % 200) }
	noisy := func(x, y int) uint8 {
		v := base(x, y)
		if (x+y)%17 == 0 && v < 250 {
			return v + 5
		}
		return v
	}
	if d := Distance(Analyze(encode(t, base)).DHash, Analyze(encode(t, noisy)).DHash); d > 6 {
		t.Errorf("slight noise moved the hash by %d bits", d)
	}
}

func TestUndecodableGivesNoOpinion(t *testing.T) {
	if Analyze([]byte("not an image at all")).Decoded {
		t.Error("garbage decoded")
	}
}

func TestFlatImagesAreNotInformative(t *testing.T) {
	flat := Analyze(encode(t, func(x, y int) uint8 { return 16 }))
	vertical := Analyze(encode(t, func(x, y int) uint8 { return uint8(y * 2) }))
	textured := Analyze(encode(t, func(x, y int) uint8 { return uint8((x*37 + y*91) % 251) }))
	if Informative(flat.DHash) || Informative(vertical.DHash) {
		t.Errorf("flat hash %x or vertical hash %x treated as informative", flat.DHash, vertical.DHash)
	}
	if !Informative(textured.DHash) {
		t.Errorf("textured hash %x not informative", textured.DHash)
	}
}
