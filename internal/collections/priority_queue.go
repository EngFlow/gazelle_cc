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

import "container/heap"

type (
	// Describes a type that can be ordered with Strict Weak Ordering.
	//
	// See: https://en.wikipedia.org/wiki/Weak_ordering#Strict_weak_orderings
	Ordered[T any] interface {
		// Less reports whether this element must sort before the other element.
		Less(T) bool
	}

	// A thin wrapper around a slice to implement heap.Interface.
	heapBase[T Ordered[T]] []T

	// A generic priority queue implementation. Elements with the lowest value
	// as determined by Ordered.Less are popped first.
	PriorityQueue[T Ordered[T]] struct {
		base heapBase[T]
	}
)

func (h heapBase[T]) Len() int           { return len(h) }
func (h heapBase[T]) Less(i, j int) bool { return h[i].Less(h[j]) }
func (h heapBase[T]) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *heapBase[T]) Push(x any)        { *h = append(*h, x.(T)) }
func (h *heapBase[T]) Pop() any {
	last := (*h)[len(*h)-1]
	*h = (*h)[:len(*h)-1]
	return last
}

// Creates a new PriorityQueue initialized with the given elements.
func NewPriorityQueue[T Ordered[T]](init []T) *PriorityQueue[T] {
	q := &PriorityQueue[T]{base: heapBase[T](init)}
	heap.Init(&q.base)
	return q
}

// Creates a new empty PriorityQueue.
func NewEmptyPriorityQueue[T Ordered[T]]() *PriorityQueue[T] {
	return NewPriorityQueue([]T(nil))
}

// Checks whether the priority queue is empty.
func (q PriorityQueue[T]) Empty() bool {
	return q.base.Len() == 0
}

// Pushes an element onto the priority queue.
func (q *PriorityQueue[T]) Push(item T) {
	heap.Push(&q.base, item)
}

// Pops the lowest value as determined by Ordered.Less from the priority queue.
// Will panic if the queue is empty.
func (q *PriorityQueue[T]) Pop() T {
	return heap.Pop(&q.base).(T)
}

// Peeks at the lowest value as determined by Ordered.Less in the priority queue
// without removing it. Will panic if the queue is empty.
func (q PriorityQueue[T]) Peek() T {
	return q.base[0]
}
