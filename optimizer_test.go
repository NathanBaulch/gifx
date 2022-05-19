package gif

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

func TestOptimize(t *testing.T) {
	pms := parseFrames(`
		000 000 111 111 010 000 001 222 1-- --- --- --- ---
		000 000 111 101 101 000 100 222 --- -1- --- -1- -11
		000 000 111 111 010 000 000 222 --- --- --1 --- -11`)
	wants := parseFrames(`
		000 2-- 111 --- 020 202 221 2-- 1-- --- --- --- ---
		000 --- 111 -0- 222 020 122 --- --- -1- --- -2- -21
		000 --- 111 --- 020 202 --- --- --- --- --1 --- -12`)
	if err := OptimizeAll(pms, 2); err != nil {
		t.Fatal("OptimizeAll:", err)
	}
	for i, want := range wants {
		pm := pms[i]
		if pm.Rect != want.Rect {
			t.Fatal("unexpected frame", i, "size: got:", pm.Rect, "want:", want.Rect)
		}

		for y := want.Rect.Min.Y; y < want.Rect.Max.Y; y++ {
			for x := want.Rect.Min.X; x < want.Rect.Max.X; x++ {
				if g, w := pm.Pix[pm.PixOffset(x, y)], want.Pix[want.PixOffset(x, y)]; g != w {
					t.Fatal("unexpected frame", i, image.Pt(x, y), "pixel: got:", g, "want:", w)
				}
			}
		}
	}
}

func parseFrames(str string) []*image.Paletted {
	str = strings.TrimLeft(str, "\n")
	rect := image.Rect(0, 0, 3, 3)
	pms := make([]*image.Paletted, len(str)/12)
	pal := color.Palette([]color.Color{color.Black, color.White, color.Transparent})
	for i := range pms {
		pms[i] = image.NewPaletted(rect, pal)
	}

	crops := make([]image.Rectangle, len(pms))
	for y, line := range strings.Split(str, "\n") {
		for i, c := range strings.TrimSpace(line) {
			if c == ' ' || c == '-' {
				continue
			}
			x := i % 4
			j := i / 4
			pms[j].SetColorIndex(x, y, uint8(c-48))
			r := image.Rect(x, y, x+1, y+1)
			if crop := crops[j]; crop.Empty() {
				crops[j] = r
			} else {
				crops[j] = crop.Union(r)
			}
		}
	}

	for i := range pms {
		pms[i] = pms[i].SubImage(crops[i]).(*image.Paletted)
	}
	return pms
}
