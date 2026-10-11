// Package images turns an uploaded picture into what e-mail designs serve: a PNG, JPEG or GIF
// without metadata, at most 1,200 pixels wide.
package images

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"net/http"

	"golang.org/x/image/draw"
	"golang.org/x/image/webp"
)

const (
	MaxUploadBytes = 10 << 20
	MaxGIFBytes    = 5 << 20
	MaxWidth       = 1200
	MaxHeight      = 4000
	maxSide        = 8000
	maxPixels      = 40_000_000
	maxGIFPixels   = 50_000_000
	jpegQuality    = 85
)

var (
	ErrType     = errors.New("images: not a PNG, JPEG, GIF or WebP image")
	ErrTooLarge = errors.New("images: the image is too large")
	ErrGIFWide  = errors.New("images: GIFs must be at most 1,200 pixels wide")
)

type Result struct {
	Data          []byte
	ContentType   string
	Width, Height int
	SHA256        [32]byte
}

// Sniff names the type from the bytes: image/png, image/jpeg, image/gif, image/webp or "".
func Sniff(b []byte) string {
	switch ct := http.DetectContentType(b); ct {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
		return ct
	}
	return ""
}

// Normalize checks and re-encodes an upload.
func Normalize(b []byte) (Result, error) {
	ct := Sniff(b)
	if ct == "" {
		return Result{}, ErrType
	}
	cfg, err := decodeConfig(ct, b)
	if err != nil {
		return Result{}, ErrType
	}
	if cfg.Width < 1 || cfg.Height < 1 {
		return Result{}, ErrType
	}
	if cfg.Width > maxSide || cfg.Height > maxSide || cfg.Width*cfg.Height > maxPixels {
		return Result{}, ErrTooLarge
	}
	if ct == "image/gif" {
		return keepGIF(b, cfg)
	}
	var img image.Image
	switch ct {
	case "image/png":
		img, err = png.Decode(bytes.NewReader(b))
	case "image/jpeg":
		img, err = jpeg.Decode(bytes.NewReader(b))
	case "image/webp":
		img, err = webp.Decode(bytes.NewReader(b))
	}
	if err != nil {
		return Result{}, ErrType
	}
	if ct == "image/jpeg" {
		img = orient(img, jpegOrientation(b))
	}
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w > MaxWidth {
		h = max(1, h*MaxWidth/w)
		w = MaxWidth
	}
	if h > MaxHeight {
		return Result{}, ErrTooLarge
	}
	out := ct
	if ct == "image/webp" {
		out = "image/jpeg"
		if !opaque(img) {
			out = "image/png"
		}
	}
	var buf bytes.Buffer
	switch out {
	case "image/png":
		dst := image.NewNRGBA(image.Rect(0, 0, w, h))
		scale(dst, img)
		err = (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(&buf, dst)
	default:
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
		scale(dst, img)
		err = jpeg.Encode(&buf, dst, &jpeg.Options{Quality: jpegQuality})
	}
	if err != nil {
		return Result{}, err
	}
	return Result{Data: buf.Bytes(), ContentType: out, Width: w, Height: h, SHA256: sha256.Sum256(buf.Bytes())}, nil
}

func scale(dst draw.Image, src image.Image) {
	if dst.Bounds().Dx() == src.Bounds().Dx() && dst.Bounds().Dy() == src.Bounds().Dy() {
		draw.Draw(dst, dst.Bounds(), src, src.Bounds().Min, draw.Over)
		return
	}
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
}

func decodeConfig(ct string, b []byte) (image.Config, error) {
	r := bytes.NewReader(b)
	switch ct {
	case "image/png":
		return png.DecodeConfig(r)
	case "image/jpeg":
		return jpeg.DecodeConfig(r)
	case "image/gif":
		return gif.DecodeConfig(r)
	default:
		return webp.DecodeConfig(r)
	}
}

func opaque(img image.Image) bool {
	if o, ok := img.(interface{ Opaque() bool }); ok {
		return o.Opaque()
	}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a != 0xffff {
				return false
			}
		}
	}
	return true
}

// keepGIF stores a GIF as uploaded, animation included, once it decodes and fits.
func keepGIF(b []byte, cfg image.Config) (Result, error) {
	if cfg.Width > MaxWidth {
		return Result{}, ErrGIFWide
	}
	if cfg.Height > MaxHeight || len(b) > MaxGIFBytes {
		return Result{}, ErrTooLarge
	}
	px, err := gifPixels(b)
	if err != nil {
		return Result{}, ErrType
	}
	if px > maxGIFPixels {
		return Result{}, ErrTooLarge
	}
	if _, err := gif.DecodeAll(bytes.NewReader(b)); err != nil {
		return Result{}, ErrType
	}
	return Result{Data: b, ContentType: "image/gif", Width: cfg.Width, Height: cfg.Height, SHA256: sha256.Sum256(b)}, nil
}

// gifPixels adds up the frame areas from the image descriptors without decoding, so a small file
// of many large frames is refused before it is expanded in memory.
func gifPixels(b []byte) (int, error) {
	bad := errors.New("images: malformed GIF")
	if len(b) < 13 {
		return 0, bad
	}
	i := 13
	if b[10]&0x80 != 0 {
		i += 3 << (int(b[10]&7) + 1)
	}
	skipSub := func() error {
		for {
			if i >= len(b) {
				return bad
			}
			n := int(b[i])
			i++
			if n == 0 {
				return nil
			}
			i += n
		}
	}
	total := 0
	for i < len(b) {
		switch b[i] {
		case 0x3b:
			return total, nil
		case 0x21:
			i += 2
			if err := skipSub(); err != nil {
				return 0, err
			}
		case 0x2c:
			if i+10 > len(b) {
				return 0, bad
			}
			w, h := int(binary.LittleEndian.Uint16(b[i+5:])), int(binary.LittleEndian.Uint16(b[i+7:]))
			total += w * h
			flags := b[i+9]
			i += 10
			if flags&0x80 != 0 {
				i += 3 << (int(flags&7) + 1)
			}
			i++
			if err := skipSub(); err != nil {
				return 0, err
			}
		default:
			return 0, bad
		}
	}
	return total, nil
}
