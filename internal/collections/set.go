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

package collections

import (
	"iter"
	"maps"
	"slices"
)

// Set is a generic implementation of a mathematical set for comparable types.
// It is implemented as a map with empty struct values for minimal memory usage.
type Set[T comparable] map[T]struct{}

// SetOf creates a new Set containing the given elements.
// It is a shorthand for ToSet with variadic arguments.
func SetOf[T comparable](elems ...T) Set[T] {
	return ToSet(elems)
}

// ToSet converts a slice into a Set, eliminating duplicates.
func ToSet[T comparable](slice []T) Set[T] {
	return make(Set[T], len(slice)).AddSlice(slice)
}

// FindDuplicates returns a slice of elements that appear more than once in the
// input slice or nil if there are no duplicates. The order follows the second
// occurrence of each duplicate.
func FindDuplicates[S ~[]T, T comparable](slice S) S {
	var result S
	seen := make(Set[T])
	for _, elem := range slice {
		if seen.Contains(elem) {
			result = append(result, elem)
		} else {
			seen.Add(elem)
		}
	}
	return result
}

// Diff returns a new Set containing elements that are defined in current Set
// but not in the other set.
func (s Set[T]) Diff(other Set[T]) Set[T] {
	diff := make(Set[T])
	for elem := range s {
		if !other.Contains(elem) {
			diff.Add(elem)
		}
	}
	return diff
}

// Add inserts an element into the Set.
// Returns the Set to allow chaining.
func (s Set[T]) Add(elem T) Set[T] {
	s[elem] = struct{}{}
	return s
}

// AddSeq inserts all elements from the given sequence to the Set.
// Returns the Set to allow chaining.
func (s Set[T]) AddSeq(elems iter.Seq[T]) Set[T] {
	for elem := range elems {
		s.Add(elem)
	}
	return s
}

// AddSlice inserts all elements from the given slice to the Set.
// Returns the Set to allow chaining.
func (s Set[T]) AddSlice(elems []T) Set[T] {
	return s.AddSeq(slices.Values(elems))
}

// Contains checks whether an element exists in the Set.
func (s Set[T]) Contains(elem T) bool {
	_, exists := s[elem]
	return exists
}

// Join adds all elements from another Set into the current Set (union).
// Returns the modified Set to allow chaining.
func (s Set[T]) Join(other Set[T]) Set[T] {
	for elem := range other {
		s.Add(elem)
	}
	return s
}

// Intersect returns a new Set containing only elements present in both Sets.
func (s Set[T]) Intersect(other Set[T]) Set[T] {
	result := make(Set[T])
	for elem := range s {
		if other.Contains(elem) {
			result.Add(elem)
		}
	}
	return result
}

// Intersects returns true if there is at least one common element between the
// Sets.
func (s Set[T]) Intersects(other Set[T]) bool {
	for elem := range s {
		if other.Contains(elem) {
			return true
		}
	}
	return false
}

// All returns a sequence containing all elements in the Set. The order is not
// guaranteed.
func (s Set[T]) All() iter.Seq[T] {
	return maps.Keys(s)
}

// Values returns a slice containing all elements in the Set.
// The order is not guaranteed. For guaranteed order, use SortedValues.
func (s Set[T]) Values() []T {
	return slices.Collect(s.All())
}

// SortedValues returns a sorted slice containing all elements in the Set.
func (s Set[T]) SortedValues(cmp func(l, r T) int) []T {
	return slices.SortedFunc(s.All(), cmp)
}
