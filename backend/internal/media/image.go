package media

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	MaxInputBytes  int64 = 2 << 20
	TargetMaxBytes int64 = 1 << 20
	MaxSourceSide        = 2048
	AvatarMaxSide        = 512
)

var (
	ErrTooLarge           = errors.New("image upload too large")
	ErrDimensionsTooLarge = errors.New("image dimensions too large")
	ErrUnsupported        = errors.New("unsupported image type")
	ErrInvalidImage       = errors.New("invalid image")
	ErrCompressedTooLarge = errors.New("compressed image remains too large")
	ErrUnsafeSVG          = errors.New("unsafe svg")
)

type Policy struct {
	MaxOutputSide int
	AllowSVG      bool
	PreserveAlpha bool
	ForceJPEG     bool
}

type Result struct {
	Data             []byte
	MIMEType         string
	Extension        string
	Width            int
	Height           int
	SHA256           string
	OriginalFilename string
	OriginalBytes    int64
}

func AvatarPolicy() Policy {
	return Policy{MaxOutputSide: AvatarMaxSide, PreserveAlpha: true}
}

func IconPolicy() Policy {
	return Policy{MaxOutputSide: AvatarMaxSide, PreserveAlpha: true, AllowSVG: true}
}

func ReviewPolicy() Policy {
	return Policy{MaxOutputSide: MaxSourceSide, ForceJPEG: true}
}

func ScreenshotPolicy() Policy {
	return ReviewPolicy()
}

func Process(reader io.Reader, originalName string, policy Policy) (*Result, error) {
	if reader == nil {
		return nil, ErrInvalidImage
	}
	if policy.MaxOutputSide <= 0 || policy.MaxOutputSide > MaxSourceSide {
		policy.MaxOutputSide = MaxSourceSide
	}

	raw, err := io.ReadAll(io.LimitReader(reader, MaxInputBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > MaxInputBytes {
		return nil, ErrTooLarge
	}
	if len(raw) == 0 {
		return nil, ErrInvalidImage
	}

	name := filepath.Base(strings.TrimSpace(originalName))
	if strings.EqualFold(filepath.Ext(name), ".svg") || looksLikeSVG(raw) {
		if !policy.AllowSVG {
			return nil, ErrUnsupported
		}
		result, err := processSVG(raw, name, policy.MaxOutputSide)
		if err != nil {
			return nil, err
		}
		result.OriginalBytes = int64(len(raw))
		return result, nil
	}

	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, ErrUnsupported
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, ErrInvalidImage
	}
	if cfg.Width > MaxSourceSide || cfg.Height > MaxSourceSide {
		return nil, ErrDimensionsTooLarge
	}

	img, decodedFormat, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, ErrInvalidImage
	}
	if decodedFormat != "" {
		format = decodedFormat
	}
	if format != "jpeg" && format != "png" && format != "gif" {
		return nil, ErrUnsupported
	}
	_ = gif.GIF{} // keep image/gif registered for Decode without side effects being optimized away

	img = resizeToFit(img, policy.MaxOutputSide, policy.MaxOutputSide)
	var encoded []byte
	var mimeType, extension string

	hasAlpha := imageHasAlpha(img)
	switch {
	case policy.ForceJPEG:
		encoded, img = encodeJPEGToTarget(flattenToWhite(img))
		mimeType, extension = "image/jpeg", ".jpg"
	case policy.PreserveAlpha && hasAlpha:
		encoded, img = encodePNGToTarget(img)
		mimeType, extension = "image/png", ".png"
	default:
		encoded, img = encodeJPEGToTarget(flattenToWhite(img))
		mimeType, extension = "image/jpeg", ".jpg"
	}

	if len(encoded) == 0 || int64(len(encoded)) > TargetMaxBytes {
		return nil, ErrCompressedTooLarge
	}
	bounds := img.Bounds()
	sum := sha256.Sum256(encoded)
	return &Result{
		Data:             encoded,
		MIMEType:         mimeType,
		Extension:        extension,
		Width:            bounds.Dx(),
		Height:           bounds.Dy(),
		SHA256:           hex.EncodeToString(sum[:]),
		OriginalFilename: name,
		OriginalBytes:    int64(len(raw)),
	}, nil
}

func looksLikeSVG(raw []byte) bool {
	s := strings.TrimSpace(string(raw))
	return strings.HasPrefix(s, "<svg") ||
		strings.HasPrefix(s, "<?xml") && strings.Contains(strings.ToLower(s[:min(len(s), 2048)]), "<svg")
}

func encodeJPEGToTarget(img image.Image) ([]byte, image.Image) {
	current := img
	qualities := []int{90, 84, 78, 72, 66, 60, 54}
	for {
		for _, quality := range qualities {
			var out bytes.Buffer
			if err := jpeg.Encode(&out, current, &jpeg.Options{Quality: quality}); err == nil && int64(out.Len()) <= TargetMaxBytes {
				return out.Bytes(), current
			}
		}
		b := current.Bounds()
		if b.Dx() <= 320 && b.Dy() <= 320 {
			var out bytes.Buffer
			_ = jpeg.Encode(&out, current, &jpeg.Options{Quality: 48})
			return out.Bytes(), current
		}
		nextW := max(320, int(math.Round(float64(b.Dx())*0.88)))
		nextH := max(320, int(math.Round(float64(b.Dy())*0.88)))
		if nextW == b.Dx() && nextH == b.Dy() {
			return nil, current
		}
		current = resizeToFit(current, nextW, nextH)
	}
}

func encodePNGToTarget(img image.Image) ([]byte, image.Image) {
	current := img
	for {
		var out bytes.Buffer
		encoder := png.Encoder{CompressionLevel: png.BestCompression}
		if err := encoder.Encode(&out, current); err == nil && int64(out.Len()) <= TargetMaxBytes {
			return out.Bytes(), current
		}
		b := current.Bounds()
		if b.Dx() <= 256 && b.Dy() <= 256 {
			return out.Bytes(), current
		}
		nextW := max(256, int(math.Round(float64(b.Dx())*0.88)))
		nextH := max(256, int(math.Round(float64(b.Dy())*0.88)))
		current = resizeToFit(current, nextW, nextH)
	}
}

func flattenToWhite(src image.Image) image.Image {
	b := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			c := color.NRGBAModel.Convert(src.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA)
			if c.A == 255 {
				dst.SetNRGBA(x, y, c)
				continue
			}
			a := uint16(c.A)
			inv := uint16(255 - c.A)
			dst.SetNRGBA(x, y, color.NRGBA{
				R: uint8((uint16(c.R)*a + 255*inv) / 255),
				G: uint8((uint16(c.G)*a + 255*inv) / 255),
				B: uint8((uint16(c.B)*a + 255*inv) / 255),
				A: 255,
			})
		}
	}
	return dst
}

func imageHasAlpha(img image.Image) bool {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if a != 0xffff {
				return true
			}
		}
	}
	return false
}

func resizeToFit(src image.Image, maxW, maxH int) image.Image {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	if sw <= maxW && sh <= maxH {
		return normalizeNRGBA(src)
	}
	scale := math.Min(float64(maxW)/float64(sw), float64(maxH)/float64(sh))
	dw := max(1, int(math.Round(float64(sw)*scale)))
	dh := max(1, int(math.Round(float64(sh)*scale)))
	return resizeBilinear(src, dw, dh)
}

func normalizeNRGBA(src image.Image) image.Image {
	b := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			dst.Set(x, y, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

func resizeBilinear(src image.Image, dw, dh int) image.Image {
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, dw, dh))
	if dw == 1 || dh == 1 {
		for y := 0; y < dh; y++ {
			sy := sb.Min.Y + y*sh/max(1, dh)
			for x := 0; x < dw; x++ {
				sx := sb.Min.X + x*sw/max(1, dw)
				dst.Set(x, y, src.At(sx, sy))
			}
		}
		return dst
	}

	xScale := float64(sw-1) / float64(dw-1)
	yScale := float64(sh-1) / float64(dh-1)
	for y := 0; y < dh; y++ {
		fy := float64(y) * yScale
		y0 := int(math.Floor(fy))
		y1 := min(y0+1, sh-1)
		wy := fy - float64(y0)
		for x := 0; x < dw; x++ {
			fx := float64(x) * xScale
			x0 := int(math.Floor(fx))
			x1 := min(x0+1, sw-1)
			wx := fx - float64(x0)
			c00 := color.NRGBAModel.Convert(src.At(sb.Min.X+x0, sb.Min.Y+y0)).(color.NRGBA)
			c10 := color.NRGBAModel.Convert(src.At(sb.Min.X+x1, sb.Min.Y+y0)).(color.NRGBA)
			c01 := color.NRGBAModel.Convert(src.At(sb.Min.X+x0, sb.Min.Y+y1)).(color.NRGBA)
			c11 := color.NRGBAModel.Convert(src.At(sb.Min.X+x1, sb.Min.Y+y1)).(color.NRGBA)
			dst.SetNRGBA(x, y, color.NRGBA{
				R: bilerp(c00.R, c10.R, c01.R, c11.R, wx, wy),
				G: bilerp(c00.G, c10.G, c01.G, c11.G, wx, wy),
				B: bilerp(c00.B, c10.B, c01.B, c11.B, wx, wy),
				A: bilerp(c00.A, c10.A, c01.A, c11.A, wx, wy),
			})
		}
	}
	return dst
}

func bilerp(c00, c10, c01, c11 uint8, wx, wy float64) uint8 {
	top := float64(c00)*(1-wx) + float64(c10)*wx
	bottom := float64(c01)*(1-wx) + float64(c11)*wx
	value := top*(1-wy) + bottom*wy
	if value < 0 {
		return 0
	}
	if value > 255 {
		return 255
	}
	return uint8(math.Round(value))
}

func processSVG(raw []byte, originalName string, maxOutputSide int) (*Result, error) {
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	var out bytes.Buffer
	encoder := xml.NewEncoder(&out)
	depth := 0
	foundSVG := false
	width, height := 0, 0

	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, ErrInvalidImage
		}

		switch t := token.(type) {
		case xml.Directive:
			return nil, ErrUnsafeSVG
		case xml.Comment:
			continue
		case xml.StartElement:
			depth++
			local := strings.ToLower(t.Name.Local)
			if blockedSVGElement(local) {
				return nil, ErrUnsafeSVG
			}
			if depth == 1 {
				if local != "svg" {
					return nil, ErrInvalidImage
				}
				foundSVG = true
				w, h, err := svgDimensions(t.Attr)
				if err != nil {
					return nil, err
				}
				if w > MaxSourceSide || h > MaxSourceSide {
					return nil, ErrDimensionsTooLarge
				}
				width, height = fitDimensions(w, h, maxOutputSide)
				t.Attr = replaceSVGDimensions(t.Attr, width, height)
			}
			for _, attr := range t.Attr {
				name := strings.ToLower(attr.Name.Local)
				value := strings.ToLower(strings.TrimSpace(attr.Value))
				if strings.HasPrefix(name, "on") {
					return nil, ErrUnsafeSVG
				}
				if name == "href" {
					if value != "" && !strings.HasPrefix(value, "#") {
						return nil, ErrUnsafeSVG
					}
				}
				if name == "style" && (strings.Contains(value, "url(") || strings.Contains(value, "javascript:") || strings.Contains(value, "expression(")) {
					return nil, ErrUnsafeSVG
				}
			}
			if err := encoder.EncodeToken(t); err != nil {
				return nil, err
			}
		case xml.EndElement:
			if err := encoder.EncodeToken(t); err != nil {
				return nil, err
			}
			depth--
		default:
			if err := encoder.EncodeToken(token); err != nil {
				return nil, err
			}
		}
	}
	if !foundSVG || depth != 0 {
		return nil, ErrInvalidImage
	}
	if err := encoder.Flush(); err != nil {
		return nil, err
	}
	if out.Len() == 0 || int64(out.Len()) > TargetMaxBytes {
		return nil, ErrCompressedTooLarge
	}
	data := out.Bytes()
	sum := sha256.Sum256(data)
	return &Result{
		Data:             data,
		MIMEType:         "image/svg+xml",
		Extension:        ".svg",
		Width:            width,
		Height:           height,
		SHA256:           hex.EncodeToString(sum[:]),
		OriginalFilename: originalName,
	}, nil
}

func blockedSVGElement(name string) bool {
	switch name {
	case "script", "foreignobject", "iframe", "object", "embed", "audio", "video", "image", "style":
		return true
	default:
		return false
	}
}

func svgDimensions(attrs []xml.Attr) (int, int, error) {
	var width, height int
	var viewBox string
	for _, attr := range attrs {
		switch strings.ToLower(attr.Name.Local) {
		case "width":
			width = parseSVGNumber(attr.Value)
		case "height":
			height = parseSVGNumber(attr.Value)
		case "viewbox":
			viewBox = attr.Value
		}
	}
	if (width <= 0 || height <= 0) && viewBox != "" {
		fields := strings.Fields(strings.ReplaceAll(viewBox, ",", " "))
		if len(fields) == 4 {
			vw, e1 := strconv.ParseFloat(fields[2], 64)
			vh, e2 := strconv.ParseFloat(fields[3], 64)
			if e1 == nil && e2 == nil && vw > 0 && vh > 0 {
				width = int(math.Ceil(vw))
				height = int(math.Ceil(vh))
			}
		}
	}
	if width <= 0 || height <= 0 {
		return 0, 0, ErrInvalidImage
	}
	return width, height, nil
}

func parseSVGNumber(value string) int {
	v := strings.TrimSpace(strings.ToLower(value))
	v = strings.TrimSuffix(v, "px")
	if strings.ContainsAny(v, "%emremptcmmin") {
		return 0
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || f <= 0 {
		return 0
	}
	return int(math.Ceil(f))
}

func fitDimensions(width, height, maxSide int) (int, int) {
	if width <= maxSide && height <= maxSide {
		return width, height
	}
	scale := math.Min(float64(maxSide)/float64(width), float64(maxSide)/float64(height))
	return max(1, int(math.Round(float64(width)*scale))), max(1, int(math.Round(float64(height)*scale)))
}

func replaceSVGDimensions(attrs []xml.Attr, width, height int) []xml.Attr {
	out := make([]xml.Attr, 0, len(attrs)+2)
	for _, attr := range attrs {
		name := strings.ToLower(attr.Name.Local)
		if name == "width" || name == "height" {
			continue
		}
		out = append(out, attr)
	}
	out = append(out,
		xml.Attr{Name: xml.Name{Local: "width"}, Value: strconv.Itoa(width)},
		xml.Attr{Name: xml.Name{Local: "height"}, Value: strconv.Itoa(height)},
	)
	return out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func DescribeError(err error) string {
	switch {
	case errors.Is(err, ErrTooLarge):
		return fmt.Sprintf("image exceeds %d bytes", MaxInputBytes)
	case errors.Is(err, ErrDimensionsTooLarge):
		return fmt.Sprintf("image exceeds %dx%d pixels", MaxSourceSide, MaxSourceSide)
	case errors.Is(err, ErrCompressedTooLarge):
		return fmt.Sprintf("image could not be compressed below %d bytes", TargetMaxBytes)
	default:
		return err.Error()
	}
}
