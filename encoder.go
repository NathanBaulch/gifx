package gif

import (
	"bufio"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/color/palette"
	"image/draw"
	"io"
	"math"
	"time"
	"unicode"
)

func NewEncoder(w io.Writer) *Encoder {
	w1, _ := w.(writer)
	if w1 == nil {
		w1 = bufio.NewWriter(w)
	}
	return &encoder{w: w1}
}

type Encoder = encoder

func (e *Encoder) Encode(g *GIF) error {
	if len(g.Image) == 0 {
		return errors.New("gif: must provide at least one image")
	}
	if len(g.Image) != len(g.Delay) {
		return errors.New("gif: mismatched image and delay lengths")
	}
	if g.Disposal != nil && len(g.Image) != len(g.Disposal) {
		return errors.New("gif: mismatched image and disposal lengths")
	}

	if g.Config == (image.Config{}) {
		p := g.Image[0].Bounds().Max
		g.Config.Width = p.X
		g.Config.Height = p.Y
	}

	if err := e.WriteHeader(g.Config, g.BackgroundIndex); err != nil {
		return err
	}
	if len(g.Image) > 1 && g.LoopCount >= 0 {
		if err := e.WriteApplicationNetscape(&ApplicationNetscape{LoopCount: g.LoopCount}); err != nil {
			return err
		}
	}

	for i, pm := range g.Image {
		disposal := uint8(0)
		if g.Disposal != nil {
			disposal = g.Disposal[i]
		}
		if err := e.WriteFrame(&Frame{Image: pm, DelayTime: time.Duration(g.Delay[i]) * 10 * time.Millisecond, DisposalMethod: disposal}); err != nil {
			return err
		}
	}
	if err := e.WriteTrailer(); err != nil {
		return err
	}
	return e.Flush()
}

type option func(*Options)

func WithNumColors(n int) option {
	return func(o *Options) {
		o.NumColors = n
	}
}

func WithQuantizer(q draw.Quantizer) option {
	return func(o *Options) {
		o.Quantizer = q
	}
}

func WithDrawer(d draw.Drawer) option {
	return func(o *Options) {
		o.Drawer = d
	}
}

func (e *Encoder) EncodeImage(m image.Image, o ...option) error {
	b := m.Bounds()
	if b.Dx() > math.MaxUint16 || b.Dy() > math.MaxUint16 {
		return errors.New("gif: image is too large to encode")
	}

	opts := &Options{}
	for _, o := range o {
		o(opts)
	}
	if opts.NumColors < 1 || 256 < opts.NumColors {
		opts.NumColors = 256
	}
	if opts.Drawer == nil {
		opts.Drawer = draw.FloydSteinberg
	}

	pm, _ := m.(*image.Paletted)
	if pm == nil {
		if cp, ok := m.ColorModel().(color.Palette); ok {
			pm = image.NewPaletted(b, cp)
			for y := b.Min.Y; y < b.Max.Y; y++ {
				for x := b.Min.X; x < b.Max.X; x++ {
					pm.Set(x, y, cp.Convert(m.At(x, y)))
				}
			}
		}
	}
	if pm == nil || len(pm.Palette) > opts.NumColors {
		pm = image.NewPaletted(b, palette.Plan9[:opts.NumColors])
		if opts.Quantizer != nil {
			pm.Palette = opts.Quantizer.Quantize(make(color.Palette, 0, opts.NumColors), m)
		}
		opts.Drawer.Draw(pm, b, m, b.Min)
	}

	if pm.Rect.Min != (image.Point{}) {
		dup := *pm
		dup.Rect = dup.Rect.Sub(dup.Rect.Min)
		pm = &dup
	}

	return e.Encode(&GIF{
		Image: []*image.Paletted{pm},
		Delay: []int{0},
		Config: image.Config{
			ColorModel: pm.Palette,
			Width:      b.Dx(),
			Height:     b.Dy(),
		},
	})
}

func (e *Encoder) WriteHeader(cfg image.Config, backgroundIndex byte) error {
	if cfg.ColorModel != nil {
		if _, ok := cfg.ColorModel.(color.Palette); !ok {
			return errors.New("gif: color model must be a color.Palette")
		}
	}

	e.g.Config = cfg
	e.g.BackgroundIndex = backgroundIndex
	e.writeHeader()
	return e.err
}

func (e *Encoder) WritePlainText(pt *PlainText) error {
	if err := validateStrings(pt.Strings); err != nil {
		return fmt.Errorf("gif: plain text %v", err)
	}

	if pt.DelayTime > 0 || pt.DisposalMethod != 0 {
		e.buf[0] = sExtension
		e.buf[1] = eGraphicControl
		e.buf[2] = 0x04
		e.buf[3] = pt.DisposalMethod << 2
		writeUint16(e.buf[4:6], uint16(pt.DelayTime/(10*time.Millisecond)))
		e.buf[6] = 0x00
		e.buf[7] = 0x00
		e.write(e.buf[:8])
	}

	e.buf[0] = sExtension
	e.buf[1] = eText
	e.buf[2] = 0x0c
	writeUint16(e.buf[3:5], pt.TextGridLeftPosition)
	writeUint16(e.buf[5:7], pt.TextGridTopPosition)
	writeUint16(e.buf[7:9], pt.TextGridWidth)
	writeUint16(e.buf[9:11], pt.TextGridHeight)
	e.buf[11] = pt.CharacterCellWidth
	e.buf[12] = pt.CharacterCellHeight
	e.buf[13] = pt.TextForegroundColorIndex
	e.buf[14] = pt.TextBackgroundColorIndex
	if e.write(e.buf[:15]); e.err != nil {
		return e.err
	}

	return e.writeStrings(pt.Strings)
}

func (e *Encoder) WriteComment(c *Comment) error {
	if err := validateStrings(c.Strings); err != nil {
		return fmt.Errorf("gif: comment %v", err)
	}

	e.buf[0] = sExtension
	e.buf[1] = eComment
	if e.write(e.buf[:2]); e.err != nil {
		return e.err
	}

	return e.writeStrings(c.Strings)
}

func validateStrings(strings []string) error {
	if len(strings) == 0 {
		return errors.New("must provide at least one string")
	}
	for _, str := range strings {
		if err := validateString(str); err != nil {
			return err
		}
	}

	return nil
}

func (e *Encoder) writeStrings(strings []string) error {
	for _, s := range strings {
		if e.writeByte(byte(len(s))); e.err != nil {
			return e.err
		}
		if _, e.err = io.WriteString(e.w, s); e.err != nil {
			return e.err
		}
	}

	e.writeByte(0x00)
	return e.err
}

func (e *Encoder) WriteApplicationNetscape(an *ApplicationNetscape) error {
	if err := validateSubBlocks(an.SubBlocks); err != nil {
		return fmt.Errorf("gif: application %v", err)
	}

	e.buf[0] = sExtension
	e.buf[1] = eApplication
	e.buf[2] = 0x0b
	if e.write(e.buf[:3]); e.err != nil {
		return e.err
	}
	if _, e.err = io.WriteString(e.w, "NETSCAPE2.0"); e.err != nil {
		return e.err
	}

	e.buf[0] = 0x03
	e.buf[1] = 0x01
	writeUint16(e.buf[2:4], uint16(an.LoopCount))
	if e.write(e.buf[:4]); e.err != nil {
		return e.err
	}

	return e.writeSubBlocks(an.SubBlocks)
}

func (e *Encoder) WriteUnknownApplication(ua *UnknownApplication) error {
	if err := validateString(ua.Identifier); err != nil {
		return fmt.Errorf("gif: application identifier %v", err)
	}
	if err := validateSubBlocks(ua.SubBlocks); err != nil {
		return fmt.Errorf("gif: application %v", err)
	}

	e.buf[0] = sExtension
	e.buf[1] = eApplication
	e.buf[2] = byte(len(ua.Identifier))
	if e.write(e.buf[:3]); e.err != nil {
		return e.err
	}
	if _, e.err = io.WriteString(e.w, ua.Identifier); e.err != nil {
		return e.err
	}

	return e.writeSubBlocks(ua.SubBlocks)
}

func validateString(str string) error {
	if len(str) > 0xff {
		return errors.New("string too long")
	}
	for c := range str {
		if c > unicode.MaxASCII {
			return errors.New("string must only contain ASCII characters")
		}
	}

	return nil
}

func (e *Encoder) WriteUnknownExtension(ue *UnknownExtension) error {
	if err := validateSubBlocks(ue.SubBlocks); err != nil {
		return fmt.Errorf("gif: extension %v", err)
	}

	e.buf[0] = sExtension
	e.buf[1] = ue.Label
	if e.write(e.buf[:2]); e.err != nil {
		return e.err
	}

	return e.writeSubBlocks(ue.SubBlocks)
}

func validateSubBlocks(subBlocks [][]byte) error {
	for _, sb := range subBlocks {
		if len(sb) > 0xff {
			return errors.New("sub-block too long")
		}
	}

	return nil
}

func (e *Encoder) writeSubBlocks(subBlocks [][]byte) error {
	for _, sb := range subBlocks {
		if e.writeByte(byte(len(sb))); e.err != nil {
			return e.err
		}
		if e.write(sb); e.err != nil {
			return e.err
		}
	}

	e.writeByte(0x00)
	return e.err
}

func (e *Encoder) WriteFrame(f *Frame) error {
	e.writeImageBlock(f.Image, int(f.DelayTime/(10*time.Millisecond)), f.DisposalMethod)
	return e.err
}

func (e *Encoder) WriteTrailer() error {
	e.writeByte(sTrailer)
	return e.err
}

func (e *Encoder) Flush() error {
	e.flush()
	return e.err
}
