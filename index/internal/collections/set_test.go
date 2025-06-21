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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetOf(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		expected Set[int]
	}{
		{
			name:     "no arguments",
			input:    []int{},
			expected: Set[int]{},
		},
		{
			name:     "single argument",
			input:    []int{1},
			expected: Set[int]{1: struct{}{}},
		},
		{
			name:     "multiple arguments",
			input:    []int{1, 2, 3},
			expected: Set[int]{1: struct{}{}, 2: struct{}{}, 3: struct{}{}},
		},
		{
			name:     "duplicate arguments",
			input:    []int{1, 2, 2, 3, 3, 3},
			expected: Set[int]{1: struct{}{}, 2: struct{}{}, 3: struct{}{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SetOf(tt.input...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToSet(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		expected Set[int]
	}{
		{
			name:     "empty slice",
			input:    []int{},
			expected: SetOf[int](),
		},
		{
			name:     "single element",
			input:    []int{1},
			expected: SetOf(1),
		},
		{
			name:     "multiple elements",
			input:    []int{1, 2, 3},
			expected: SetOf(1, 2, 3),
		},
		{
			name:     "duplicate elements",
			input:    []int{1, 2, 2, 3, 3, 3},
			expected: SetOf(1, 2, 3),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToSet(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSet_Add(t *testing.T) {
	tests := []struct {
		name     string
		set      Set[int]
		elem     int
		expected Set[int]
	}{
		{
			name:     "add to empty set",
			set:      SetOf[int](),
			elem:     1,
			expected: SetOf(1),
		},
		{
			name:     "add to non-empty set",
			set:      SetOf(1),
			elem:     2,
			expected: SetOf(1, 2),
		},
		{
			name:     "add existing element",
			set:      SetOf(1),
			elem:     1,
			expected: SetOf(1),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.set.Add(tt.elem)
			assert.Equal(t, tt.expected, *result)
		})
	}
}

func TestSet_Contains(t *testing.T) {
	tests := []struct {
		name     string
		set      Set[int]
		elem     int
		expected bool
	}{
		{
			name:     "empty set",
			set:      SetOf[int](),
			elem:     1,
			expected: false,
		},
		{
			name:     "element exists",
			set:      SetOf(1),
			elem:     1,
			expected: true,
		},
		{
			name:     "element does not exist",
			set:      SetOf(1),
			elem:     2,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.set.Contains(tt.elem)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSet_Diff(t *testing.T) {
	tests := []struct {
		name     string
		set1     Set[int]
		set2     Set[int]
		expected Set[int]
	}{
		{
			name:     "empty sets",
			set1:     SetOf[int](),
			set2:     SetOf[int](),
			expected: SetOf[int](),
		},
		{
			name:     "no difference",
			set1:     SetOf(1, 2),
			set2:     SetOf(1, 2),
			expected: SetOf[int](),
		},
		{
			name:     "some difference",
			set1:     SetOf(1, 2),
			set2:     SetOf(1, 3),
			expected: SetOf(2),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.set1.Diff(tt.set2)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSet_Join(t *testing.T) {
	tests := []struct {
		name     string
		set1     Set[int]
		set2     Set[int]
		expected Set[int]
	}{
		{
			name:     "empty sets",
			set1:     SetOf[int](),
			set2:     SetOf[int](),
			expected: SetOf[int](),
		},
		{
			name:     "disjoint sets",
			set1:     SetOf(1),
			set2:     SetOf(2),
			expected: SetOf(1, 2),
		},
		{
			name:     "overlapping sets",
			set1:     SetOf(1, 2),
			set2:     SetOf(2, 3),
			expected: SetOf(1, 2, 3),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.set1.Join(tt.set2)
			assert.Equal(t, tt.expected, *result)
		})
	}
}

func TestSet_Intersect(t *testing.T) {
	tests := []struct {
		name     string
		set1     Set[int]
		set2     Set[int]
		expected Set[int]
	}{
		{
			name:     "empty sets",
			set1:     SetOf[int](),
			set2:     SetOf[int](),
			expected: SetOf[int](),
		},
		{
			name:     "no intersection",
			set1:     SetOf(1),
			set2:     SetOf(2),
			expected: SetOf[int](),
		},
		{
			name:     "some intersection",
			set1:     SetOf(1, 2),
			set2:     SetOf(2, 3),
			expected: SetOf(2),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.set1.Intersect(tt.set2)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSet_Intersects(t *testing.T) {
	tests := []struct {
		name     string
		set1     Set[int]
		set2     Set[int]
		expected bool
	}{
		{
			name:     "empty sets",
			set1:     SetOf[int](),
			set2:     SetOf[int](),
			expected: false,
		},
		{
			name:     "no intersection",
			set1:     SetOf(1),
			set2:     SetOf(2),
			expected: false,
		},
		{
			name:     "has intersection",
			set1:     SetOf(1, 2),
			set2:     SetOf(2, 3),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.set1.Intersects(tt.set2)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSet_Values(t *testing.T) {
	tests := []struct {
		name     string
		set      Set[int]
		expected []int
	}{
		{
			name:     "empty set",
			set:      SetOf[int](),
			expected: []int{},
		},
		{
			name:     "single element",
			set:      SetOf(1),
			expected: []int{1},
		},
		{
			name:     "multiple elements",
			set:      SetOf(1, 2, 3),
			expected: []int{1, 2, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.set.Values()
			// Sort the result since the order is not guaranteed
			assert.ElementsMatch(t, tt.expected, result)
		})
	}
}
