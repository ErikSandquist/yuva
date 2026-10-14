package monitor

import (
	"math"
	"math/rand/v2"
	"slices"
	"testing"
)

func exactQuantile(sorted []int64, q float64) float64 {
	rank := int(math.Ceil(q * float64(len(sorted))))
	rank = min(max(rank, 1), len(sorted))
	return float64(sorted[rank-1])
}

func durations(r *rand.Rand, n int) []int64 {
	out := make([]int64, n)
	for i := range out {
		switch {
		case i%97 == 0:
			out[i] = 0
		case i%13 == 0:
			out[i] = int64(math.Exp(r.NormFloat64()*0.5 + 14))
		default:
			out[i] = int64(math.Exp(r.NormFloat64()*1.2+10)) + 1
		}
	}
	return out
}

func TestSketchPercentilesWithinBucketWidth(t *testing.T) {
	cases := []struct {
		name  string
		scale int8
		bound float64
		seed  uint64
		n     int
	}{
		{"durations", DurationScale, 0.011, 1, 100_000},
		{"durations, few", DurationScale, 0.011, 2, 37},
		{"sizes", SizeScale, 0.09, 3, 50_000},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := rand.New(rand.NewPCG(c.seed, 99))
			vals := durations(r, c.n)
			s := NewSketch(c.scale)
			for _, v := range vals {
				s.Add(v, 1)
			}
			sorted := slices.Clone(vals)
			slices.Sort(sorted)
			worst := 0.0
			for _, q := range []float64{0.001, 0.01, 0.1, 0.25, 0.5, 0.75, 0.9, 0.95, 0.99, 0.999, 1} {
				want, got := exactQuantile(sorted, q), s.Quantile(q)
				if want == 0 {
					if got != 0 {
						t.Fatalf("p%v: %v, want 0", q*100, got)
					}
					continue
				}
				rel := math.Abs(got-want) / want
				worst = max(worst, rel)
				if rel > c.bound {
					t.Fatalf("p%v: %v, exact %v, off by %.3f%%, more than %.1f%%", q*100, got, want, rel*100, c.bound*100)
				}
			}
			t.Logf("%d values, worst error %.3f%%", c.n, worst*100)
		})
	}
}

func TestSketchMergeIsExactAndAssociative(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 6))
	vals := durations(r, 20_000)
	whole := NewSketch(DurationScale)
	parts := make([]Sketch, 1440)
	for i := range parts {
		parts[i] = NewSketch(DurationScale)
	}
	for i, v := range vals {
		whole.Add(v, 1)
		parts[i%len(parts)].Add(v, 1)
	}
	forward := NewSketch(DurationScale)
	for _, p := range parts {
		forward.Merge(p)
	}
	tree := slices.Clone(parts)
	for len(tree) > 1 {
		var next []Sketch
		for i := 0; i < len(tree); i += 2 {
			m := tree[i].Clone()
			if i+1 < len(tree) {
				m.Merge(tree[i+1])
			}
			next = append(next, m)
		}
		tree = next
	}
	want := string(whole.Encode())
	if string(forward.Encode()) != want || string(tree[0].Encode()) != want {
		t.Fatal("merged sketches differ from the sketch of every value")
	}
	if forward.Count() != uint64(len(vals)) {
		t.Fatalf("count %d, want %d", forward.Count(), len(vals))
	}
}

func TestSketchEncoding(t *testing.T) {
	r := rand.New(rand.NewPCG(7, 8))
	s := NewSketch(DurationScale)
	for _, v := range durations(r, 5000) {
		s.Add(v, uint64(1+r.IntN(3)))
	}
	s.Add(1, 1)
	s.Add(math.MaxInt64, 1)
	back, err := DecodeSketch(s.Encode())
	if err != nil {
		t.Fatal(err)
	}
	if string(back.Encode()) != string(s.Encode()) || back.Count() != s.Count() || back.Scale() != DurationScale {
		t.Fatal("a decoded sketch differs from the one encoded")
	}
	empty, err := DecodeSketch(NewSketch(SizeScale).Encode())
	if err != nil || empty.Count() != 0 || empty.Quantile(0.5) != 0 {
		t.Fatalf("empty sketch: %v %v", empty, err)
	}
	for _, bad := range [][]byte{nil, {5}, {5, 0}, {5, 0, 3, 1}, append(s.Encode(), 0)} {
		if _, err := DecodeSketch(bad); err == nil {
			t.Fatalf("%v decoded", bad)
		}
	}
}

func TestSketchWeightsAndBuckets(t *testing.T) {
	s := NewSketch(DurationScale)
	s.Add(1000, 99)
	s.Add(100_000, 1)
	if s.Count() != 100 {
		t.Fatalf("count %d", s.Count())
	}
	if p := s.Quantile(0.99); math.Abs(p-1000)/1000 > 0.011 {
		t.Fatalf("p99 %v, want about 1000", p)
	}
	if p := s.Quantile(1); math.Abs(p-100_000)/100_000 > 0.011 {
		t.Fatalf("p100 %v, want about 100000", p)
	}
	b := NewSketch(DurationScale)
	for _, v := range []int64{1 << 20, 1<<20 + 1, 1<<20 - 1} {
		b.Add(v, 1)
		if got := b.value(b.index(float64(v))); math.Abs(got-float64(v))/float64(v) > 0.011 {
			t.Fatalf("%d lands in a bucket whose value is %v", v, got)
		}
	}
}

func TestApdex(t *testing.T) {
	const tMS = 500
	threshold := int64(tMS * 1000)
	vals := []int64{100_000, 200_000, 450_000, 600_000, 1_500_000, 1_900_000, 2_500_000, 9_000_000}
	s := NewSketch(DurationScale)
	for _, v := range vals {
		s.Add(v, 1)
	}
	sat, tol := 0, 0
	for _, v := range vals {
		switch {
		case v <= threshold:
			sat++
		case v <= 4*threshold:
			tol++
		}
	}
	want := (float64(sat) + float64(tol)/2) / float64(len(vals))
	if got := s.Apdex(threshold); got != want {
		t.Fatalf("apdex %v, want %v", got, want)
	}
	if NewSketch(DurationScale).Apdex(threshold) != 0 {
		t.Fatal("an empty sketch has no apdex")
	}
}
