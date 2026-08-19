// Package captcha implements the self-hosted image captcha used when the
// administrator selects the "image" captcha type. Codes are kept in an
// in-memory store, are single-use and expire quickly.
package captcha

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math/big"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

const (
	codeLength   = 5
	imageWidth   = 180
	imageHeight  = 60
	codeTTL      = 5 * time.Minute
	storeMaxSize = 10000
)

// Digits and letters that stay distinguishable even after distortion.
var codeAlphabet = []byte("ABCDEFGHJKMNPQRSTUVWXYZ23456789")

type captchaEntry struct {
	code      string
	expiresAt time.Time
}

var store = struct {
	sync.Mutex
	items map[string]captchaEntry
}{items: make(map[string]captchaEntry)}

func randomInt(max int64) int {
	n, err := rand.Int(rand.Reader, big.NewInt(max))
	if err != nil {
		return 0
	}
	return int(n.Int64())
}

func randomCode() string {
	code := make([]byte, codeLength)
	for i := range code {
		code[i] = codeAlphabet[randomInt(int64(len(codeAlphabet)))]
	}
	return string(code)
}

func purgeExpiredLocked(now time.Time) {
	for id, entry := range store.items {
		if now.After(entry.expiresAt) {
			delete(store.items, id)
		}
	}
}

// Generate creates a new captcha and returns its id together with a PNG
// encoded as a base64 data payload (without the data: prefix).
func Generate() (id string, pngBase64 string, err error) {
	code := randomCode()
	img, err := render(code)
	if err != nil {
		return "", "", err
	}

	id = uuid.NewString()
	now := time.Now()
	store.Lock()
	purgeExpiredLocked(now)
	// Bound the store size so a flood of requests cannot grow it forever.
	if len(store.items) >= storeMaxSize {
		for existingID := range store.items {
			delete(store.items, existingID)
			if len(store.items) < storeMaxSize/2 {
				break
			}
		}
	}
	store.items[id] = captchaEntry{code: code, expiresAt: now.Add(codeTTL)}
	store.Unlock()

	return id, base64.StdEncoding.EncodeToString(img), nil
}

// Verify checks the answer for a captcha id. Every captcha is single-use: it
// is removed from the store regardless of the outcome so a rejected attempt
// forces the client to fetch a fresh image.
func Verify(id string, answer string) bool {
	if id == "" || answer == "" {
		return false
	}
	store.Lock()
	entry, ok := store.items[id]
	delete(store.items, id)
	store.Unlock()
	if !ok || time.Now().After(entry.expiresAt) {
		return false
	}
	if len(answer) != len(entry.code) {
		return false
	}
	for i := 0; i < len(entry.code); i++ {
		a, b := answer[i], entry.code[i]
		if a >= 'a' && a <= 'z' {
			a -= 'a' - 'A'
		}
		if a != b {
			return false
		}
	}
	return true
}

var palette = []color.RGBA{
	{0x1f, 0x49, 0x7d, 0xff},
	{0x9c, 0x27, 0x10, 0xff},
	{0x2e, 0x7d, 0x32, 0xff},
	{0x6a, 0x1b, 0x9a, 0xff},
	{0xb0, 0x52, 0x00, 0xff},
	{0x00, 0x60, 0x6b, 0xff},
}

func randomColor() color.RGBA {
	return palette[randomInt(int64(len(palette)))]
}

func render(code string) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, imageWidth, imageHeight))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{0xf5, 0xf5, 0xf0, 0xff}}, image.Point{}, draw.Src)

	// Background noise dots.
	for i := 0; i < 120; i++ {
		x := randomInt(imageWidth)
		y := randomInt(imageHeight)
		c := randomColor()
		c.A = 0x40
		img.SetRGBA(x, y, c)
	}

	// A few light interference lines crossing the whole image.
	for i := 0; i < 3; i++ {
		c := randomColor()
		c.A = 0x50
		drawLine(img, randomInt(imageWidth), randomInt(imageHeight), randomInt(imageWidth), randomInt(imageHeight), c)
	}

	// Characters are rendered at 2x scale with a small per-glyph offset and a
	// gentle wave so the code stays human-readable while still resisting OCR.
	face := basicfont.Face7x13
	charWidth := imageWidth / (len(code) + 1)
	for i, ch := range code {
		baseX := charWidth/2 + i*charWidth + randomInt(6) - 3
		baseY := imageHeight/2 + randomInt(8) - 4
		drawCharDistorted(img, face, string(ch), baseX, baseY, randomColor())
	}

	// A faint strike-through line over the text.
	strike := randomColor()
	strike.A = 0x70
	midY := imageHeight/2 + randomInt(10) - 5
	drawLine(img, 4, midY+randomInt(6)-3, imageWidth-4, midY+randomInt(6)-3, strike)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encode captcha png: %w", err)
	}
	return buf.Bytes(), nil
}

// drawCharDistorted renders one glyph at 2x scale, column by column, with a
// gentle sine wave vertical shift. The result is easy for humans to read but
// still awkward to segment for OCR.
func drawCharDistorted(img *image.RGBA, face font.Face, ch string, x, y int, c color.RGBA) {
	tmp := image.NewRGBA(image.Rect(0, 0, 12, 20))
	d := &font.Drawer{
		Dst:  tmp,
		Src:  image.NewUniform(c),
		Face: face,
		Dot:  fixed.P(2, 14),
	}
	d.DrawString(ch)

	const scale = 2
	amplitude := 1 + randomInt(2)
	phase := randomInt(628) / 100
	for col := 0; col < 12; col++ {
		shift := int(float64(amplitude) * sinApprox(float64(col+phase)*0.5))
		for row := 0; row < 20; row++ {
			px := tmp.RGBAAt(col, row)
			if px.A == 0 {
				continue
			}
			for sx := 0; sx < scale; sx++ {
				for sy := 0; sy < scale; sy++ {
					dstX := x + col*scale + sx
					dstY := y - 20 + row*scale + sy + shift
					if dstX >= 0 && dstX < imageWidth && dstY >= 0 && dstY < imageHeight {
						img.SetRGBA(dstX, dstY, px)
					}
				}
			}
		}
	}
}

func sinApprox(v float64) float64 {
	// Wrap into [-pi, pi] and use a Taylor approximation; plenty for distortion.
	const pi = 3.141592653589793
	for v > pi {
		v -= 2 * pi
	}
	for v < -pi {
		v += 2 * pi
	}
	return v - v*v*v/6 + v*v*v*v*v/120
}

func drawLine(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	dx := abs(x1 - x0)
	dy := -abs(y1 - y0)
	sx := 1
	if x0 > x1 {
		sx = -1
	}
	sy := 1
	if y0 > y1 {
		sy = -1
	}
	err := dx + dy
	for {
		if x0 >= 0 && x0 < imageWidth && y0 >= 0 && y0 < imageHeight {
			img.SetRGBA(x0, y0, c)
		}
		if x0 == x1 && y0 == y1 {
			return
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
