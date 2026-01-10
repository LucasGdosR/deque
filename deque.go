package deque

import (
	"errors"
	"fmt"
	"math/bits"
	"slices"
)

// Deque is a double-ended queue that can be used for either LIFO or FIFO
// ordering, or something in between.
//
// To create a Deque instance, you must use one of the available constructors,
// MakeDeque() and MakeDequeWithCapacity(cap). Creating a Deque in the
// following way is wrong:
//
//	var deque Deque[int] // wrong
//
// This implementation requires a buffer with a power of two length. If a Deque
// ever overflows its underlying buffer, it reallocates to twice the size. It
// does not shrink by default, so you must explicitly call a method to shrink it.
type Deque[T any] struct {
	buf              []T
	head, tail, mask uint
}

/******************************************************************************
 * CONSTRUCTORS
 ******************************************************************************/

// MakeDeque allocates a default sized buffer for a Deque.
func MakeDeque[T any]() *Deque[T] {
	const defaultCapacity = 16
	return MakeDequeWithCapacity[T](defaultCapacity)
}

// MakeDequeWithCapacity takes in the desired capacity. Note that if the
// supplied capacity is not a power of two, it will be increased to the next
// power of two. Returns nil if passed a negative value.
func MakeDequeWithCapacity[T any](capacity int) *Deque[T] {
	if capacity < 0 {
		return nil
	}
	c := uint(capacity)
	c = ceilPow2(max(1, c))
	buf := make([]T, c)
	return &Deque[T]{buf: buf, mask: c - 1}
}

/******************************************************************************
 * DEQUE API
 ******************************************************************************/

// Len returns the number of elements in the Deque.
func (d *Deque[T]) Len() int  { return int(d.len()) }
func (d *Deque[T]) len() uint { return d.tail - d.head }

// Empty returns whether the Deque is empty.
func (d *Deque[T]) Empty() bool { return d.tail == d.head }

// Full returns whether the Deque is full. Pushing to a full Deque reallocates.
func (d *Deque[T]) Full() bool { return d.len() == d.cap() }

// PushBack takes in a variable number of arguments and puts them at the back
// of the Deque. Use PushBack and PopFront for FIFO ordering, or PushBack and
// PopBack for LIFO ordering.
//
// PushBack reallocates at most once, no matter how many arguments. It is more
// efficient to push multiple elements at once. The last argument is the new
// back of the list.
func (d *Deque[T]) PushBack(ts ...T) {
	n := uint(len(ts))
	if d.len()+n > d.cap() {
		d.resize(ceilPow2(d.len() + n))
	}
	for i, t := range ts {
		d.buf[(d.tail+uint(i))&d.mask] = t
	}
	d.tail += n
}

// PushFront takes in a variable number of arguments and puts them at the front
// of the Deque.
//
// PushFront reallocates at most once, no matter how many arguments. It is more
// efficient to push multiple elements at once. The last argument is the new
// front of the list.
func (d *Deque[T]) PushFront(ts ...T) {
	n := uint(len(ts))
	if d.len()+n > d.cap() {
		d.resize(ceilPow2(d.len() + n))
	}
	base := d.head - 1
	for i, t := range ts {
		d.buf[(base-uint(i))&d.mask] = t
	}
	d.head -= n
}

// PeekBack returns the last element in the Deque. If the Deque is empty, it
// returns false.
func (d *Deque[T]) PeekBack() (t T, ok bool) {
	if d.Empty() {
		return
	}
	return d.PeekBackUnchecked(), true
}

// PeekBackUnchecked returns the last element in the Deque. Does not panic, but
// worse: silently returns garbage.
func (d *Deque[T]) PeekBackUnchecked() T {
	return d.buf[(d.tail-1)&d.mask]
}

// PeekFront returns the first element in the Deque. If the Deque is empty, it
// returns false.
func (d *Deque[T]) PeekFront() (t T, ok bool) {
	if d.Empty() {
		return
	}
	return d.PeekFrontUnchecked(), true
}

// PeekFrontUnchecked returns the first element in the Deque. Does not panic,
// but worse: silently returns garbage.
func (d *Deque[T]) PeekFrontUnchecked() T {
	return d.buf[d.head&d.mask]
}

// PopBack removes the last element in the Deque and returns it. If it's empty,
// returns false. It is mainly used for LIFO ordering.
func (d *Deque[T]) PopBack() (t T, ok bool) {
	if t, ok = d.PeekBack(); ok {
		d.tail--
	}
	return
}

// PopBackShrink removes the last element in the Deque and returns it. If it's
// empty, false is returned. If the Deque is at <= 25% capacity, it is shrunk
// to <= 50% capacity.
//
// It is more efficient to call PopBackShrink only once, when you're done
// popping, to avoid multiple reallocations. It is also more efficient to
// avoid calling this method when you might push many elements, which might
// require growing the Deque back.
func (d *Deque[T]) PopBackShrink() (t T, ok bool) {
	if t, ok = d.PeekBack(); ok {
		d.tail--
	}
	if d.len() <= d.cap()>>2 {
		d.resize(ceilPow2(d.len() << 1))
	}
	return
}

// PopBackUnchecked removes the last element in the Deque and returns it.
// Calling this method with an empty Deque leads to undefined behavior from
// then on.
func (d *Deque[T]) PopBackUnchecked() T {
	result := d.PeekBackUnchecked()
	d.tail--
	return result
}

// PopFront removes the first element in the Deque and returns it. If it's
// empty, returns false. It is mainly used for FIFO ordering.
func (d *Deque[T]) PopFront() (t T, ok bool) {
	if t, ok = d.PeekFront(); ok {
		d.head++
	}
	return
}

// PopFrontShrink removes the first element in the Deque and returns it. If
// it's empty, false is returned. If the Deque is at <= 25% capacity, it is
// shrunk to <= 50% capacity.
//
// It is more efficient to call PopFrontShrink only once, when you're done
// popping, to avoid multiple reallocations. It is also more efficient to
// avoid calling this method when you might push many elements, which might
// require growing the Deque back.
func (d *Deque[T]) PopFrontShrink() (t T, ok bool) {
	if t, ok = d.PeekFront(); ok {
		d.head++
	}
	if d.len() <= d.cap()>>2 {
		d.resize(ceilPow2(d.len() << 1))
	}
	return
}

// PopFrontUnchecked removes the first element in the Deque and returns it.
// Calling this method with an empty Deque leads to undefined behavior from
// then on.
func (d *Deque[T]) PopFrontUnchecked() T {
	result := d.PeekFrontUnchecked()
	d.head++
	return result
}

/******************************************************************************
 * SLICE API
 ******************************************************************************/

// Cap returns the current Deque capacity.
func (d *Deque[T]) Cap() int  { return len(d.buf) }
func (d *Deque[T]) cap() uint { return uint(len(d.buf)) }

// Resize takes in the minimum desired capacity, rounds it up to a power of
// two, and reallocates the underlying buffer.
//
// It returns an error if the new capacity matches the old, or if the new
// capacity cannot hold the existing elements, or if minCapacity is negative.
func (d *Deque[T]) Resize(minCapacity int) error {
	if minCapacity < 0 {
		return ErrNegativeCapacity
	}
	return d.resize(ceilPow2(uint(minCapacity)))
}

// Internal implementation for Resize and Shrink.
func (d *Deque[T]) resize(newCap uint) error {
	if newCap == d.cap() {
		return ErrSameCapacity
	}

	oldLen := d.len()
	if oldLen > newCap {
		return ErrNotEnoughCapacity
	}

	newBuf := make([]T, newCap)
	for i := range oldLen {
		newBuf[i] = d.buf[(d.head+i)&d.mask]
	}

	d.buf = newBuf
	d.head = 0
	d.tail = oldLen
	d.mask = newCap - 1
	return nil
}

// Shrink reallocates the underlying slice to the smallest size possible and
// returns the new Deque's capacity.
func (d *Deque[T]) Shrink() uint {
	newCap := ceilPow2(d.len())
	_ = d.resize(newCap)
	return newCap
}

// Helper to reuse the slices package functions.
func (d *Deque[T]) slices() (a, b []T) {
	if d.Empty() {
		return nil, nil
	}

	h := d.head & d.mask
	t := d.tail & d.mask

	if h < t {
		return d.buf[h:t], nil
	}
	return d.buf[h:], d.buf[:t]
}

// MakeSliceCopy allocates a slice to hold every Deque element and copies them.
// Prefer passing a buffer to CopyToSlice for memory reuse.
func (d *Deque[T]) MakeSliceCopy() []T {
	s := make([]T, d.len())
	_ = d.CopySlice(0, s)
	return s
}

// MakeSliceIndexCopy allocates a slice and copies the contents from the start
// index (inclusive) to the end index (non-inclusive). This is regular slice
// semantics, except it's a copy, and doesn't share memory with the Deque. This
// means it also panics with invalid indexes.
//
// Prefer passing a buffer to CopyToSlice for memory reuse.
func (d *Deque[T]) MakeSliceIndexCopy(start, end int) []T {
	s := make([]T, end-start)
	_ = d.CopySlice(start, s)
	return s
}

// MakeSliceIndexCopyWithCapacity allocates a slice and copies the contents from
// the start index (inclusive) to the end index (non-inclusive). This is regular
// slice semantics, except it's a copy, and doesn't share memory with the Deque.
// This means it also panics with invalid indexes.
//
// Use this method when you need to append to the slice after copying it. Prefer
// passing a subslice of a buffer to CopyToSlice for memory reuse.
func (d *Deque[T]) MakeSliceIndexCopyWithCapacity(start, end, capacity int) []T {
	s := make([]T, capacity)
	// This copies (end-start) elements to the same memory and keeps the extra
	// capacity filled with zeroes.
	_ = d.CopySlice(start, s[:end-start])
	return s
}

// CopySlice has the same semantics as the copy() built-in function. It copies
// elements in the Deque starting at the start index up until the buffer is full
// or the Deque is over, whichever happens first.
//
// CopySlice returns the number of elements copied, which will be the minimum
// of len(buf) and d.Len().
func (d *Deque[T]) CopySlice(start int, buf []T) int {
	var result int
	s1, s2 := d.slices()
	if start < len(s1) {
		result = copy(buf, s1[start:])
	}
	end := start + len(buf)
	if end > len(s1) {
		result += copy(buf[len(s1)-start:], s2)
	}
	return result
}

// At indexes into the i-th position in the Deque. Panics if out of bounds.
func (d *Deque[T]) At(i int) T {
	d.checkBounds(i)
	return d.AtUnchecked(i)
}

// AtUnchecked indexes into the i-th position in the Deque. It never panics, but
// returns garbage if i is out of bounds.
func (d *Deque[T]) AtUnchecked(i int) T {
	return d.buf[(d.head+uint(i))&d.mask]
}

// Set writes t to the i-th position in the Deque. Panics if out of bounds.
func (d *Deque[T]) Set(i int, t T) {
	d.checkBounds(i)
	d.SetUnchecked(i, t)
}

// Set writes t to the i-th position in the Deque. It never panics, but writes
// to another index inside the deque if out of bounds.
func (d *Deque[T]) SetUnchecked(i int, t T) {
	d.buf[(d.head+uint(i))&d.mask] = t
}

// Clear removes every element of the Deque in O(1), making it ready for reuse.
// Capacity is retained.
func (d *Deque[T]) Clear() { d.head, d.tail = 0, 0 }

// Contains returns whether the element is in the Deque. This must not be a
// method, otherwise Deque would be constrained to comparable elements.
func Contains[T comparable](d *Deque[T], t T) bool {
	a, b := d.slices()
	return slices.Contains(a, t) || slices.Contains(b, t)
}

// TODO: rest of the slices package, iter, Swap, SliceToDeque, Rotate.

/******************************************************************************
 * SENTINEL ERRORS
 ******************************************************************************/

// ErrSameCapacity is returned when trying to resize a Deque to its current cap.
var ErrSameCapacity = errors.New("already at asked capacity")

// ErrNotEnoughCapacity is returned when trying to resize a Deque to a capacity
// that cannot hold its existing elements.
var ErrNotEnoughCapacity = errors.New("cannot hold existing elements in asked capacity")

// ErrNegativeCapacity is returned when trying to resize a Deque to a negative
// capacity.
var ErrNegativeCapacity = errors.New("capacity cannot be negative")

/******************************************************************************
 * HELPERS
 ******************************************************************************/

func ceilPow2(x uint) uint {
	// For our purposes, 0 is invalid.
	if x == 0 {
		return 1
	}
	const arch = bits.UintSize
	msb := arch - 1 - bits.LeadingZeros(x)
	var result uint = 1 << msb
	if result < x {
		result <<= 1
	}
	return result
}

func (d *Deque[T]) checkBounds(i int) {
	if i < 0 || i >= d.Len() {
		panic(fmt.Sprintf("deque: index %d out of bounds with length %d", i, d.Len()))
	}
}
