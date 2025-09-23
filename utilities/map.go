package utilities

import (
	"slices"
)

// MapGetOrDefault returns m[key] if present, otherwise def.
func MapGetOrDefault[K comparable, V any](m map[K]V, key K, def V) V {
	if v, ok := m[key]; ok {
		return v
	}
	return def
}

// GetOrInsert returns the existing value, or inserts def and returns it.
func GetOrInsert[K comparable, V any](m map[K]V, key K, def V) V {
	if v, ok := m[key]; ok {
		return v
	}
	m[key] = def
	return def
}

// MustGet returns m[key] or panics if missing (useful in tests/config boot).
func MustGet[K comparable, V any](m map[K]V, key K) V {
	if v, ok := m[key]; ok {
		return v
	}
	panic("key not found")
}

// HasAnyKey reports whether at least one key exists in m.
func HasAnyKey[K comparable, V any](m map[K]V, keys ...K) bool {
	for _, k := range keys {
		if _, ok := m[k]; ok {
			return true
		}
	}
	return false
}

// HasAllKeys reports whether all keys exist in m.
func HasAllKeys[K comparable, V any](m map[K]V, keys ...K) bool {
	for _, k := range keys {
		if _, ok := m[k]; !ok {
			return false
		}
	}
	return true
}

// Keys returns keys of m (iteration order is random).
func Keys[K comparable, V any](m map[K]V) []K {
	out := make([]K, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// KeysSorted returns keys of m sorted with the provided less function.
// Example: KeysSorted(m, func(a, b string) bool { return a < b })
func KeysSorted[K comparable, V any](m map[K]V, less func(K, K) bool) []K {
	ks := Keys(m)
	slices.SortFunc(ks, func(a, b K) int {
		switch {
		case less(a, b):
			return -1
		case less(b, a):
			return 1
		default:
			return 0
		}
	})
	return ks
}

// Values returns values of m (no specific order).
func Values[K comparable, V any](m map[K]V) []V {
	out := make([]V, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

// Clone returns a shallow copy of m (nil-safe).
func Clone[K comparable, V any](m map[K]V) map[K]V {
	if m == nil {
		return nil
	}
	out := make(map[K]V, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// Merge copies all entries from src into dst (overwrites conflicts).
func Merge[K comparable, V any](dst, src map[K]V) {
	for k, v := range src {
		dst[k] = v
	}
}

// MergeIfAbsent copies entries that don't already exist in dst.
func MergeIfAbsent[K comparable, V any](dst, src map[K]V) {
	for k, v := range src {
		if _, exists := dst[k]; !exists {
			dst[k] = v
		}
	}
}

// MergeFunc merges src into dst using resolver when a key exists in both.
func MergeFunc[K comparable, V any](dst, src map[K]V, resolver func(dstV, srcV V) V) {
	for k, v := range src {
		if old, ok := dst[k]; ok {
			dst[k] = resolver(old, v)
		} else {
			dst[k] = v
		}
	}
}

// FilterKeys returns a new map keeping entries whose key satisfies f.
func FilterKeys[K comparable, V any](m map[K]V, f func(K) bool) map[K]V {
	out := make(map[K]V)
	for k, v := range m {
		if f(k) {
			out[k] = v
		}
	}
	return out
}

// FilterValues returns a new map keeping entries whose value satisfies f.
func FilterValues[K comparable, V any](m map[K]V, f func(V) bool) map[K]V {
	out := make(map[K]V)
	for k, v := range m {
		if f(v) {
			out[k] = v
		}
	}
	return out
}

// MapValues transforms values with f (keys unchanged).
func MapValues[K comparable, V any, R any](m map[K]V, f func(V) R) map[K]R {
	out := make(map[K]R, len(m))
	for k, v := range m {
		out[k] = f(v)
	}
	return out
}

// MapKeys transforms keys with f; last-write-wins on key collisions.
func MapKeys[K comparable, V any, NK comparable](m map[K]V, f func(K) NK) map[NK]V {
	out := make(map[NK]V, len(m))
	for k, v := range m {
		out[f(k)] = v
	}
	return out
}

// Pick returns a new map with only the specified keys (missing keys ignored).
func Pick[K comparable, V any](m map[K]V, keys ...K) map[K]V {
	out := make(map[K]V, len(keys))
	for _, k := range keys {
		if v, ok := m[k]; ok {
			out[k] = v
		}
	}
	return out
}

// Omit returns a new map without the specified keys.
func Omit[K comparable, V any](m map[K]V, keys ...K) map[K]V {
	if len(m) == 0 {
		return map[K]V{}
	}
	rm := make(map[K]struct{}, len(keys))
	for _, k := range keys {
		rm[k] = struct{}{}
	}
	out := make(map[K]V, len(m))
	for k, v := range m {
		if _, skip := rm[k]; !skip {
			out[k] = v
		}
	}
	return out
}

// Invert swaps keys and values (requires comparable values).
// If multiple keys share the same value, the last one wins.
func Invert[K comparable, V comparable](m map[K]V) map[V]K {
	out := make(map[V]K, len(m))
	for k, v := range m {
		out[v] = k
	}
	return out
}

// MapGroupBy groups items from slice s into map[key][]T using key selector k.
func MapGroupBy[T any, K comparable](s []T, k func(T) K) map[K][]T {
	out := make(map[K][]T)
	for _, v := range s {
		key := k(v)
		out[key] = append(out[key], v)
	}
	return out
}

// CountBy counts items from slice s by key selector k.
func CountBy[T any, K comparable](s []T, k func(T) K) map[K]int {
	out := make(map[K]int)
	for _, v := range s {
		out[k(v)]++
	}
	return out
}

// UnionKeys returns the union of keys from a and b (order unspecified).
func UnionKeys[K comparable, V1 any, V2 any](a map[K]V1, b map[K]V2) []K {
	out := make([]K, 0, len(a)+len(b))
	seen := make(map[K]struct{}, len(a)+len(b))
	for k := range a {
		if _, ok := seen[k]; !ok {
			seen[k] = struct{}{}
			out = append(out, k)
		}
	}
	for k := range b {
		if _, ok := seen[k]; !ok {
			seen[k] = struct{}{}
			out = append(out, k)
		}
	}
	return out
}

// IntersectKeys returns keys present in both a and b (order by iteration of a).
func IntersectKeys[K comparable, V1 any, V2 any](a map[K]V1, b map[K]V2) []K {
	if len(a) == 0 || len(b) == 0 {
		return nil
	}
	out := make([]K, 0)
	for k := range a {
		if _, ok := b[k]; ok {
			out = append(out, k)
		}
	}
	return out
}

// DiffKeys returns (aOnly, bOnly) — keys present only in a, and only in b.
func DiffKeys[K comparable, V1 any, V2 any](a map[K]V1, b map[K]V2) (aOnly, bOnly []K) {
	for k := range a {
		if _, ok := b[k]; !ok {
			aOnly = append(aOnly, k)
		}
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			bOnly = append(bOnly, k)
		}
	}
	return
}

// EqualFunc compares two maps using a value equality function (nil-safe).
// Keys must match exactly; values compared with eq.
func EqualFunc[K comparable, V any](a, b map[K]V, eq func(V, V) bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k, av := range a {
		bv, ok := b[k]
		if !ok || !eq(av, bv) {
			return false
		}
	}
	return true
}
