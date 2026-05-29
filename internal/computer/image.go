package computer

import (
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

func ensureOutputPath(out string) error {
	if strings.TrimSpace(out) == "" {
		return RuntimeError{Code: "invalid_args", Message: "--out is required"}
	}
	dir := filepath.Dir(out)
	if dir == "." {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return RuntimeError{Code: "write_failed", Message: err.Error(), Retryable: true}
	}
	return nil
}

func writePNG(out string, img image.Image) error {
	file, err := os.Create(out)
	if err != nil {
		return RuntimeError{Code: "write_failed", Message: err.Error(), Retryable: true}
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		return RuntimeError{Code: "write_failed", Message: err.Error(), Retryable: true}
	}
	return nil
}

func cropImage(img image.Image, x1 int, y1 int, x2 int, y2 int) (image.Image, error) {
	if x2 <= x1 || y2 <= y1 {
		return nil, RuntimeError{Code: "invalid_args", Message: "zoom coordinates must satisfy x2 > x1 and y2 > y1"}
	}
	bounds := img.Bounds()
	rect := image.Rect(x1, y1, x2, y2).Intersect(bounds)
	if rect.Empty() {
		return nil, RuntimeError{Code: "invalid_args", Message: "zoom rectangle is outside the screenshot bounds"}
	}
	cropped := image.NewRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))
	draw.Draw(cropped, cropped.Bounds(), img, rect.Min, draw.Src)
	return cropped, nil
}
