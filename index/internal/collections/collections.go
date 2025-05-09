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
	"maps"
	"slices"
)

func Map[T, V any](ts []T, fn func(T) V) []V {
	result := make([]V, len(ts))
	for i, t := range ts {
		result[i] = fn(t)
	}
	return result
}

func FlatMap[T, V any](ts []T, fn func(T) []V) []V {
	result := []V{}
	for _, t := range ts {
		result = slices.AppendSeq(result, slices.Values(fn(t)))
	}
	return result
}

func Collect[T, V any](ts []T, fn func(T) (V, error)) []V {
	result := []V{}
	for _, t := range ts {
		transformed, err := fn(t)
		if err == nil {
			result = append(result, transformed)
		}
	}
	return result
}

func Find[T any](ts []T, predicate func(T) bool) *T {
	for _, t := range ts {
		if predicate(t) {
			return &t
		}
	}
	return nil
}

func Filter[T any](ts []T, predicate func(T) bool) []T {
	result := make([]T, 0, len(ts))
	for _, elem := range ts {
		if predicate(elem) {
			result = append(result, elem)
		}
	}
	return result
}

type Set[T comparable] map[T]bool

func ToSet[T comparable](slice []T) Set[T] {
	set := make(Set[T])
	for _, elem := range slice {
		set[elem] = true
	}
	return set
}

func (s Set[T]) Diff(other Set[T]) Set[T] {
	diff := make(Set[T])
	for elem := range other {
		if _, exists := (s)[elem]; !exists {
			diff[elem] = true
		}
	}
	return diff
}

func (s *Set[T]) Add(elem T) *Set[T] {
	(*s)[elem] = true
	return s
}

func (s *Set[T]) Join(other Set[T]) *Set[T] {
	for elem := range other {
		s.Add(elem)
	}
	return s
}

func (s Set[T]) Intersect(other Set[T]) Set[T] {
	result := make(Set[T])
	for elem := range s {
		if (other)[elem] {
			result[elem] = true
		}
	}
	return result
}

func (s Set[T]) Intersects(other Set[T]) bool {
	for elem := range s {
		if (other)[elem] {
			return true
		}
	}
	return false
}

func (s Set[T]) Values() []T {
	return slices.Collect(maps.Keys(s))
}
