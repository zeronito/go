// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

import (
	"math"
	"sort"
)

// Mean returns the mean (average) of a slice of float64 numbers.
//
// Special cases are:
//
//	Mean([]float64{}) = 0
func Mean(numbers []float64) float64 {
	if len(numbers) == 0 {
		return 0
	}

	var sum float64
	for _, num := range numbers {
		sum += num
	}

	return sum / float64(len(numbers))
}

// Median returns the median of a slice of float64 numbers.
//
// Special cases are:
//
//	Median([]float64{}) = 0
func Median(numbers []float64) float64 {
	if len(numbers) == 0 {
		return 0
	}

	sorted := make([]float64, len(numbers))
	copy(sorted, numbers)
	sort.Float64s(sorted)

	n := len(sorted)
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return sorted[n/2]
}

// Mode returns the mode(s) of a slice of float64 numbers.
//
// Special cases are:
//
//	Mode([]float64{}) = nil
func Mode(numbers []float64) []float64 {
	if len(numbers) == 0 {
		return nil
	}

	counts := make(map[float64]int)
	for _, num := range numbers {
		counts[num]++
	}

	var maxCount int
	for _, count := range counts {
		if count > maxCount {
			maxCount = count
		}
	}

	var modes []float64
	for num, count := range counts {
		if count == maxCount {
			modes = append(modes, num)
		}
	}

	return modes
}

// Variance returns the variance of a slice of float64 numbers.
//
// Special cases are:
//
//	Variance([]float64{}) = 0
func Variance(numbers []float64) float64 {
	if len(numbers) == 0 {
		return 0
	}

	mean := Mean(numbers)
	var sumSquares float64
	for _, num := range numbers {
		diff := num - mean
		sumSquares += diff * diff
	}

	return sumSquares / float64(len(numbers))
}

// StdDev returns the standard deviation of a slice of float64 numbers.
//
// Special cases are:
//
//	StdDev([]float64{}) = 0
func StdDev(numbers []float64) float64 {
	return math.Sqrt(Variance(numbers))
}