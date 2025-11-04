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

	"github.com/stretchr/testify/require"
)

type Int int

func (a Int) Less(b Int) bool {
	return a < b
}

func expectPop(t *testing.T, q *PriorityQueue[Int], expected Int) {
	t.Helper()
	require.False(t, q.Empty())
	require.Equal(t, expected, q.Peek())
	require.Equal(t, expected, q.Pop())
}

func TestNewPriorityQueue(t *testing.T) {
	q := NewPriorityQueue([]Int{4, 3, 5, 1, 2})

	expectPop(t, q, Int(1))
	expectPop(t, q, Int(2))
	expectPop(t, q, Int(3))
	expectPop(t, q, Int(4))
	expectPop(t, q, Int(5))
	require.True(t, q.Empty())
}

func TestNewEmptyPriorityQueue(t *testing.T) {
	q := NewEmptyPriorityQueue[Int]()
	require.True(t, q.Empty())

	for i := Int(5); i >= 1; i-- {
		q.Push(i)
	}

	expectPop(t, q, Int(1))
	expectPop(t, q, Int(2))
	expectPop(t, q, Int(3))
	expectPop(t, q, Int(4))
	expectPop(t, q, Int(5))
	require.True(t, q.Empty())
}
