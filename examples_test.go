package gif_test

import (
	"fmt"
	"image"
	"image/color"
	"io"
	"math/rand"
	"os"

	"github.com/NathanBaulch/gifx"
)

func ExampleNewEncoder() {
	f, _ := os.Create("tmp.gif")
	defer f.Close()

	enc := gif.NewEncoder(f)
	pal := make(color.Palette, 0x100)
	for i := range pal {
		pal[i] = color.Gray{Y: uint8(i)}
	}

	_ = enc.WriteHeader(image.Config{Width: 100, Height: 100, ColorModel: pal}, 0)

	pm := image.NewPaletted(image.Rect(0, 0, 100, 100), pal)
	for i := 0; i < 100; i++ {
		rand.Read(pm.Pix)
		_ = enc.WriteFrame(&gif.Frame{Image: pm})
	}

	_ = enc.WriteTrailer()
	_ = enc.Flush()
}

func ExampleNewDecoder() {
	f, _ := os.Open("tmp.gif")
	defer f.Close()

	dec := gif.NewDecoder(f)
	_, _ = dec.ReadHeader()

	i := 0
	for {
		if blk, err := dec.ReadBlock(); err == io.EOF {
			break
		} else if frm, ok := blk.(*gif.Frame); ok {
			f, _ := os.Create(fmt.Sprintf("tmp%d.gif", i))
			_ = gif.Encode(f, frm.Image, nil)
			_ = f.Close()
			i++
		}
	}
}
