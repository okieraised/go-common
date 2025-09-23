package sortutils

func bubbleSort[T any](x []T, less func(a, b T) bool) {
	n := len(x)
	for {
		swapped := false
		for i := 1; i < n; i++ {
			if less(x[i], x[i-1]) {
				x[i], x[i-1] = x[i-1], x[i]
				swapped = true
			}
		}
		n--
		if !swapped || n <= 1 {
			break
		}
	}
}
