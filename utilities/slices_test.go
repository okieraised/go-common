package utilities

import (
	"fmt"
	"testing"
)

func TestIsSubset(t *testing.T) {
	superset := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	subset := []int{1, 3, 5, 7}

	fmt.Println(IsSubset(subset, superset))
}
