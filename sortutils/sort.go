package sortutils

import "cmp"

type Algorithm int

const (
	QuickSort     Algorithm = iota // Unstable, in-place, average O(n log n)
	MergeSort                      // Stable, O(n log n), extra memory
	HeapSort                       // Unstable, in-place, O(n log n)
	InsertionSort                  // Stable, good for small/near-sorted, O(n^2)
	SelectionSort                  // Unstable, O(n^2)
	BubbleSort                     // Stable (with swap flag), O(n^2)
)

// Sort sorts x using the chosen algorithm with natural (ordered) comparison.
func Sort[T cmp.Ordered](x []T, algo Algorithm) {
	SortFunc(x, func(a, b T) bool { return a < b }, algo)
}

// SortFunc sorts x using the chosen algorithm with a custom less function.
// less(a,b) should return true if a < b.
func SortFunc[T any](x []T, less func(a, b T) bool, algo Algorithm) {
	switch algo {
	case QuickSort:
		quickSort(x, less, 0, len(x)-1)
	case MergeSort:
		mergeSort(x, less)
	case HeapSort:
		heapSort(x, less)
	case InsertionSort:
		insertionSort(x, less)
	case SelectionSort:
		selectionSort(x, less)
	case BubbleSort:
		bubbleSort(x, less)
	default:
		quickSort(x, less, 0, len(x)-1)
	}
}
