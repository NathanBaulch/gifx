`gifx` is a drop-in replacement fork of the standard [image/gif](https://pkg.go.dev/image/gif) package with improved support for animated GIF files.

* Encode and decode images one at a time to reduce peak memory usage.
* Extract the first image from an animation without parsing the entire file. 
* Store and retrieve comment and plain text extension data.

Original code copyright 2013 The Go Authors. No changes have been made to the original `reader.go` and `writer.go` source files as forked from Go 1.18.

# Decode example

Extract each frame from an animated GIF and save them to disk. Each image is eligible for GC before the next frame is decoded.

```go
f, _ := os.Open("tmp.gif")
defer f.Close()
dec := gif.NewDecoder(f)
dec.ReadHeader()
i := 0
for {
    if blk, err := dec.ReadBlock(); err == io.EOF {
        break
    } else if frm, ok := blk.(*gif.Frame); ok {
        f, _ := os.Create(fmt.Sprintf("tmp%d.gif", i))
        gif.Encode(f, frm.Image, nil)
        f.Close()
        i++
    }
}
```

# Encode example

Create a random 100 frame greyscale animated GIF using only a single allocated `image.Paletted` struct.

```go
f, _ := os.Create("tmp.gif")
defer f.Close()
enc := gif.NewEncoder(f)
pal := make(color.Palette, 0x100)
for i := range pal {
    pal[i] = color.Gray{Y: uint8(i)}
}
enc.WriteHeader(image.Config{Width: 100, Height: 100, ColorModel: pal}, 0)
pm := image.NewPaletted(image.Rect(0, 0, 100, 100), pal)
for i := 0; i < 100; i++ {
    rand.Read(pm.Pix)
    enc.WriteFrame(&gif.Frame{Image: pm})
}
enc.WriteTrailer()
enc.Flush()
```
