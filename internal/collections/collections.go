// Copyright 2026 EngFlow Inc. All rights reserved.
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
func MapSlice[TSlice ~[]T, T, V any](s TSlice, fn func(T) V) []V {
	return slices.AppendSeq(make([]V, 0, len(s)), MapSeq(slices.Values(s), fn))
}

// FilterSeq returns a new sequence containing only the elements of `seq` for
// which the `predicate` function returns true.
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
func FilterSlice[TSlice ~[]T, T any](s TSlice, predicate func(T) bool) TSlice {
	return slices.AppendSeq(make(TSlice, 0, len(s)), FilterSeq(slices.Values(s), predicate))
}

// FlatMapSeq applies the provided transformation function `fn` to each element
// of the input sequence `seq`, where `fn` returns a slice, and flattens the
// resulting slices into a single sequence.
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
func FlatMapSlice[TSlice ~[]T, VSlice ~[]V, T, V any](s TSlice, fn func(T) VSlice) VSlice {
	return slices.Collect(FlatMapSeq(slices.Values(s), fn))
}

// FilterMapSeq applies a transformation function `fn` to each element of the
// input sequence `seq`, where `fn` returns both a transformed value and a
// boolean indicating success. Returns a new sequence containing only the
// successfully transformed values.
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
func FilterMapSlice[TSlice ~[]T, T, V any](s TSlice, fn func(T) (V, bool)) []V {
	return slices.AppendSeq(make([]V, 0, len(s)), FilterMapSeq(slices.Values(s), fn))
}

// ConcatSeq concatenates multiple input sequences into a single sequence.
func ConcatSeq[T any](seqs ...iter.Seq[T]) iter.Seq[T] {
	return func(yield func(T) bool) {
		for _, seq := range seqs {
			for elem := range seq {
				if !yield(elem) {
					return
				}
			}
		}
	}
}

// Maps every element in the input sequence to the number of times it appears in
// the sequence.
func CountRepeatsInSeq[T comparable](seq iter.Seq[T]) map[T]int {
	counts := make(map[T]int)
	for elem := range seq {
		counts[elem]++
	}
	return counts
}

// Maps every element in the input slice to the number of times it appears in
// the slice.
func CountRepeatsInSlice[T comparable](slice []T) map[T]int {
	return CountRepeatsInSeq(slices.Values(slice))
}

// Finds all elements that appear more than once in the input sequence and
// returns a Set of them.
func FindDuplicatesInSeq[T comparable](seq iter.Seq[T]) Set[T] {
	duplicates := make(Set[T])
	for elem, count := range CountRepeatsInSeq(seq) {
		if count > 1 {
			duplicates.Add(elem)
		}
	}
	return duplicates
}

// Finds all elements that appear more than once in the input slice and
// returns a Set of them.
func FindDuplicatesInSlice[T comparable](slice []T) Set[T] {
	return FindDuplicatesInSeq(slices.Values(slice))
}
