package gif

import (
	"errors"
	"image"
)

// OptimizeAll takes a slice of images and replaces unchanged pixels with the transparent
// palette index. Each image is then cropped to omit unchanged regions where possible.
func OptimizeAll(pms []*image.Paletted, transparentIndex uint8) error {
	if len(pms) < 2 {
		return nil
	}

	o := NewOptimizer(transparentIndex)
	for i := range pms {
		if pm, err := o.Optimize(pms[i]); err != nil {
			return err
		} else {
			pms[i] = pm
		}
	}
	return nil
}

// NewOptimizer returns a new Optimizer with the given transparent palette index.
func NewOptimizer(transparentIndex uint8) *Optimizer {
	return &Optimizer{xs: []uint8{transparentIndex}}
}

type Optimizer struct {
	pm *image.Paletted
	xs []uint8
}

// Optimize compares the given image with the previous frame and replaces identical pixels
// with the transparent palette index. The smallest possible sub-image containing all
// changed pixels is returned.
// The first image passed cannot be optimized and is only used to initialize the internal
// image buffer.
func (o *Optimizer) Optimize(pm *image.Paletted) (*image.Paletted, error) {
	if o.pm == nil {
		o.pm = image.NewPaletted(pm.Rect, pm.Palette)
		copy(o.pm.Pix, pm.Pix)
		return pm, nil
	}

	if !pm.Rect.In(o.pm.Rect) {
		return nil, errors.New("image outside bounds")
	}

	var crop image.Rectangle
	if pm.Rect.Eq(o.pm.Rect) && len(pm.Pix) == len(o.pm.Pix) {
		// fast path that directly optimizes the raw pixels
		crop = o.optimizeByPix(pm)
	} else {
		crop = o.optimizeByLine(pm)
	}

	if crop.Empty() {
		crop = image.Rect(pm.Rect.Min.X, pm.Rect.Min.Y, pm.Rect.Min.X+1, pm.Rect.Min.Y+1)
	}
	if !pm.Rect.Eq(crop) {
		pm = pm.SubImage(crop).(*image.Paletted)
	}

	return pm, nil
}

func (o *Optimizer) optimizeByPix(pm *image.Paletted) image.Rectangle {
	var crop image.Rectangle
	var same bool
	var i0, x0, y0 int
	for i := 0; i <= len(pm.Pix); i++ {
		if i == 0 {
			same = pm.Pix[i] == o.pm.Pix[i] || pm.Pix[i] == o.xs[0]
		} else if i == len(pm.Pix) || (pm.Pix[i] == o.pm.Pix[i] || pm.Pix[i] == o.xs[0]) != same {
			x := i % pm.Stride
			y := i / pm.Stride
			if same {
				for len(o.xs) < i-i0 {
					o.xs = append(o.xs, o.xs...)
				}
				copy(pm.Pix[i0:i], o.xs[:i-i0])
			} else {
				copy(o.pm.Pix[i0:i], pm.Pix[i0:i])
				var r image.Rectangle
				if y > y0 {
					r = image.Rect(0, y0, pm.Stride, y+1)
				} else {
					r = image.Rect(x0, y0, x, y+1)
				}
				if crop.Empty() {
					crop = r
				} else {
					crop = crop.Union(r)
				}
			}
			same = !same
			i0, x0, y0 = i, x, y
		}
	}
	return crop
}

func (o *Optimizer) optimizeByLine(pm *image.Paletted) image.Rectangle {
	var crop image.Rectangle
	for y := pm.Rect.Min.Y; y < pm.Rect.Max.Y; y++ {
		x0 := pm.Rect.Min.X
		i, j := o.pm.PixOffset(x0, y), pm.PixOffset(x0, y)
		var same bool
		var i0, j0 int
		for x := x0; x <= pm.Rect.Max.X; x++ {
			if x == x0 {
				same = pm.Pix[j] == o.pm.Pix[i] || pm.Pix[j] == o.xs[0]
				i0, j0 = i, j
			} else {
				if x == pm.Rect.Max.X || (pm.Pix[j] == o.pm.Pix[i] || pm.Pix[j] == o.xs[0]) != same {
					if same {
						for len(o.xs) < j-j0 {
							o.xs = append(o.xs, o.xs...)
						}
						copy(pm.Pix[j0:j], o.xs[:j-j0])
					} else {
						copy(o.pm.Pix[i0:i], pm.Pix[j0:j])
						r := image.Rect(x0, y, x, y+1)
						if crop.Empty() {
							crop = r
						} else {
							crop = crop.Union(r)
						}
					}
					same = !same
					i0, j0, x0 = i, j, x
				}
			}
			i++
			j++
		}
	}
	return crop
}
