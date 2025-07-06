package utils

import (
	"testing"
)

func TestForEach(t *testing.T) {
	var sum int
	ForEach([]int {1, 2, 3}, func(n int) {
		sum += n
	})

	if sum != 6 {
		t.Errorf("ForEach failed, expected sum 6, got %d", sum)
	}
}

func TestMap(t *testing.T) {
	input := []int{1, 2, 3}
	expect := []int{2, 4, 6}
	result := Map(input, func(n int) int {
		return n * 2
	})

	for i, v := range expect {
		if v != result[i] {
			t.Errorf("Map failed at index %d expected %d, got %d", i, v, result[i])
		}
	}
}

func TestFilter(t *testing.T) {
	input := []int{1, 2, 3, 4}
	expect := []int{2, 4}
	result := Filter(input, func(n int) bool {
		return n % 2 == 0
	})

	for i, v := range expect {
		if v != result[i] {
			t.Errorf("Filter faild at index %d expected %d, got %d", i, v, result[i])
		}
	}
}