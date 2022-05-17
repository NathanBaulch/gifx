package gif

import (
	"bytes"
	"image"
	"image/color"
	stdgif "image/gif"
	"io"
	"reflect"
	"testing"
	"time"

	"golang.org/x/image/colornames"
)

func TestFrame(t *testing.T) {
	f := &Frame{
		Image:          image.NewPaletted(image.Rect(0, 0, 1, 1), color.Palette{colornames.Black, colornames.White}),
		DelayTime:      90 * time.Millisecond,
		DisposalMethod: DisposalBackground,
	}
	data := doEncode(t, func(enc *Encoder) {
		if err := enc.WriteFrame(f); err != nil {
			t.Fatal("WriteFrame:", err)
		}
	})
	doDecode(t, func(dec *Decoder) {
		if blk, err := dec.ReadBlock(); err != nil {
			t.Fatal("ReadBlock:", err)
		} else if blk == f || !reflect.DeepEqual(blk, f) {
			t.Fatal("unexpected block: got:", blk, "want:", f)
		}
	}, data)
	if _, err := stdgif.DecodeAll(bytes.NewBuffer(data)); err != nil {
		t.Fatal("standard lib DecodeAll:", err)
	}
}

func TestPlainText(t *testing.T) {
	pt := &PlainText{
		TextGridLeftPosition:     1,
		TextGridTopPosition:      2,
		TextGridWidth:            3,
		TextGridHeight:           4,
		CharacterCellWidth:       5,
		CharacterCellHeight:      6,
		TextForegroundColorIndex: 7,
		TextBackgroundColorIndex: 8,
		Strings:                  []string{"hello"},
		DelayTime:                90 * time.Millisecond,
		DisposalMethod:           DisposalBackground,
	}
	data := doEncode(t, func(enc *Encoder) {
		if err := enc.WritePlainText(pt); err != nil {
			t.Fatal("WritePlainText:", err)
		}
	})
	doDecode(t, func(dec *Decoder) {
		if blk, err := dec.ReadBlock(); err != nil {
			t.Fatal("ReadBlock:", err)
		} else if blk == pt || !reflect.DeepEqual(blk, pt) {
			t.Fatal("unexpected block: got:", blk, "want:", pt)
		}
	}, data)
	if _, err := stdgif.DecodeAll(bytes.NewBuffer(data)); err != nil {
		t.Fatal("standard lib DecodeAll:", err)
	}
}

func TestComment(t *testing.T) {
	c := &Comment{Strings: []string{"hello", "world"}}
	data := doEncode(t, func(enc *Encoder) {
		if err := enc.WriteComment(c); err != nil {
			t.Fatal("WriteComment:", err)
		}
	})
	doDecode(t, func(dec *Decoder) {
		if blk, err := dec.ReadBlock(); err != nil {
			t.Fatal("ReadBlock:", err)
		} else if blk == c || !reflect.DeepEqual(blk, c) {
			t.Fatal("unexpected block: got:", blk, "want:", c)
		}
	}, data)
	if _, err := stdgif.DecodeAll(bytes.NewBuffer(data)); err != nil {
		t.Fatal("standard lib DecodeAll:", err)
	}
}

func TestApplicationNetscape(t *testing.T) {
	an := &ApplicationNetscape{LoopCount: 13, SubBlocks: [][]byte{[]byte("hello")}}
	data := doEncode(t, func(enc *Encoder) {
		if err := enc.WriteApplicationNetscape(an); err != nil {
			t.Fatal("WriteApplicationNetscape:", err)
		}
	})
	doDecode(t, func(dec *Decoder) {
		if blk, err := dec.ReadBlock(); err != nil {
			t.Fatal("ReadBlock:", err)
		} else if blk == an || !reflect.DeepEqual(blk, an) {
			t.Fatal("unexpected block: got:", blk, "want:", an)
		}
	}, data)
	if _, err := stdgif.DecodeAll(bytes.NewBuffer(data)); err != nil {
		t.Fatal("standard lib DecodeAll:", err)
	}
}

func TestUnknownApplication(t *testing.T) {
	ua := &UnknownApplication{Identifier: "foo", SubBlocks: [][]byte{[]byte("hello")}}
	data := doEncode(t, func(enc *Encoder) {
		if err := enc.WriteUnknownApplication(ua); err != nil {
			t.Fatal("WriteUnknownApplication:", err)
		}
	})
	doDecode(t, func(dec *Decoder) {
		if blk, err := dec.ReadBlock(); err != nil {
			t.Fatal("ReadBlock:", err)
		} else if blk == ua || !reflect.DeepEqual(blk, ua) {
			t.Fatal("unexpected block: got:", blk, "want:", ua)
		}
	}, data)
	if _, err := stdgif.DecodeAll(bytes.NewBuffer(data)); err != nil {
		t.Fatal("standard lib DecodeAll:", err)
	}
}

func TestUnknownExtension(t *testing.T) {
	ue := &UnknownExtension{Label: 42, SubBlocks: [][]byte{[]byte("hello")}}
	data := doEncode(t, func(enc *Encoder) {
		if err := enc.WriteUnknownExtension(ue); err != nil {
			t.Fatal("WriteUnknownExtension:", err)
		}
	})
	doDecode(t, func(dec *Decoder) {
		if blk, err := dec.ReadBlock(); err != nil {
			t.Fatal("ReadBlock:", err)
		} else if blk == ue || !reflect.DeepEqual(blk, ue) {
			t.Fatal("unexpected block: got:", blk, "want:", ue)
		}
	}, data)
}

func doEncode(t *testing.T, fn func(*Encoder)) []byte {
	buf := &bytes.Buffer{}
	enc := NewEncoder(buf)
	if err := enc.WriteHeader(image.Config{Width: 1, Height: 1}, 0); err != nil {
		t.Fatal("WriteHeader:", err)
	}
	fn(enc)
	if err := enc.WriteFrame(&Frame{Image: image.NewPaletted(image.Rect(0, 0, 1, 1), color.Palette{colornames.Black, colornames.White})}); err != nil {
		t.Fatal("WriteFrame:", err)
	}
	if err := enc.WriteTrailer(); err != nil {
		t.Fatal("WriteTrailer:", err)
	}
	if err := enc.Flush(); err != nil {
		t.Fatal("Flush:", err)
	}
	return buf.Bytes()
}

func doDecode(t *testing.T, fn func(*Decoder), data []byte) {
	dec := NewDecoder(bytes.NewReader(data))
	if _, err := dec.ReadHeader(); err != nil {
		t.Fatal("ReadHeader:", err)
	}
	fn(dec)
	if _, err := dec.ReadBlock(); err != nil {
		t.Fatal("ReadBlock:", err)
	}
	if blk, err := dec.ReadBlock(); err != io.EOF {
		t.Fatal("unexpected block: got:", blk, "want: EOF")
	}
}
