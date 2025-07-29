// Copyright 2025 EngFlow Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package collections

import (
	"slices"
)

// Map applies the provided transformation function `fn` to each element of the input slice `ts`
// and returns a new slice of the resulting values.
//
// Example:
//
//	Map([]int{1, 2, 3}, func(x int) string { return fmt.Sprint(x) })
//	=> []string{"1", "2", "3"}
func Map[T, V any](ts []T, fn func(T) V) []V {
	result := make([]V, len(ts))
	for i, t := range ts {
		result[i] = fn(t)
	}
	return result
}

// FlatMap applies the function `fn` to each element of the input slice `ts`,
// where `fn` returns a slice, and flattens the resulting slices into a single slice.
//
// Example:
//
//	FlatMap([]int{1, 2}, func(x int) []int { return []int{x, x} })
//	=> []int{1, 1, 2, 2}
func FlatMap[T, V any](ts []T, fn func(T) []V) []V {
	result := []V{}
	for _, t := range ts {
		result = slices.AppendSeq(result, slices.Values(fn(t)))
	}
	return result
}

// Collect applies the function `fn` to each element of the input slice `ts`,
// collecting only the successfully transformed values (i.e., those that do not return an error).
//
// Example:
//
//	FilterMap([]int{1, -1, 2}, func(x int) (int, error) {
//	    if x < 0 { return 0, false }
//	    return x * 2, true
//	})
//	=> []int{2, 4}
func FilterMap[T, V any](ts []T, fn func(T) (V, bool)) []V {
	result := []V{}
	for _, t := range ts {
		if transformed, ok := fn(t); ok {
			result = append(result, transformed)
		}
	}
	return result
}

// Filter returns a new slice containing only the elements of `ts` for which
// the `predicate` function returns true.
//
// Example:
//
//	Filter([]int{1, 2, 3, 4}, func(x int) bool { return x%2 == 0 })
//	=> []int{2, 4}
func Filter[T any](ts []T, predicate func(T) bool) []T {
	result := make([]T, 0, len(ts))
	for _, elem := range ts {
		if predicate(elem) {
			result = append(result, elem)
		}
	}
	return result
}
