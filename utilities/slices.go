package utilities

func IsSubset[T comparable](subset, superset []T) bool {
	set := make(map[T]struct{})
	for _, val := range superset {
		set[val] = struct{}{}
	}
	for _, val := range subset {
		if _, exists := set[val]; !exists {
			return false
		}
	}
	return true
}

// Filter keeps only elements matching f
func Filter[T any](s []T, f func(T) bool) []T {
	out := s[:0]
	for _, v := range s {
		if f(v) {
			out = append(out, v)
		}
	}
	return out
}

// Partition splits s into matching and non-matching slices
func Partition[T any](s []T, f func(T) bool) (matches, rest []T) {
	for _, v := range s {
		if f(v) {
			matches = append(matches, v)
		} else {
			rest = append(rest, v)
		}
	}
	return
}

func Insert[T any](s []T, i int, v T) []T {
	if i <= 0 {
		return append([]T{v}, s...)
	}
	if i >= len(s) {
		return append(s, v)
	}
	s = append(s, v)     // grow by 1
	copy(s[i+1:], s[i:]) // shift right
	s[i] = v
	return s
}

// Chunk splits s into slices of size n
func Chunk[T any](s []T, n int) [][]T {
	if n <= 0 {
		return nil
	}
	if len(s) == 0 {
		return [][]T{}
	}
	var out [][]T
	for i := 0; i < len(s); i += n {
		end := i + n
		if end > len(s) {
			end = len(s)
		}
		out = append(out, s[i:end])
	}
	return out
}

// Count returns how many elements equal to v appear in s.
func Count[T comparable](s []T, v T) int {
	n := 0
	for _, x := range s {
		if x == v {
			n++
		}
	}
	return n
}

// GetOrDefault returns s[i] or def if i is out of bounds.
func GetOrDefault[T any](s []T, i int, def T) T {
	if i < 0 || i >= len(s) {
		return def
	}
	return s[i]
}

// Reject removes elements for which f returns true (opposite of Filter).
func Reject[T any](s []T, f func(T) bool) []T {
	out := s[:0]
	for _, v := range s {
		if !f(v) {
			out = append(out, v)
		}
	}
	return out
}

// Map applies f to each element, returning a new slice of the same length.
func Map[T any, R any](s []T, f func(T) R) []R {
	out := make([]R, len(s))
	for i, v := range s {
		out[i] = f(v)
	}
	return out
}

// FlatMap maps each element to a slice and flattens the results.
func FlatMap[T any, R any](s []T, f func(T) []R, avgPerElem int) []R {
	if len(s) == 0 {
		return nil
	}
	var out []R
	if avgPerElem > 0 {
		out = make([]R, 0, len(s)*avgPerElem)
	}
	for _, v := range s {
		out = append(out, f(v)...)
	}
	return out
}

// Reduce folds s left-to-right: acc = f(acc, v) for each v.
func Reduce[T any, R any](s []T, init R, f func(R, T) R) R {
	acc := init
	for _, v := range s {
		acc = f(acc, v)
	}
	return acc
}

// GroupBy groups elements by key selector k into map[key][]T.
func GroupBy[T any, K comparable](s []T, k func(T) K) map[K][]T {
	m := make(map[K][]T)
	for _, v := range s {
		key := k(v)
		m[key] = append(m[key], v)
	}
	return m
}

// UniqueUnordered removes duplicates while preserving first occurrence order (comparable T).
func UniqueUnordered[T comparable](s []T) []T {
	seen := make(map[T]struct{}, len(s))
	out := s[:0]
	for _, v := range s {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// UniqueBy deduplicates using a key selector (works for non-comparable T).
func UniqueBy[T any, K comparable](s []T, key func(T) K) []T {
	seen := make(map[K]struct{}, len(s))
	out := s[:0]
	for _, v := range s {
		k := key(v)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, v)
	}
	return out
}

// EqualSet reports whether two slices contain the same unique elements (ignores multiplicity).
func EqualSet[T comparable](a, b []T) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	ma := make(map[T]struct{}, len(a))
	mb := make(map[T]struct{}, len(b))
	for _, v := range a {
		ma[v] = struct{}{}
	}
	for _, v := range b {
		mb[v] = struct{}{}
	}
	if len(ma) != len(mb) {
		return false
	}
	for v := range ma {
		if _, ok := mb[v]; !ok {
			return false
		}
	}
	return true
}

// Diff returns (aMinusB, bMinusA) treating slices as sets (ignores multiplicity).
func Diff[T comparable](a, b []T) (onlyA, onlyB []T) {
	setA := make(map[T]struct{}, len(a))
	setB := make(map[T]struct{}, len(b))
	for _, v := range a {
		setA[v] = struct{}{}
	}
	for _, v := range b {
		setB[v] = struct{}{}
	}
	for v := range setA {
		if _, ok := setB[v]; !ok {
			onlyA = append(onlyA, v)
		}
	}
	for v := range setB {
		if _, ok := setA[v]; !ok {
			onlyB = append(onlyB, v)
		}
	}
	return
}

// Intersect returns elements present in both a and b (set semantics; order by a).
func Intersect[T comparable](a, b []T) []T {
	if len(a) == 0 || len(b) == 0 {
		return nil
	}
	setB := make(map[T]struct{}, len(b))
	for _, v := range b {
		setB[v] = struct{}{}
	}
	out := make([]T, 0, len(a))
	seen := make(map[T]struct{}, len(a)) // prevent dupes if a contains duplicates
	for _, v := range a {
		if _, ok := setB[v]; ok {
			if _, dup := seen[v]; !dup {
				out = append(out, v)
				seen[v] = struct{}{}
			}
		}
	}
	return out
}

// Without returns a copy of s without any elements in remove (set semantics).
func Without[T comparable](s, remove []T) []T {
	if len(s) == 0 {
		return nil
	}
	if len(remove) == 0 {
		return append([]T(nil), s...)
	}
	rm := make(map[T]struct{}, len(remove))
	for _, v := range remove {
		rm[v] = struct{}{}
	}
	out := make([]T, 0, len(s))
	for _, v := range s {
		if _, bad := rm[v]; !bad {
			out = append(out, v)
		}
	}
	return out
}

func InsertSlice[T any](s []T, i int, t []T) []T {
	if len(t) == 0 {
		return s
	}
	if i <= 0 {
		out := make([]T, 0, len(t)+len(s))
		out = append(out, t...)
		out = append(out, s...)
		return out
	}
	if i >= len(s) {
		return append(s, t...)
	}
	oldLen := len(s)
	s = append(s, t...) // grow
	copy(s[i+len(t):], s[i:oldLen])
	copy(s[i:], t)
	return s
}

// Move moves the element at index from -> to (stable relative order).
func Move[T any](s []T, from, to int) []T {
	n := len(s)
	if from < 0 || from >= n || to < 0 || to >= n || from == to {
		return s
	}
	v := s[from]
	if from < to {
		copy(s[from:], s[from+1:to+1])
	} else {
		copy(s[to+1:], s[to:from])
	}
	s[to] = v
	return s
}

// Rotate rotates s by k steps (k>0 rotates right, k<0 rotates left).
func Rotate[T any](s []T, k int) []T {
	n := len(s)
	if n == 0 {
		return s
	}
	k %= n
	if k < 0 {
		k += n
	}
	reverse := func(a []T) {
		for i, j := 0, len(a)-1; i < j; i, j = i+1, j-1 {
			a[i], a[j] = a[j], a[i]
		}
	}
	reverse(s)
	reverse(s[:k])
	reverse(s[k:])
	return s
}

// Pad extends s to length n by appending copies of fill.
// If len(s) >= n, returns s unchanged.
func Pad[T any](s []T, n int, fill T) []T {
	if n <= len(s) {
		return s
	}
	add := n - len(s)
	for i := 0; i < add; i++ {
		s = append(s, fill)
	}
	return s
}

// Window returns sliding windows of size k with step 'step' (default 1 if step<=0).
// If k <= 0 or k > len(s), returns nil.
func Window[T any](s []T, k, step int) [][]T {
	if k <= 0 || k > len(s) {
		return nil
	}
	if step <= 0 {
		step = 1
	}
	var out [][]T
	for i := 0; i+k <= len(s); i += step {
		out = append(out, s[i:i+k])
	}
	return out
}

// Concat returns a new slice containing s followed by t.
func Concat[T any](s, t []T) []T {
	out := make([]T, 0, len(s)+len(t))
	out = append(out, s...)
	out = append(out, t...)
	return out
}

// First returns up to n elements from the start of s.
func First[T any](s []T, n int) []T {
	if n <= 0 {
		return nil
	}
	if n >= len(s) {
		return append([]T(nil), s...)
	}
	return append([]T(nil), s[:n]...)
}

// Last returns up to n elements from the end of s.
func Last[T any](s []T, n int) []T {
	if n <= 0 {
		return nil
	}
	if n >= len(s) {
		return append([]T(nil), s...)
	}
	return append([]T(nil), s[len(s)-n:]...)
}

// Pop removes and returns the last element. ok=false if empty.
func Pop[T any](s []T) (tail T, rest []T, ok bool) {
	if len(s) == 0 {
		var zero T
		return zero, s, false
	}
	return s[len(s)-1], s[:len(s)-1], true
}

// Shift removes and returns the first element. ok=false if empty.
func Shift[T any](s []T) (head T, rest []T, ok bool) {
	if len(s) == 0 {
		var zero T
		return zero, s, false
	}
	return s[0], s[1:], true
}

// SwapRemove removes an element at i by swapping it with the last element (O(1), not stable).
// Returns a removed element, new slice, and ok=false if i out of range.
func SwapRemove[T any](s []T, i int) (removed T, rest []T, ok bool) {
	if i < 0 || i >= len(s) {
		var zero T
		return zero, s, false
	}
	last := len(s) - 1
	s[i], s[last] = s[last], s[i]
	removed = s[last]
	rest = s[:last]
	return removed, rest, true
}
