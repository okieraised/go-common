package sortutils

func heapSort[T any](x []T, less func(a, b T) bool) {
	n := len(x)
	// Build max-heap using sift-down
	for i := n/2 - 1; i >= 0; i-- {
		siftDown(x, less, i, n)
	}
	// Extract
	for end := n - 1; end > 0; end-- {
		x[0], x[end] = x[end], x[0]
		siftDown(x, less, 0, end)
	}
}

func siftDown[T any](x []T, less func(a, b T) bool, root, end int) {
	for {
		l := 2*root + 1
		if l >= end {
			return
		}
		lMax := l
		r := l + 1
		if r < end && less(x[lMax], x[r]) {
			lMax = r
		}
		if !less(x[root], x[lMax]) {
			return
		}
		x[root], x[lMax] = x[lMax], x[root]
		root = lMax
	}
}
