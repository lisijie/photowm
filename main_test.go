package main

import (
    "image"
    "image/draw"
    "log"
    "github.com/golang/freetype"
    "image/color"
    "image/jpeg"
    "bufio"
    "os"
    "testing"
)

func TestDraw(t *testing.T) {
    dpi := 100.0
    dst := image.NewRGBA(image.Rect(0, 0, 1000, 1000))
    draw.Draw(dst, dst.Bounds(), &image.Uniform{color.Black}, image.ZP, draw.Src)

    f, err := getFont("hwxihei.ttf")

    if err != nil {
        log.Fatalln(err)
    }

    c := freetype.NewContext()
    c.SetDPI(dpi)
    c.SetFont(f)

    c.SetClip(dst.Bounds())
    c.SetDst(dst)
    c.SetSrc(image.White)

    y := 0
    for i := 10; i < 100; i++ {
        c.SetFontSize(float64(i))
        c.DrawString("中文abc123", freetype.Pt(10, y))
        y = y + i * 2 + 10
    }

    outFile, err := os.Create("out.jpg")
    b := bufio.NewWriter(outFile)
    jpeg.Encode(b, dst, &jpeg.Options{Quality: 90})
    b.Flush()
}
