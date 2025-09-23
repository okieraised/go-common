package sortutils

func selectionSort[T any](x []T, less func(a, b T) bool) {
	n := len(x)
	for i := 0; i < n; i++ {
		minIdx := i
		for j := i + 1; j < n; j++ {
			if less(x[j], x[minIdx]) {
				minIdx = j
			}
		}
		x[i], x[minIdx] = x[minIdx], x[i]
	}
}
