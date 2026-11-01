package avatar

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"io"
)

const (
	MaxBytes  = 8 << 20
	MaxSide   = 1024
	maxDecode = 8000
)

// Decode reads a png or jpg and returns a re-encoded png no larger than 1024
// on either side. A bigger picture is refused so the browser crop is required.
func Decode(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || len(data) > MaxBytes {
		return nil, errors.New("picture is too large")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "png" && format != "jpeg") {
		return nil, errors.New("picture must be a png or jpg")
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > maxDecode || cfg.Height > maxDecode {
		return nil, errors.New("picture is too large")
	}
	if cfg.Width > MaxSide || cfg.Height > MaxSide {
		return nil, errors.New("crop the picture to 1024×1024")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, errors.New("picture must be a png or jpg")
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
