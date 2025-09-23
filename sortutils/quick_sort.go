package sortutils

import "math/rand"

func quickSort[T any](x []T, less func(a, b T) bool, lo, hi int) {
	for lo < hi {
		i, j := lo, hi
		// median-of-three pivot
		m := lo + (hi-lo)/2
		p := medianIndex(x, less, lo, m, hi)
		pivot := x[p]

		for i <= j {
			for less(x[i], pivot) {
				i++
			}
			for less(pivot, x[j]) {
				j--
			}
			if i <= j {
				x[i], x[j] = x[j], x[i]
				i++
				j--
			}
		}
		// Tail recursion elimination: recurse on smaller side first
		if (j - lo) < (hi - i) {
			if lo < j {
				quickSort[T](x, less, lo, j)
			}
			lo = i
		} else {
			if i < hi {
				quickSort[T](x, less, i, hi)
			}
			hi = j
		}
	}
}

func medianIndex[T any](x []T, less func(a, b T) bool, a, b, c int) int {
	switch rand.Intn(3) {
	case 1:
		a, b = b, a
	case 2:
		a, c = c, a
	}
	if less(x[a], x[b]) {
		if less(x[b], x[c]) { // a < b < c
			return b
		} else if less(x[a], x[c]) { // a < c <= b
			return c
		}
		return a // c <= a < b
	} else {
		if less(x[a], x[c]) { // b <= a < c
			return a
		} else if less(x[b], x[c]) { // b < c <= a
			return c
		}
		return b // c <= b <= a
	}
}
