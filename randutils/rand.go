package randutils

import (
	"container/heap"
	crypto "crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	"math"
	mrand "math/rand"

	"github.com/okieraised/go-common/ptrutils"
)

var (
	ErrWeightsLenMismatch = errors.New("items and weights length mismatch")
	ErrNonPositiveWeight  = errors.New("all weights must be > 0")
	ErrEmptyAlphabet      = errors.New("alphabet must not be empty")
	ErrNonPositiveK       = errors.New("k must be > 0")
	ErrKExceedsLength     = errors.New("k exceeds input length")
	ErrNoChoices          = errors.New("no choices")
	ErrNonPositiveSum     = errors.New("sum of weights must be > 0")
	ErrNegativeWeight     = errors.New("weights must be >= 0")
	ErrInvalidRange       = errors.New("invalid range")
)

// rngUint64 returns a random uint64 from an io.Reader (usually crypto/rand.Reader).
func rngUint64(r io.Reader) (uint64, error) {
	var b [8]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(b[:]), nil
}

// RandIntRange returns a random int in [min, max] inclusive using math/rand.
func RandIntRange(r *mrand.Rand, min, max int) (int, error) {
	if max < min {
		return 0, ErrInvalidRange
	}
	// NOTE: handle full range and single-value fast paths
	if max == min {
		return min, nil
	}
	n := max - min + 1
	return min + r.Intn(n), nil
}

// RandBool returns true with probability p in [0,1].
func RandBool(r *mrand.Rand, p float64) bool {
	if p <= 0 {
		return false
	}
	if p >= 1 {
		return true
	}
	return r.Float64() < p
}

// Shuffle shuffles a slice in-place using Fisher–Yates.
func Shuffle[T any](r *mrand.Rand, s []T) {
	r.Shuffle(len(s), func(i, j int) { s[i], s[j] = s[j], s[i] })
}

// SampleWithoutReplacement returns k items from data without replacement (uniform).
func SampleWithoutReplacement[T any](r *mrand.Rand, data []T, k int) ([]T, error) {
	n := len(data)
	if k <= 0 {
		return nil, ErrNonPositiveK
	}
	if k > n {
		return nil, ErrKExceedsLength
	}
	// copy and partial shuffle
	cp := make([]T, n)
	copy(cp, data)
	for i := 0; i < k; i++ {
		j := i + r.Intn(n-i)
		cp[i], cp[j] = cp[j], cp[i]
	}
	out := make([]T, k)
	copy(out, cp[:k])
	return out, nil
}

// RandPermRange returns a permutation of integers in [start, end] inclusive.
func RandPermRange(r *mrand.Rand, start, end int) ([]int, error) {
	if end < start {
		return nil, ErrInvalidRange
	}
	n := end - start + 1
	perm := r.Perm(n)
	for i := range perm {
		perm[i] += start
	}
	return perm, nil
}

// RandString generates a random string of length n from the given alphabet (fast, not CSPRNG).
func RandString(r *mrand.Rand, n int, alphabet string) (string, error) {
	if len(alphabet) == 0 {
		return "", ErrEmptyAlphabet
	}
	b := make([]byte, n)
	for i := range b {
		b[i] = alphabet[r.Intn(len(alphabet))]
	}
	return string(b), nil
}

// Choice with weight for WeightedChoice.
type Choice[T any] struct {
	Item   T
	Weight float64 // must be >= 0
}

// WeightedChoice selects one item from choices, with probability proportional to Weight.
func WeightedChoice[T any](r *mrand.Rand, choices []Choice[T]) (T, error) {
	var zero T
	if len(choices) == 0 {
		return zero, ErrNoChoices
	}
	sum := 0.0
	for _, c := range choices {
		if c.Weight < 0 {
			return zero, ErrNegativeWeight
		}
		sum += c.Weight
	}
	if !(sum > 0) {
		return zero, ErrNonPositiveSum
	}
	// inversion sampling
	x := r.Float64() * sum
	for _, c := range choices {
		x -= c.Weight
		if x <= 0 {
			return c.Item, nil
		}
	}
	// numeric edge case
	return choices[len(choices)-1].Item, nil
}

// Normal is a stateful normal(μ,σ) generator (Box–Muller, with spare).
type Normal struct {
	R        *mrand.Rand
	Mu       float64
	Sigma    float64
	spare    float64
	hasSpare bool
}

// NewNormal creates a Normal generator using r, mean mu and stddev sigma (>0).
func NewNormal(r *mrand.Rand, mu, sigma float64) *Normal {
	return &Normal{R: r, Mu: mu, Sigma: sigma}
}

// Float64 returns a N(mu, sigma^2) sample.
func (n *Normal) Float64() float64 {
	if n.hasSpare {
		n.hasSpare = false
		return n.Mu + n.Sigma*n.spare
	}
	// Box–Muller
	var u1, u2 float64
	for {
		u1 = n.R.Float64()
		if u1 > 0 { // avoid log(0)
			break
		}
	}
	u2 = n.R.Float64()
	r := math.Sqrt(-2 * math.Log(u1))
	z0 := r * math.Cos(2*math.Pi*u2)
	z1 := r * math.Sin(2*math.Pi*u2)
	n.spare = z1
	n.hasSpare = true
	return n.Mu + n.Sigma*z0
}

// Reservoir is an online reservoir sampler of fixed size k.
type Reservoir[T any] struct {
	R   *mrand.Rand
	K   int
	N   int
	Buf []T
}

// NewReservoir creates a reservoir sampler for K items.
func NewReservoir[T any](r *mrand.Rand, k int) *Reservoir[T] {
	return &Reservoir[T]{R: r, K: k, Buf: make([]T, 0, k)}
}

// Add considers x for inclusion; after streaming, call Sample().
func (res *Reservoir[T]) Add(x T) {
	res.N++
	if len(res.Buf) < res.K {
		res.Buf = append(res.Buf, x)
		return
	}
	// Replace it with probability K/N
	j := res.R.Intn(res.N)
	if j < res.K {
		res.Buf[j] = x
	}
}

// Sample returns a copy of the current reservoir (size <= K).
func (res *Reservoir[T]) Sample() []T {
	out := make([]T, len(res.Buf))
	copy(out, res.Buf)
	return out
}

// ReservoirSample is a convenience for an in-memory slice.
func ReservoirSample[T any](r *mrand.Rand, stream []T, k int) ([]T, error) {
	if k <= 0 {
		return nil, ErrNonPositiveK
	}
	res := NewReservoir[T](r, k)
	for _, x := range stream {
		res.Add(x)
	}
	return res.Sample(), nil
}

// CryptoUint64 returns a uniformly random uint64 using crypto/rand.
func CryptoUint64() (uint64, error) { return rngUint64(crypto.Reader) }

// CryptoIntRange returns a secure random int in [min,max] inclusive using rejection sampling.
func CryptoIntRange(min, max int) (int, error) {
	if max < min {
		return 0, ErrInvalidRange
	}
	if max == min {
		return min, nil
	}
	// Turn the range into [0,n) and rejection sample on uint64
	n := uint64(max - min + 1)
	// Find the largest multiple of n that fits in uint64
	limit := (math.MaxUint64 / n) * n
	for {
		x, err := CryptoUint64()
		if err != nil {
			return 0, err
		}
		if x < limit {
			return int(min + int(x%n)), nil
		}
	}
}

// CryptoString generates a CSPRNG string of length n from alphabet.
// Uses unbiased modulo-rejection to avoid bias when len(alphabet) doesn't divide 256/64bits nicely.
func CryptoString(n int, alphabet string) (string, error) {
	if len(alphabet) == 0 {
		return "", ErrEmptyAlphabet
	}
	out := make([]byte, n)
	// Rejection on uint64 blocks for fewer syscalls
	L := uint64(len(alphabet))
	limit := (math.MaxUint64 / L) * L
	for i := 0; i < n; i++ {
		for {
			x, err := CryptoUint64()
			if err != nil {
				return "", err
			}
			if x < limit {
				out[i] = alphabet[x%L]
				break
			}
		}
	}
	return string(out), nil
}

// WeightedSampleWithoutReplacement selects k distinct items
// with probability proportional to weights (w_i > 0).
// Complexity: O(n log k) with a fixed-size max-heap.
func WeightedSampleWithoutReplacement[T any](r *mrand.Rand, items []T, weights []float64, k int) ([]T, error) {
	n := len(items)
	if n != len(weights) {
		return nil, ErrWeightsLenMismatch
	}
	if k <= 0 {
		return nil, ErrNonPositiveK
	}
	if k > n {
		return nil, ErrKExceedsLength
	}
	// Max-heap of the current k smallest keys (t = -ln(U)/w).
	h := make(esMaxHeap, 0, k)
	for i := 0; i < n; i++ {
		w := weights[i]
		if !(w > 0) {
			return nil, ErrNonPositiveWeight
		}
		u := r.Float64()
		// Guard against u == 0; push away from 0
		for u == 0 {
			u = r.Float64()
		}
		t := -math.Log(u) / w // smaller is better
		if len(h) < k {
			heap.Push(&h, esPair{idx: i, key: t})
			continue
		}
		// If this key is better (smaller) than the worst in heap, replace
		if t < h[0].key {
			h[0] = esPair{idx: i, key: t}
			heap.Fix(&h, 0)
		}
	}
	// Extract indices from heap (unordered)
	out := make([]T, len(h))
	for i := range h {
		out[i] = items[h[i].idx]
	}
	return out, nil
}

type esPair struct {
	idx int
	key float64
}

// esMaxHeap is a max-heap by key.
type esMaxHeap []esPair

func (h *esMaxHeap) Len() int {
	return len(ptrutils.Deref(h))
}

func (h *esMaxHeap) Less(i, j int) bool {
	return ptrutils.Deref(h)[i].key > ptrutils.Deref(h)[j].key
} // max-heap

func (h *esMaxHeap) Swap(i, j int) {
	ptrutils.Deref(h)[i], ptrutils.Deref(h)[j] = ptrutils.Deref(h)[j], ptrutils.Deref(h)[i]
}
func (h *esMaxHeap) Push(x any) {
	*h = append(*h, x.(esPair))
}

func (h *esMaxHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

type ZipfInt struct {
	z        *mrand.Zipf
	min, max int
	span     uint64
}

// NewZipfInt creates a discrete Zipf sampler over [min, max].
// Parameters match stdlib: s (>1 typical), v (>=1 typical).
// Output x = min + z.Uint64() with z in [0, span].
func NewZipfInt(r *mrand.Rand, s, v float64, min, max int) (*ZipfInt, error) {
	if max < min {
		return nil, ErrInvalidRange
	}
	span := uint64(max - min)
	z := mrand.NewZipf(r, s, v, span)
	return &ZipfInt{z: z, min: min, max: max, span: span}, nil
}

// Int returns one Zipf-distributed integer in [min, max].
func (z *ZipfInt) Int() int {
	return z.min + int(z.z.Uint64())
}

// SampleN returns k Zipf samples in [min,max].
func (z *ZipfInt) SampleN(k int) []int {
	out := make([]int, k)
	for i := 0; i < k; i++ {
		out[i] = z.Int()
	}
	return out
}

type SplitMix64 struct {
	state uint64
}

// NewSplitMix64 seeds with a 64-bit value.
func NewSplitMix64(seed uint64) *SplitMix64 {
	return &SplitMix64{state: seed}
}

// Seed implements rand.Source.
func (s *SplitMix64) Seed(seed int64) {
	s.state = uint64(seed)
}

// Uint64 implements rand.Source64.
func (s *SplitMix64) Uint64() uint64 {
	// Step the state
	s.state += 0x9E3779B97F4A7C15
	z := s.state
	// Mix
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

// Int63 implements rand.Source.
func (s *SplitMix64) Int63() int64 {
	return int64(s.Uint64() >> 1)
}

// NewRandSplitMix64 returns *rand.Rand backed by SplitMix64.
func NewRandSplitMix64(seed int64) *mrand.Rand {
	return mrand.New(NewSplitMix64(uint64(seed)))
}
