package ui

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"

	"fyne.io/fyne/v2"
)

// appIcon generates a simple 32×32 sand-coloured circle on a dark background
// and returns it as a Fyne static resource suitable for the system tray.
func appIcon() fyne.Resource {
	const size = 32
	img := image.NewNRGBA(image.Rect(0, 0, size, size))

	bg := color.NRGBA{R: 25, G: 20, B: 15, A: 255}
	fg := color.NRGBA{R: 210, G: 160, B: 75, A: 255}

	cx, cy, r := float64(size)/2, float64(size)/2, float64(size)/2-2

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x) + 0.5 - cx
			dy := float64(y) + 0.5 - cy
			if math.Sqrt(dx*dx+dy*dy) <= r {
				img.SetNRGBA(x, y, fg)
			} else {
				img.SetNRGBA(x, y, bg)
			}
		}
	}

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return fyne.NewStaticResource("appicon.png", buf.Bytes())
}
