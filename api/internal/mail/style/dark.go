package style

import (
	"fmt"
	"math"
	"strconv"
)

// OKLCH is a colour in the OKLab space as lightness (0 to 1), chroma and hue in degrees.
type OKLCH struct{ L, C, H float64 }

func toLinear(c float64) float64 {
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

func fromLinear(c float64) float64 {
	if c <= 0.0031308 {
		return 12.92 * c
	}
	return 1.055*math.Pow(c, 1/2.4) - 0.055
}

// ToOKLCH reads a #rrggbb colour.
func ToOKLCH(hex string) OKLCH {
	v, _ := strconv.ParseUint(hex[1:], 16, 32)
	r, g, b := toLinear(float64(v>>16&0xff)/255), toLinear(float64(v>>8&0xff)/255), toLinear(float64(v&0xff)/255)
	l := math.Cbrt(0.4122214708*r + 0.5363325363*g + 0.0514459929*b)
	m := math.Cbrt(0.2119034982*r + 0.6806995451*g + 0.1073969566*b)
	s := math.Cbrt(0.0883024619*r + 0.2817188376*g + 0.6299787005*b)
	L := 0.2104542553*l + 0.7936177850*m - 0.0040720468*s
	A := 1.9779984951*l - 2.4285922050*m + 0.4505937099*s
	B := 0.0259040371*l + 0.7827717662*m - 0.8086757660*s
	h := math.Atan2(B, A) * 180 / math.Pi
	if h < 0 {
		h += 360
	}
	return OKLCH{L: L, C: math.Hypot(A, B), H: h}
}

func (c OKLCH) rgb() (r, g, b float64) {
	A, B := c.C*math.Cos(c.H*math.Pi/180), c.C*math.Sin(c.H*math.Pi/180)
	l := c.L + 0.3963377774*A + 0.2158037573*B
	m := c.L - 0.1055613458*A - 0.0638541728*B
	s := c.L - 0.0894841775*A - 1.2914855480*B
	l, m, s = l*l*l, m*m*m, s*s*s
	return 4.0767416621*l - 3.3077115913*m + 0.2309699292*s,
		-1.2684380046*l + 2.6097574011*m - 0.3413193965*s,
		-0.0041960863*l - 0.7034186147*m + 1.7076147010*s
}

// Hex writes the colour, lowering chroma until it fits in sRGB.
func (c OKLCH) Hex() string {
	c.L = math.Max(0, math.Min(1, c.L))
	for i := 0; i < 40; i++ {
		r, g, b := c.rgb()
		if r >= -1e-4 && r <= 1+1e-4 && g >= -1e-4 && g <= 1+1e-4 && b >= -1e-4 && b <= 1+1e-4 {
			break
		}
		c.C *= 0.9
	}
	r, g, b := c.rgb()
	ch := func(x float64) int { return int(math.Round(math.Max(0, math.Min(1, fromLinear(math.Max(0, x)))) * 255)) }
	return fmt.Sprintf("#%02x%02x%02x", ch(r), ch(g), ch(b))
}

// DarkSurface turns a light background into a dark one of the same hue; depth 0 is the page,
// higher depths are raised surfaces (cards, sections). Dark colours stay as they are.
func DarkSurface(hex string, depth int) string {
	c := ToOKLCH(hex)
	if c.L < 0.45 {
		return hex
	}
	c.L = 0.17 + 0.045*float64(depth) + (1-c.L)*0.12
	c.C = math.Min(c.C, 0.035)
	return c.Hex()
}

// DarkInk turns a text colour for a dark background: dark text becomes light, then lightness is
// raised until it reads at 4.5:1 on bg.
func DarkInk(hex, bg string) string {
	c := ToOKLCH(hex)
	if c.L < 0.6 {
		c.L = 1 - c.L*0.45
	}
	for i := 0; i < 30 && Contrast(c.Hex(), bg) < 4.5; i++ {
		c.L = math.Min(1, c.L+0.02)
		c.C *= 0.95
	}
	return c.Hex()
}

// DarkAccent keeps a filled colour (a button) recognisable on a dark surface: very dark ones are
// lifted so the fill stands out from bg.
func DarkAccent(hex, bg string) string {
	c := ToOKLCH(hex)
	for i := 0; i < 30 && Contrast(c.Hex(), bg) < 3 && c.L < 0.85; i++ {
		c.L += 0.02
	}
	return c.Hex()
}

// DarkLine is a border or divider on a dark surface.
func DarkLine(hex, bg string) string {
	c, b := ToOKLCH(hex), ToOKLCH(bg)
	c.L = math.Min(0.95, b.L+0.14)
	c.C = math.Min(c.C, 0.03)
	return c.Hex()
}
