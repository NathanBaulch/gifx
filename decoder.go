package gif

import (
	"bufio"
	"fmt"
	"image"
	"io"
)

type (
	Header struct {
		Version         string       // GIF version, either GIF87a or GIF89a.
		Config          image.Config // Global color table (palette), width and height.
		BackgroundIndex byte         // Background index in the global color table, for use with the DisposalBackground disposal method.
	}
	PlainText struct {
		TextGridLeftPosition     uint16
		TextGridTopPosition      uint16
		TextGridWidth            uint16
		TextGridHeight           uint16
		CharacterCellWidth       byte
		CharacterCellHeight      byte
		TextForegroundColorIndex byte
		TextBackgroundColorIndex byte
		Strings                  []string // Text, up to 255 ASCII characters per string.
		DelayTime                uint16   // Delay time in 100ths of a second.
		DisposalMethod           byte     // Disposal method, one of DisposalNone, DisposalBackground, DisposalPrevious.
	}
	Comment struct {
		Strings []string // Comments, up to 255 ASCII characters per string.
	}
	ApplicationNetscape struct {
		LoopCount int      // Number of times an animation will be restarted during display.
		SubBlocks [][]byte // Optional sub-blocks of arbitrary data.
	}
	UnknownApplication struct {
		Identifier string   // Identifier string of the application.
		SubBlocks  [][]byte // Optional sub-blocks of arbitrary data.
	}
	UnknownExtension struct {
		Label     byte
		SubBlocks [][]byte // Optional sub-blocks of arbitrary data.
	}
	Frame struct {
		Image          *image.Paletted // Paletted image.
		DelayTime      uint16          // Delay time in 100ths of a second.
		DisposalMethod byte            // Disposal method, one of DisposalNone, DisposalBackground, DisposalPrevious.
	}
)

func NewDecoder(r io.Reader) *Decoder {
	r1, _ := r.(reader)
	if r1 == nil {
		r1 = bufio.NewReader(r)
	}
	return &decoder{r: r1}
}

type Decoder = decoder

func (d *Decoder) Decode() (*GIF, error) {
	d.loopCount = -1
	hdr, err := d.ReadHeader()
	if err != nil {
		return nil, err
	}

	g := &GIF{Config: hdr.Config, BackgroundIndex: hdr.BackgroundIndex, LoopCount: -1}
	for {
		if b, err := d.ReadBlock(); err != nil {
			if err != io.EOF {
				return nil, err
			}
			if len(g.Image) == 0 {
				return nil, fmt.Errorf("gif: missing image data")
			}
			return g, nil
		} else {
			switch b := b.(type) {
			case *ApplicationNetscape:
				g.LoopCount = b.LoopCount
			case *Frame:
				g.Image = append(g.Image, b.Image)
				g.Delay = append(g.Delay, int(b.DelayTime))
				g.Disposal = append(g.Disposal, b.DisposalMethod)
			}
		}
	}
}

func (d *Decoder) DecodeFirst() (image.Image, error) {
	if _, err := d.ReadHeader(); err != nil {
		return nil, err
	}

	for {
		if b, err := d.ReadBlock(); err != nil {
			if err != io.EOF {
				return nil, err
			}
			return nil, fmt.Errorf("gif: missing image data")
		} else {
			switch b := b.(type) {
			case *Frame:
				return b.Image, nil
			}
		}
	}
}

func (d *Decoder) DecodeConfig() (image.Config, error) {
	if hdr, err := d.ReadHeader(); err != nil {
		return image.Config{}, err
	} else {
		return hdr.Config, nil
	}
}

func (d *Decoder) ReadHeader() (*Header, error) {
	if err := d.readHeaderAndScreenDescriptor(); err != nil {
		return nil, err
	}
	return &Header{
		Version: d.vers,
		Config: image.Config{
			ColorModel: d.globalColorTable,
			Width:      d.width,
			Height:     d.height,
		},
		BackgroundIndex: d.backgroundIndex,
	}, nil
}

func (d *Decoder) ReadBlock() (any, error) {
	for {
		c, err := readByte(d.r)
		if err != nil {
			return nil, fmt.Errorf("gif: reading block: %v", err)
		}

		switch c {
		case sExtension:
			if e, err := d.readExtension_(); e != nil || err != nil {
				return e, err
			}

		case sImageDescriptor:
			if err = d.readImageDescriptor(false); err != nil {
				return nil, err
			}
			f := &Frame{
				Image:          d.image[0],
				DelayTime:      uint16(d.delay[0]),
				DisposalMethod: d.disposal[0],
			}
			d.image = d.image[:0]
			d.delay = d.delay[:0]
			d.disposal = d.disposal[:0]
			return f, nil

		case sTrailer:
			return nil, io.EOF

		default:
			return nil, fmt.Errorf("gif: unknown block type: 0x%.2x", c)
		}
	}
}

func (d *Decoder) readExtension_() (any, error) {
	label, err := readByte(d.r)
	if err != nil {
		return nil, fmt.Errorf("gif: reading extension: %v", err)
	}
	switch label {
	case eText:
		return d.readPlainText()

	case eGraphicControl:
		return nil, d.readGraphicControl()

	case eComment:
		return d.readComment()

	case eApplication:
		return d.readApplication()

	default:
		return d.readUnknownExtension(label)
	}
}

func (d *Decoder) readPlainText() (*PlainText, error) {
	if err := readFull(d.r, d.tmp[:13]); err != nil {
		return nil, fmt.Errorf("gif: reading plain text extension: %v", err)
	}
	if d.tmp[0] != 0x0c {
		return nil, fmt.Errorf("gif: invalid plain text extension block size: %d", d.tmp[0])
	}

	pt := &PlainText{
		TextGridLeftPosition:     readUint16(d.tmp[1:3]),
		TextGridTopPosition:      readUint16(d.tmp[3:5]),
		TextGridWidth:            readUint16(d.tmp[5:7]),
		TextGridHeight:           readUint16(d.tmp[7:9]),
		CharacterCellWidth:       d.tmp[9],
		CharacterCellHeight:      d.tmp[10],
		TextForegroundColorIndex: d.tmp[11],
		TextBackgroundColorIndex: d.tmp[12],
		DelayTime:                uint16(d.delayTime),
		DisposalMethod:           d.disposalMethod,
	}

	if strings, err := d.readStrings(); err != nil {
		return nil, fmt.Errorf("gif: reading plain text extension: %v", err)
	} else {
		pt.Strings = strings
	}

	d.delayTime = 0
	d.disposalMethod = 0
	d.hasTransparentIndex = false
	return pt, nil
}

func (d *Decoder) readComment() (*Comment, error) {
	if strings, err := d.readStrings(); err != nil {
		return nil, fmt.Errorf("gif: reading comment extension: %v", err)
	} else {
		return &Comment{Strings: strings}, nil
	}
}

func (d *Decoder) readStrings() ([]string, error) {
	var strings []string
	for {
		if n, err := d.readBlock(); err != nil {
			return nil, err
		} else if n == 0 {
			return strings, nil
		} else {
			strings = append(strings, string(d.tmp[:n]))
		}
	}
}

func (d *Decoder) readApplication() (any, error) {
	if b, err := readByte(d.r); err != nil {
		return nil, fmt.Errorf("gif: reading application extension: %v", err)
	} else if err := readFull(d.r, d.tmp[:int(b)]); err != nil {
		return nil, fmt.Errorf("gif: reading application extension: %v", err)
	} else if id := string(d.tmp[:int(b)]); id == "NETSCAPE2.0" {
		if n, err := d.readBlock(); err != nil {
			return nil, fmt.Errorf("gif: reading application extension: %v", err)
		} else if n == 3 && d.tmp[0] == 1 {
			d.loopCount = int(readUint16(d.tmp[1:]))
		}
		if sb, err := d.readSubBlocks(); err != nil {
			return nil, fmt.Errorf("gif: reading application extension: %v", err)
		} else {
			return &ApplicationNetscape{LoopCount: d.loopCount, SubBlocks: sb}, nil
		}
	} else {
		if sb, err := d.readSubBlocks(); err != nil {
			return nil, fmt.Errorf("gif: reading application extension: %v", err)
		} else {
			return &UnknownApplication{Identifier: id, SubBlocks: sb}, nil
		}
	}
}

func (d *Decoder) readUnknownExtension(label byte) (*UnknownExtension, error) {
	if sb, err := d.readSubBlocks(); err != nil {
		return nil, fmt.Errorf("gif: reading unknown extension: %v", err)
	} else {
		return &UnknownExtension{Label: label, SubBlocks: sb}, nil
	}
}

func (d *Decoder) readSubBlocks() ([][]byte, error) {
	var sb [][]byte
	for {
		if n, err := d.readBlock(); err != nil {
			return nil, err
		} else if n == 0 {
			return sb, nil
		} else {
			data := make([]byte, n)
			copy(data, d.tmp[:n])
			sb = append(sb, data)
		}
	}
}

func readUint16(b []uint8) uint16 {
	return uint16(b[0]) | uint16(b[1])<<8
}
