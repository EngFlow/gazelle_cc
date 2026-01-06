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
	"fmt"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMapSlice(t *testing.T) {
	input := []int{1, 2, 3}
	expected := []string{"1", "2", "3"}

	result := MapSlice(input, func(i int) string {
		return string(rune('0' + i))
	})

	assert.Equal(t, expected, result)
}

func TestFlatMapSlice(t *testing.T) {
	input := []int{1, 2}
	expected := []int{1, 1, 2, 2}

	result := FlatMapSlice(input, func(i int) []int {
		return []int{i, i}
	})

	assert.Equal(t, expected, result)
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

	assert.Equal(t, expected, result)
}

func TestFilterSlice(t *testing.T) {
	input := []int{1, 2, 3, 4}
	expected := []int{2, 4}

	result := FilterSlice(input, func(i int) bool {
		return i%2 == 0
	})

	assert.Equal(t, expected, result)
}

func TestConcatSeq(t *testing.T) {
	input1 := []string{"a", "b"}
	input2 := []string{"c", "d"}
	expected := []string{"a", "b", "c", "d"}
	result := slices.Collect(ConcatSeq(slices.Values(input1), slices.Values(input2)))
	assert.Equal(t, expected, result)
}

func TestCountRepeatsInSlice(t *testing.T) {
	input := []string{"a", "b", "a", "c", "b", "a"}
	expected := map[string]int{
		"a": 3,
		"b": 2,
		"c": 1,
	}
	result := CountRepeatsInSlice(input)
	assert.Equal(t, expected, result)
}

func ExampleMapSeq() {
	seq := MapSeq(
		slices.Values([]int{1, 2, 3}),
		func(x int) string { return fmt.Sprint(x) },
	)
	fmt.Println(slices.Collect(seq))
	// Output: [1 2 3]
}

func ExampleMapSlice() {
	result := MapSlice([]int{1, 2, 3}, func(x int) string { return fmt.Sprint(x) })
	fmt.Println(result)
	// Output: [1 2 3]
}

func ExampleFilterSeq() {
	seq := FilterSeq(
		slices.Values([]int{1, 2, 3, 4}),
		func(x int) bool { return x%2 == 0 },
	)
	fmt.Println(slices.Collect(seq))
	// Output: [2 4]
}

func ExampleFilterSlice() {
	result := FilterSlice([]int{1, 2, 3, 4}, func(x int) bool { return x%2 == 0 })
	fmt.Println(result)
	// Output: [2 4]
}

func ExampleFlatMapSeq() {
	seq := FlatMapSeq(
		slices.Values([]int{1, 2}),
		func(x int) []int { return []int{x, x} },
	)
	fmt.Println(slices.Collect(seq))
	// Output: [1 1 2 2]
}

func ExampleFlatMapSlice() {
	result := FlatMapSlice(
		[]int{1, 2},
		func(x int) []int { return []int{x, x} },
	)
	fmt.Println(result)
	// Output: [1 1 2 2]
}

func ExampleFilterMapSeq() {
	seq := FilterMapSeq(
		slices.Values([]int{1, -1, 2}),
		func(x int) (int, bool) {
			if x < 0 {
				return 0, false
			}
			return x * 2, true
		},
	)
	fmt.Println(slices.Collect(seq))
	// Output: [2 4]
}

func ExampleFilterMapSlice() {
	result := FilterMapSlice(
		[]int{1, -1, 2},
		func(x int) (int, bool) {
			if x < 0 {
				return 0, false
			}
			return x * 2, true
		},
	)
	fmt.Println(result)
	// Output: [2 4]
}

func ExampleConcatSeq() {
	seq := ConcatSeq(
		slices.Values([]string{"a", "b"}),
		slices.Values([]string{"c", "d"}),
	)
	fmt.Println(slices.Collect(seq))
	// Output: [a b c d]
}

func ExampleCountRepeatsInSeq() {
	seq := slices.Values([]string{"a", "b", "a", "c", "b", "a"})
	counts := CountRepeatsInSeq(seq)
	fmt.Println(counts)
	// Output: map[a:3 b:2 c:1]
}

func ExampleCountRepeatsInSlice() {
	slice := []string{"a", "b", "a", "c", "b", "a"}
	counts := CountRepeatsInSlice(slice)
	fmt.Println(counts)
	// Output: map[a:3 b:2 c:1]
}
