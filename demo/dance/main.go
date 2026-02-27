package main

import (
	"bytes"
	_ "embed"
	"image/color"
	"time"

	"github.com/NathanBaulch/gifx"
)

//go:embed gopher.gif
var gifBytes []byte

func main() {
	gm, err := gif.NewDecoder(bytes.NewReader(gifBytes)).Decode()
	if err != nil {
		panic(err)
	}

	levels := []rune(" ░▒▓█")
	var loops int
	switch gm.LoopCount {
	case -1:
		loops = 1
	case 0:
		loops = -1
	default:
		loops = gm.LoopCount + 1
	}
	buf := make([][]rune, gm.Config.Height)
	for y := range buf {
		buf[y] = make([]rune, gm.Config.Width)
		for x := range buf[y] {
			buf[y][x] = ' '
		}
	}
	n := time.Now()

	print("\x1b[2J") // clear screen
	for loops != 0 {
		for i, im := range gm.Image {
			for y := im.Bounds().Min.Y; y < im.Bounds().Max.Y; y++ {
				for x := im.Bounds().Min.X; x < im.Bounds().Max.X; x++ {
					c := im.At(x, y)
					if _, _, _, a := c.RGBA(); a > 0 {
						buf[y][x] = levels[(color.GrayModel.Convert(c).(color.Gray).Y / 52)]
					}
				}
			}
			print("\x1b[H") // move top left
			for y := range buf {
				println(string(buf[y]))
			}
			time.Sleep(time.Duration(gm.Delay[i])*10*time.Millisecond - time.Since(n))
			n = time.Now()
			if gm.Disposal[i] == gif.DisposalBackground || i == len(gm.Image)-1 {
				l := levels[(color.GrayModel.Convert(im.Palette[gm.BackgroundIndex]).(color.Gray).Y / 52)]
				for y := im.Bounds().Min.Y; y < im.Bounds().Max.Y; y++ {
					for x := im.Bounds().Min.X; x < im.Bounds().Max.X; x++ {
						buf[y][x] = l
					}
				}
			}
		}
		if loops > 0 {
			loops--
		}
	}
}
