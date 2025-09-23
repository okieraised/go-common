package sortutils

func mergeSort[T any](x []T, less func(a, b T) bool) {
	n := len(x)
	if n <= 1 {
		return
	}
	buf := make([]T, n)
	// Bottom-up merge sort for good locality and no recursion.
	for width := 1; width < n; width *= 2 {
		for lo := 0; lo < n; lo += 2 * width {
			mid := lo + width
			hi := lo + 2*width
			if mid > n {
				mid = n
			}
			if hi > n {
				hi = n
			}
			merge(x, buf, less, lo, mid, hi)
		}
		// Copy back
		copy(x, buf)
	}
}

func merge[T any](x, out []T, less func(a, b T) bool, lo, mid, hi int) {
	i, j, k := lo, mid, lo
	for i < mid && j < hi {
		// Stable: pick left when equal
		if !less(x[j], x[i]) {
			out[k] = x[i]
			i++
		} else {
			out[k] = x[j]
			j++
		}
		k++
	}
	for i < mid {
		out[k] = x[i]
		i++
		k++
	}
	for j < hi {
		out[k] = x[j]
		j++
		k++
	}
}
