package media

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func pngBytes(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetNRGBA(x, y, color.NRGBA{
				R: uint8((x * 17) % 255),
				G: uint8((y * 31) % 255),
				B: uint8((x + y) % 255),
				A: 255,
			})
		}
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestAvatarResizesAndCompresses(t *testing.T) {
	raw := pngBytes(t, 900, 700)
	result, err := Process(bytes.NewReader(raw), "avatar.png", AvatarPolicy())
	if err != nil {
		t.Fatal(err)
	}
	if result.Width > 512 || result.Height > 512 {
		t.Fatalf("avatar dimensions = %dx%d, want <= 512x512", result.Width, result.Height)
	}
	if int64(len(result.Data)) > TargetMaxBytes {
		t.Fatalf("avatar size = %d, want <= %d", len(result.Data), TargetMaxBytes)
	}
	if result.MIMEType != "image/jpeg" {
		t.Fatalf("opaque avatar MIME = %q, want image/jpeg", result.MIMEType)
	}
}

func TestRejectsOversizedDimensionsBeforeFullProcessing(t *testing.T) {
	raw := pngBytes(t, 2049, 8)
	_, err := Process(bytes.NewReader(raw), "too-wide.png", ReviewPolicy())
	if !errorsIs(err, ErrDimensionsTooLarge) {
		t.Fatalf("error = %v, want ErrDimensionsTooLarge", err)
	}
}

func TestRejectsInputOverTwoMegabytes(t *testing.T) {
	raw := bytes.Repeat([]byte{'x'}, int(MaxInputBytes)+1)
	_, err := Process(bytes.NewReader(raw), "large.jpg", ReviewPolicy())
	if !errorsIs(err, ErrTooLarge) {
		t.Fatalf("error = %v, want ErrTooLarge", err)
	}
}

func TestSVGIconSanitizedAndScaled(t *testing.T) {
	safe := `<svg xmlns="http://www.w3.org/2000/svg" width="1024" height="768" viewBox="0 0 1024 768"><rect width="1024" height="768" fill="#08f"/></svg>`
	result, err := Process(strings.NewReader(safe), "icon.svg", IconPolicy())
	if err != nil {
		t.Fatal(err)
	}
	if result.MIMEType != "image/svg+xml" {
		t.Fatalf("MIME = %q", result.MIMEType)
	}
	if result.Width > 512 || result.Height > 512 {
		t.Fatalf("SVG dimensions = %dx%d, want <= 512x512", result.Width, result.Height)
	}
	if !bytes.Contains(result.Data, []byte(`width="512"`)) {
		t.Fatalf("sanitized SVG does not contain resized width: %s", result.Data)
	}

	unsafe := `<svg xmlns="http://www.w3.org/2000/svg" width="128" height="128"><script>alert(1)</script></svg>`
	if _, err := Process(strings.NewReader(unsafe), "bad.svg", IconPolicy()); !errorsIs(err, ErrUnsafeSVG) {
		t.Fatalf("unsafe SVG error = %v, want ErrUnsafeSVG", err)
	}

	tooLarge := `<svg xmlns="http://www.w3.org/2000/svg" width="4096" height="512" viewBox="0 0 4096 512"><rect width="4096" height="512"/></svg>`
	if _, err := Process(strings.NewReader(tooLarge), "too-large.svg", IconPolicy()); !errorsIs(err, ErrDimensionsTooLarge) {
		t.Fatalf("oversized SVG error = %v, want ErrDimensionsTooLarge", err)
	}
}

func errorsIs(err, target error) bool {
	return err != nil && (err == target || strings.Contains(err.Error(), target.Error()))
}
