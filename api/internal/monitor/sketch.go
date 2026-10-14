package monitor

import (
	"encoding/binary"
	"errors"
	"math"
	"slices"
)

const (
	// DurationScale gives buckets of 2^(1/32), so a duration percentile is within 1.1% of the
	// exact value; SizeScale buckets of 2^(1/4), within 9%. Both are the bucket layouts of an
	// OpenTelemetry exponential histogram at that scale.
	DurationScale = 5
	SizeScale     = 2
)

type bucket struct {
	index int32
	count uint64
}

// Sketch is a sparse log-bucketed histogram: bucket i holds the values in (base^i, base^(i+1)]
// with base 2^(2^-scale), and zero holds the zeros. Adding counts per bucket makes merging exact
// and associative.
type Sketch struct {
	scale   int8
	zero    uint64
	buckets []bucket
}

func NewSketch(scale int8) Sketch { return Sketch{scale: scale} }

func (s Sketch) Scale() int8 { return s.scale }

func (s Sketch) base() float64 { return math.Exp2(math.Exp2(-float64(s.scale))) }

func (s Sketch) lower(i int32) float64 { return math.Exp2(float64(i) * math.Exp2(-float64(s.scale))) }

func (s Sketch) index(v float64) int32 {
	i := int32(math.Ceil(math.Log2(v)*math.Exp2(float64(s.scale)))) - 1
	for v <= s.lower(i) {
		i--
	}
	for v > s.lower(i+1) {
		i++
	}
	return i
}

// Add counts n values of v; values at or below zero count as zero.
func (s *Sketch) Add(v int64, n uint64) {
	if n == 0 {
		return
	}
	if v <= 0 {
		s.zero += n
		return
	}
	i := s.index(float64(v))
	at, found := slices.BinarySearchFunc(s.buckets, i, func(b bucket, i int32) int { return int(b.index - i) })
	if found {
		s.buckets[at].count += n
		return
	}
	s.buckets = slices.Insert(s.buckets, at, bucket{index: i, count: n})
}

// Count is how many values the sketch holds.
func (s Sketch) Count() uint64 {
	n := s.zero
	for _, b := range s.buckets {
		n += b.count
	}
	return n
}

// Merge adds o's counts; sketches of different scales are not merged.
func (s *Sketch) Merge(o Sketch) {
	if o.zero == 0 && len(o.buckets) == 0 {
		return
	}
	if s.zero == 0 && len(s.buckets) == 0 {
		s.scale = o.scale
	}
	if s.scale != o.scale {
		return
	}
	s.zero += o.zero
	if len(s.buckets) == 0 {
		s.buckets = slices.Clone(o.buckets)
		return
	}
	out := make([]bucket, 0, len(s.buckets)+len(o.buckets))
	a, b := s.buckets, o.buckets
	for len(a) > 0 && len(b) > 0 {
		switch {
		case a[0].index < b[0].index:
			out, a = append(out, a[0]), a[1:]
		case a[0].index > b[0].index:
			out, b = append(out, b[0]), b[1:]
		default:
			out, a, b = append(out, bucket{a[0].index, a[0].count + b[0].count}), a[1:], b[1:]
		}
	}
	s.buckets = append(append(out, a...), b...)
}

func (s Sketch) Clone() Sketch {
	return Sketch{scale: s.scale, zero: s.zero, buckets: slices.Clone(s.buckets)}
}

// value is the bucket's point closest, relative to the value, to every value in it: the harmonic
// mean of its bounds, at most (base-1)/(base+1) away.
func (s Sketch) value(i int32) float64 {
	b := s.base()
	return s.lower(i) * 2 * b / (1 + b)
}

// Quantile is the q-quantile (0 to 1) by nearest rank: the value the ceil(q·n)-th smallest value
// lies in the bucket of.
func (s Sketch) Quantile(q float64) float64 {
	n := s.Count()
	if n == 0 {
		return 0
	}
	rank := uint64(math.Ceil(q * float64(n)))
	rank = min(max(rank, 1), n)
	if rank <= s.zero {
		return 0
	}
	seen := s.zero
	for _, b := range s.buckets {
		seen += b.count
		if seen >= rank {
			return s.value(b.index)
		}
	}
	return s.value(s.buckets[len(s.buckets)-1].index)
}

// Apdex is (satisfied + tolerating/2) / n for a threshold t: satisfied values are at most t,
// tolerating ones at most 4t, a bucket counting where its value falls.
func (s Sketch) Apdex(t int64) float64 {
	n := s.Count()
	if n == 0 {
		return 0
	}
	sat, tol := float64(s.zero), 0.0
	for _, b := range s.buckets {
		switch v := s.value(b.index); {
		case v <= float64(t):
			sat += float64(b.count)
		case v <= 4*float64(t):
			tol += float64(b.count)
		}
	}
	return (sat + tol/2) / float64(n)
}

// Encode is the stored form: the scale, the zero count, the number of buckets and then each
// bucket's index (as a delta from the one before) and count, all varints.
func (s Sketch) Encode() []byte {
	out := make([]byte, 0, 3+len(s.buckets)*4)
	out = append(out, byte(s.scale))
	out = binary.AppendUvarint(out, s.zero)
	out = binary.AppendUvarint(out, uint64(len(s.buckets)))
	prev := int32(0)
	for _, b := range s.buckets {
		out = binary.AppendVarint(out, int64(b.index-prev))
		out = binary.AppendUvarint(out, b.count)
		prev = b.index
	}
	return out
}

var errSketch = errors.New("monitor: not a sketch")

func DecodeSketch(raw []byte) (Sketch, error) {
	if len(raw) < 3 {
		return Sketch{}, errSketch
	}
	s := Sketch{scale: int8(raw[0])}
	rest := raw[1:]
	read := func() (uint64, bool) {
		v, n := binary.Uvarint(rest)
		if n <= 0 {
			return 0, false
		}
		rest = rest[n:]
		return v, true
	}
	var ok bool
	if s.zero, ok = read(); !ok {
		return Sketch{}, errSketch
	}
	n, ok := read()
	if !ok || n > uint64(len(rest)) {
		return Sketch{}, errSketch
	}
	s.buckets = make([]bucket, 0, n)
	prev := int64(0)
	for range n {
		d, k := binary.Varint(rest)
		if k <= 0 {
			return Sketch{}, errSketch
		}
		rest = rest[k:]
		c, ok := read()
		if !ok {
			return Sketch{}, errSketch
		}
		prev += d
		if prev < math.MinInt32 || prev > math.MaxInt32 || (len(s.buckets) > 0 && int32(prev) <= s.buckets[len(s.buckets)-1].index) {
			return Sketch{}, errSketch
		}
		s.buckets = append(s.buckets, bucket{index: int32(prev), count: c})
	}
	if len(rest) != 0 {
		return Sketch{}, errSketch
	}
	return s, nil
}
