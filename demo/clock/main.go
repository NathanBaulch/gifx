package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"net/http"
	"time"

	"github.com/NathanBaulch/gifx"
)

var (
	//go:embed 7segment.png
	ssBytes      []byte
	ssImage      *image.Paletted
	ssTransIndex uint8
)

const (
	ssDigitWidth = 75
	ssColonWidth = 28
)

func main() {
	if im, err := png.Decode(bytes.NewReader(ssBytes)); err != nil {
		panic(err)
	} else {
		ssImage = im.(*image.Paletted)
	}

	found := false
	for i, c := range ssImage.Palette {
		if _, _, _, a := c.RGBA(); a == 0 {
			found = true
			ssTransIndex = uint8(i)
		}
	}
	if !found {
		if len(ssImage.Palette) == 0xff {
			panic("no space for transparent entry in image palette")
		}
		ssTransIndex = uint8(len(ssImage.Palette))
		ssImage.Palette = append(ssImage.Palette, color.Transparent)
	}

	http.HandleFunc("/time", handleTime)
	http.HandleFunc("/stopwatch", handleStopwatch)
	http.HandleFunc("/timer", handleTimer)
	if err := http.ListenAndServe(":8090", nil); err != nil {
		panic(err)
	}
}

func handleTime(w http.ResponseWriter, req *http.Request) {
	handle(w, req, func() time.Time { return time.Now() })
}

func handleStopwatch(w http.ResponseWriter, req *http.Request) {
	start := time.Now()
	zero := time.Time{}
	handle(w, req, func() time.Time { return zero.Add(time.Since(start)) })
}

func handleTimer(w http.ResponseWriter, req *http.Request) {
	d, err := time.ParseDuration(req.FormValue("d"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprintln(w, err.Error())
		return
	}

	rem := time.Time{}.Add(d)
	handle(w, req, func() time.Time {
		t := rem
		rem = rem.Add(-time.Second)
		return t
	})
}

func handle(w http.ResponseWriter, req *http.Request, fn func() time.Time) {
	w.Header().Set("content-type", "image/gif")
	w.Header().Set("cache-control", "no-store")

	rect := image.Rect(0, 0, 6*ssDigitWidth+2*ssColonWidth, ssImage.Rect.Max.Y)
	enc := gif.NewEncoder(&flushWriter{w})
	if err := enc.WriteHeader(image.Config{Width: rect.Max.X, Height: rect.Max.Y, ColorModel: ssImage.Palette}, 0); err != nil {
		panic(err)
	}

	pm := image.NewPaletted(rect, ssImage.Palette)
	ticker := time.NewTicker(time.Second)

	var prev []byte
	opt := gif.NewOptimizer(ssTransIndex)
loop:
	for {
		t := fn()
		if t.Before(time.Time{}) {
			break
		}
		this := []byte(t.Format("15:04:05"))
		x := 0
		var crop image.Rectangle
		for i, s := range this {
			width := ssDigitWidth
			if s == ':' {
				width = ssColonWidth
			}
			if len(prev) <= i || prev[i] != s {
				r := image.Rect(x, 0, x+width, ssImage.Rect.Max.Y)
				draw.Draw(pm, r, ssImage, image.Pt((int(s)-48)*ssDigitWidth, 0), draw.Over)
				crop = crop.Union(r)
			}
			x += width
		}
		pm, _ := opt.Optimize(pm.SubImage(crop).(*image.Paletted))
		if err := enc.WriteFrame(&gif.Frame{Image: pm, DelayTime: time.Second}); err != nil {
			panic(err)
		}
		if err := enc.Flush(); err != nil {
			panic(err)
		}

		select {
		case <-req.Context().Done():
			break loop
		case <-ticker.C:
			prev = this
		}
	}

	if err := enc.WriteTrailer(); err != nil {
		panic(err)
	}
	if err := enc.Flush(); err != nil {
		panic(err)
	}
}

type flushWriter struct {
	io.Writer
}

func (w *flushWriter) WriteByte(c byte) error {
	_, err := w.Write([]byte{c})
	return err
}

func (w *flushWriter) Flush() error {
	w.Writer.(http.Flusher).Flush()
	return nil
}
