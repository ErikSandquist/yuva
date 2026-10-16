package images

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func pngOf(t *testing.T, w, h int, alpha bool) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			a := uint8(255)
			if alpha && x < w/2 {
				a = 0
			}
			img.SetNRGBA(x, y, color.NRGBA{uint8(x), uint8(y), 200, a})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// exifJPEG is a w×h JPEG whose APP1 holds an orientation and a GPS position.
func exifJPEG(t *testing.T, w, h int, orientation uint16) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := uint8(0)
			if x < w/4 {
				c = 255
			}
			img.Set(x, y, color.RGBA{c, 0, 0, 255})
		}
	}
	var j bytes.Buffer
	if err := jpeg.Encode(&j, img, nil); err != nil {
		t.Fatal(err)
	}
	le := binary.LittleEndian
	tiff := []byte("II*\x00\x08\x00\x00\x00")
	ifd := make([]byte, 2+2*12+4)
	le.PutUint16(ifd, 2)
	le.PutUint16(ifd[2:], 0x0112)
	le.PutUint16(ifd[4:], 3)
	le.PutUint32(ifd[6:], 1)
	le.PutUint16(ifd[10:], orientation)
	gpsOff := uint32(8 + len(ifd))
	le.PutUint16(ifd[14:], 0x8825)
	le.PutUint16(ifd[16:], 4)
	le.PutUint32(ifd[18:], 1)
	le.PutUint32(ifd[22:], gpsOff)
	gps := make([]byte, 2+12+4)
	le.PutUint16(gps, 1)
	le.PutUint16(gps[2:], 0x0001)
	le.PutUint16(gps[4:], 2)
	le.PutUint32(gps[6:], 2)
	copy(gps[10:], "N\x00")
	app := append(append(append([]byte("Exif\x00\x00"), tiff...), ifd...), gps...)
	app = append(app, []byte("GPSLatitude=41.0082")...)
	seg := []byte{0xff, 0xe1, 0, 0}
	binary.BigEndian.PutUint16(seg[2:], uint16(len(app)+2))
	jb := j.Bytes()
	return append(append(append([]byte{}, jb[:2]...), append(seg, app...)...), jb[2:]...)
}

func TestJPEGLosesGPSAndStandsUpright(t *testing.T) {
	in := exifJPEG(t, 80, 40, 6)
	if jpegOrientation(in) != 6 || !bytes.Contains(in, []byte("GPSLatitude")) {
		t.Fatal("the fixture has no EXIF")
	}
	r, err := Normalize(in)
	if err != nil {
		t.Fatal(err)
	}
	if r.ContentType != "image/jpeg" || r.Width != 40 || r.Height != 80 {
		t.Errorf("%s %dx%d, want image/jpeg 40x80", r.ContentType, r.Width, r.Height)
	}
	if bytes.Contains(r.Data, []byte("Exif")) || bytes.Contains(r.Data, []byte("GPS")) {
		t.Error("metadata survived")
	}
	img, err := jpeg.Decode(bytes.NewReader(r.Data))
	if err != nil {
		t.Fatal(err)
	}
	if red, _, _, _ := img.At(20, 5).RGBA(); red < 0xc000 {
		t.Error("the red band is not at the top after rotating")
	}
}

func TestWideImagesShrinkTo1200(t *testing.T) {
	r, err := Normalize(pngOf(t, 2400, 600, true))
	if err != nil {
		t.Fatal(err)
	}
	if r.ContentType != "image/png" || r.Width != 1200 || r.Height != 300 {
		t.Errorf("%s %dx%d", r.ContentType, r.Width, r.Height)
	}
	if _, err := Normalize(pngOf(t, 600, 4100, false)); err != ErrTooLarge {
		t.Errorf("tall: %v", err)
	}
}

func TestWebPBecomesPNGOrJPEG(t *testing.T) {
	for file, want := range map[string]string{"blue-purple-pink.lossy.webp": "image/jpeg", "yellow_rose.lossy-with-alpha.webp": "image/png"} {
		b, err := os.ReadFile("testdata/" + file)
		if err != nil {
			t.Fatal(err)
		}
		r, err := Normalize(b)
		if err != nil {
			t.Fatal(file, err)
		}
		if r.ContentType != want || Sniff(r.Data) != want {
			t.Errorf("%s: %s, want %s", file, r.ContentType, want)
		}
	}
}

func TestRefusesSVGAndOthers(t *testing.T) {
	for name, b := range map[string][]byte{
		"svg":      []byte(`<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"><rect width="10" height="10"/></svg>`),
		"svg+xml":  []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"></svg>`),
		"html":     []byte(`<html><img src=x onerror=alert(1)></html>`),
		"bmp":      append([]byte("BM"), make([]byte, 60)...),
		"truncate": pngOf(t, 10, 10, false)[:40],
	} {
		if _, err := Normalize(b); err != ErrType {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// pngHeader is a valid signature and IHDR claiming w×h, with no pixel data: the decode guard
// must refuse it from the header alone.
func pngHeader(w, h uint32) []byte {
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr, w)
	binary.BigEndian.PutUint32(ihdr[4:], h)
	ihdr[8], ihdr[9] = 8, 6
	chunk := append([]byte("IHDR"), ihdr...)
	out := []byte("\x89PNG\r\n\x1a\n")
	out = binary.BigEndian.AppendUint32(out, 13)
	out = append(out, chunk...)
	return binary.BigEndian.AppendUint32(out, crc32.ChecksumIEEE(chunk))
}

func TestDecodeGuard(t *testing.T) {
	for _, wh := range [][2]uint32{{9000, 10}, {7000, 7000}, {60000, 60000}} {
		if _, err := Normalize(pngHeader(wh[0], wh[1])); err != ErrTooLarge {
			t.Errorf("%v: %v", wh, err)
		}
	}
}

func gifOf(t *testing.T, w, h, frames int) []byte {
	t.Helper()
	g := &gif.GIF{}
	for i := 0; i < frames; i++ {
		p := image.NewPaletted(image.Rect(0, 0, w, h), color.Palette{color.Black, color.White})
		p.SetColorIndex(i%w, 0, 1)
		g.Image = append(g.Image, p)
		g.Delay = append(g.Delay, 10)
	}
	var b bytes.Buffer
	if err := gif.EncodeAll(&b, g); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestGIFs(t *testing.T) {
	anim := gifOf(t, 64, 32, 3)
	r, err := Normalize(anim)
	if err != nil || !bytes.Equal(r.Data, anim) || r.ContentType != "image/gif" {
		t.Errorf("animated GIF not kept: %v", err)
	}
	if _, err := Normalize(gifOf(t, 1300, 10, 1)); err != ErrGIFWide {
		t.Errorf("wide GIF: %v", err)
	}
	if _, err := Normalize(gifOf(t, 1200, 1200, 40)); err != ErrTooLarge {
		t.Errorf("many large frames: %v", err)
	}
}

func TestSameUploadSameHash(t *testing.T) {
	b := pngOf(t, 30, 30, false)
	r1, _ := Normalize(b)
	r2, _ := Normalize(b)
	if r1.SHA256 != r2.SHA256 {
		t.Error("normalising is not deterministic")
	}
}

func TestPixelBudgetHoldsBackUntilReleased(t *testing.T) {
	b := newPixelBudget(100)
	b.acquire(60)
	var got atomic.Bool
	done := make(chan struct{})
	go func() {
		b.acquire(60)
		got.Store(true)
		close(done)
	}()
	time.Sleep(50 * time.Millisecond)
	if got.Load() {
		t.Fatal("second acquire passed while the budget was spent")
	}
	b.release(60)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("second acquire never passed after release")
	}
}

func TestNormalizeReleasesItsPixels(t *testing.T) {
	old := budget
	budget = newPixelBudget(2 * 200 * 100)
	defer func() { budget = old }()
	src := pngOf(t, 200, 100, false)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := Normalize(src); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if budget.free != 2*200*100 {
		t.Fatalf("budget left at %d", budget.free)
	}
	if _, err := Normalize([]byte("GIF89a")); err == nil {
		t.Fatal("truncated GIF accepted")
	}
	if budget.free != 2*200*100 {
		t.Fatalf("budget left at %d after a refusal", budget.free)
	}
}
