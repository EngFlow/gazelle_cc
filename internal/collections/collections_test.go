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

import "testing"

func TestMapSlice(t *testing.T) {
	input := []int{1, 2, 3}
	expected := []string{"1", "2", "3"}

	result := MapSlice(input, func(i int) string {
		return string(rune('0' + i))
	})

	for i := range expected {
		if result[i] != expected[i] {
			t.Errorf("MapSlice failed at index %d: expected %v, got %v", i, expected[i], result[i])
		}
	}
}

func TestFlatMapSlice(t *testing.T) {
	input := []int{1, 2}
	expected := []int{1, 1, 2, 2}

	result := FlatMapSlice(input, func(i int) []int {
		return []int{i, i}
	})

	if len(result) != len(expected) {
		t.Fatalf("FlatMapSlice length mismatch: expected %d, got %d", len(expected), len(result))
	}

	for i := range expected {
		if result[i] != expected[i] {
			t.Errorf("FlatMapSlice failed at index %d: expected %d, got %d", i, expected[i], result[i])
		}
	}
}

func TestFilterMapSlice(t *testing.T) {
	input := []int{1, -1, 2}
	expected := []int{2, 4}

	result := FilterMapSlice(input, func(i int) (int, bool) {
		if i < 0 {
			return 0, false
		}
		return i * 2, true
	})

	if len(result) != len(expected) {
		t.Fatalf("Collect length mismatch: expected %d, got %d", len(expected), len(result))
	}

	for i := range expected {
		if result[i] != expected[i] {
			t.Errorf("Collect failed at index %d: expected %d, got %d", i, expected[i], result[i])
		}
	}
}

func TestFilterSlice(t *testing.T) {
	input := []int{1, 2, 3, 4}
	expected := []int{2, 4}

	result := FilterSlice(input, func(i int) bool {
		return i%2 == 0
	})

	if len(result) != len(expected) {
		t.Fatalf("Filter length mismatch: expected %d, got %d", len(expected), len(result))
	}

	for i := range expected {
		if result[i] != expected[i] {
			t.Errorf("Filter failed at index %d: expected %d, got %d", i, expected[i], result[i])
		}
	}
}
