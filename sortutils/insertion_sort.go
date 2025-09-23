package sortutils

func insertionSort[T any](x []T, less func(a, b T) bool) {
	for i := 1; i < len(x); i++ {
		v := x[i]
		j := i - 1
		for j >= 0 && less(v, x[j]) {
			x[j+1] = x[j]
			j--
		}
		x[j+1] = v
	}
}
