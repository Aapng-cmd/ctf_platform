package avatar

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"testing"
)

func TestDecodeKeepsASmallPNG(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 32, 16))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	out, err := Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width != 32 || cfg.Height != 16 {
		t.Fatalf("size %dx%d", cfg.Width, cfg.Height)
	}
}

func TestDecodeRejectsOversize(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1025, 8))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 50}); err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(&buf); err == nil {
		t.Fatal("expected the crop limit")
	}
}
