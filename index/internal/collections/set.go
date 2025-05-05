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

// Set is a generic implementation of a mathematical set for comparable types.
// It is implemented as a map with empty struct values for minimal memory usage.
type Set[T comparable] map[T]struct{}

// SetOf creates a new Set containing the given elements.
// It is a shorthand for ToSet with variadic arguments.
//
// Example:
//
//	s := SetOf(1, 2, 3)
func SetOf[T comparable](elems ...T) Set[T] {
	return ToSet(elems)
}

// ToSet converts a slice into a Set, eliminating duplicates.
//
// Example:
//
//	s := ToSet([]string{"a", "b", "a"})
//	=> Set[string]{"a": {}, "b": {}}
func ToSet[T comparable](slice []T) Set[T] {
	set := make(Set[T])
	for _, elem := range slice {
		set.Add(elem)
	}
	return set
}

// Diff returns a new Set containing elements that are in `other` but not in the current Set.
//
// Example:
//
//	a := SetOf(1, 2, 3)
//	b := SetOf(2, 3, 4)
//	diff := a.Diff(b)
//	=> Set[int]{4}
func (s Set[T]) Diff(other Set[T]) Set[T] {
	diff := make(Set[T])
	for elem := range other {
		if _, exists := s[elem]; !exists {
			diff.Add(elem)
		}
	}
	return diff
}

// Add inserts an element into the Set.
// Returns the Set to allow chaining.
//
// Example:
//
//	s := SetOf(1).Add(2).Add(3)
func (s *Set[T]) Add(elem T) *Set[T] {
	(*s)[elem] = struct{}{}
	return s
}

// Contains checks whether an element exists in the Set.
//
// Example:
//
//	s := SetOf("apple", "banana")
//	s.Contains("banana") => true
func (s *Set[T]) Contains(elem T) bool {
	_, exists := (*s)[elem]
	return exists
}

// Join adds all elements from another Set into the current Set (union).
// Returns the modified Set to allow chaining.
//
// Example:
//
//	a := SetOf(1, 2)
//	b := SetOf(2, 3)
//	a.Join(b) => Set[int]{1, 2, 3}
func (s *Set[T]) Join(other Set[T]) *Set[T] {
	for elem := range other {
		s.Add(elem)
	}
	return s
}

// Intersect returns a new Set containing only elements present in both Sets.
//
// Example:
//
//	a := SetOf(1, 2, 3)
//	b := SetOf(2, 3, 4)
//	a.Intersect(b) => Set[int]{2, 3}
func (s Set[T]) Intersect(other Set[T]) Set[T] {
	result := make(Set[T])
	for elem := range s {
		if _, exists := other[elem]; exists {
			result.Add(elem)
		}
	}
	return result
}

// Intersects returns true if there is at least one common element between the Sets.
//
// Example:
//
//	a := SetOf("x", "y")
//	b := SetOf("y", "z")
//	a.Intersects(b) => true
func (s Set[T]) Intersects(other Set[T]) bool {
	for elem := range s {
		if _, exists := other[elem]; exists {
			return true
		}
	}
	return false
}

// Values returns a slice containing all elements in the Set.
// The order is not guaranteed.
//
// Example:
//
//	s := SetOf("a", "b")
//	vals := s.Values() => []string{"a", "b"} (order may vary)
func (s Set[T]) Values() []T {
	return slices.Collect(maps.Keys(s))
}
