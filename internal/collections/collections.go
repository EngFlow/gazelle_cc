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

// Package collections provides functional programming utilities for working
// with Go sequences and slices.
//
// The package includes a generic Set type for mathematical set operations and
// efficient membership testing.
//
// This package leverages Go's iter.Seq type to provide efficient, composable
// operations on both sequences and slices. Each operation comes in two
// variants: one for sequences (Seq suffix) and one for slices (Slice suffix).
package collections

import (
	"iter"
	"slices"
)

// MapSeq applies the provided transformation function `fn` to each element of
// the input sequence `seq` and returns a new sequence of the resulting values.
//
// Example:
//
//	MapSeq(
//		slices.Values([]int{1, 2, 3}),
//		func(x int) string { return fmt.Sprint(x) }
//	)
//	=> sequence of []string{"1", "2", "3"}
func MapSeq[T, V any](seq iter.Seq[T], fn func(T) V) iter.Seq[V] {
	return func(yield func(V) bool) {
		for t := range seq {
			if !yield(fn(t)) {
				return
			}
		}
	}
}

// MapSlice applies the provided transformation function `fn` to each element of
// the input slice `s` and returns a new slice of the resulting values.
//
// Example:
//
//	MapSlice([]int{1, 2, 3}, func(x int) string { return fmt.Sprint(x) })
//	=> []string{"1", "2", "3"}
func MapSlice[TSlice ~[]T, T, V any](s TSlice, fn func(T) V) []V {
	return slices.AppendSeq(make([]V, 0, len(s)), MapSeq(slices.Values(s), fn))
}

// FilterSeq returns a new sequence containing only the elements of `seq` for
// which the `predicate` function returns true.
//
// Example:
//
//	FilterSeq(slices.Values(
//		[]int{1, 2, 3, 4}),
//		func(x int) bool { return x%2 == 0 }
//	)
//	=> sequence of []int{2, 4}
func FilterSeq[T any](seq iter.Seq[T], predicate func(T) bool) iter.Seq[T] {
	return func(yield func(T) bool) {
		for elem := range seq {
			if predicate(elem) && !yield(elem) {
				return
			}
		}
	}
}

// FilterSlice returns a new slice containing only the elements of `s` for which
// the `predicate` function returns true.
//
// Example:
//
//	FilterSlice([]int{1, 2, 3, 4}, func(x int) bool { return x%2 == 0 })
//	=> []int{2, 4}
func FilterSlice[TSlice ~[]T, T any](s TSlice, predicate func(T) bool) TSlice {
	return slices.AppendSeq(make(TSlice, 0, len(s)), FilterSeq(slices.Values(s), predicate))
}

// FlatMapSeq applies the provided transformation function `fn` to each element
// of the input sequence `seq`, where `fn` returns a slice, and flattens the
// resulting slices into a single sequence.
//
// Example:
//
//	FlatMapSeq(
//		slices.Values([]int{1, 2}),
//		func(x int) []int { return []int{x, x} }
//	)
//	=> sequence of []int{1, 1, 2, 2}
func FlatMapSeq[VSlice ~[]V, T, V any](seq iter.Seq[T], fn func(T) VSlice) iter.Seq[V] {
	return func(yield func(V) bool) {
		for t := range seq {
			for _, v := range fn(t) {
				if !yield(v) {
					return
				}
			}
		}
	}
}

// FlatMapSlice applies the provided transformation function `fn` to each
// element of the input slice `s`, where `fn` returns a slice, and flattens the
// resulting slices into a single slice.
//
// Example:
//
//	FlatMapSlice(
//		[]int{1, 2},
//		func(x int) []int { return []int{x, x} }
//	)
//	=> []int{1, 1, 2, 2}
func FlatMapSlice[TSlice ~[]T, VSlice ~[]V, T, V any](s TSlice, fn func(T) VSlice) VSlice {
	return slices.Collect(FlatMapSeq(slices.Values(s), fn))
}

// FilterMapSeq applies a transformation function `fn` to each element of the
// input sequence `seq`, where `fn` returns both a transformed value and a
// boolean indicating success. Returns a new sequence containing only the
// successfully transformed values.
//
// Example:
//
//	FilterMapSeq(
//		slices.Values([]int{1, -1, 2}),
//		func(x int) (int, bool) {
//			if x < 0 { return 0, false }
//			return x * 2, true
//		}
//	)
//	=> sequence of []int{2, 4}
func FilterMapSeq[T, V any](seq iter.Seq[T], fn func(T) (V, bool)) iter.Seq[V] {
	type pair struct {
		value V
		ok    bool
	}

	pairReturner := func(t T) pair { v, ok := fn(t); return pair{value: v, ok: ok} }
	valueGetter := func(p pair) V { return p.value }
	okGetter := func(p pair) bool { return p.ok }

	return MapSeq(FilterSeq(MapSeq(seq, pairReturner), okGetter), valueGetter)
}

// FilterMapSlice applies a transformation function `fn` to each element of the
// input slice `s`, where `fn` returns both a transformed value and a boolean
// indicating success. Returns a new slice containing only the successfully
// transformed values.
//
// Example:
//
//	FilterMapSlice(
//		[]int{1, -1, 2},
//		func(x int) (int, bool) {
//			if x < 0 { return 0, false }
//			return x * 2, true
//		}
//	)
//	=> []int{2, 4}
func FilterMapSlice[TSlice ~[]T, T, V any](s TSlice, fn func(T) (V, bool)) []V {
	return slices.AppendSeq(make([]V, 0, len(s)), FilterMapSeq(slices.Values(s), fn))
}
