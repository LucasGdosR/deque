package deque

import (
	"cmp"
	"errors"
	"fmt"
	"iter"
	"math/bits"
	"slices"
)

// Deque is a double-ended queue that can be used for either LIFO or FIFO
// ordering, or something in between.
//
// To create a Deque instance, you must use one of the available constructors,
// MakeDeque(), MakeDequeWithCapacity(cap), or CopySliceToDeque(s). nil Deques
// panic when called, except for Len. Creating a Deque in the following way is
// wrong:
//
//	var deque Deque[int] // wrong
//
// This implementation requires a buffer with a power of two length. If a Deque
// ever overflows its underlying buffer, it reallocates to twice the size. It
// does not shrink by default, so you must explicitly call a method to shrink
// it.
type Deque[T any] struct {
	buf              []T
	head, tail, mask uint
}

/*****************************************************************************
 * CONSTRUCTORS
 *****************************************************************************/

// MakeDeque allocates a default sized buffer for a Deque.
func MakeDeque[T any]() *Deque[T] {
	const defaultCapacity = 16
	d, _ := MakeDequeWithCapacity[T](defaultCapacity)
	return d
}

// MakeDequeWithCapacity takes in the desired capacity. Note that if the
// supplied capacity is not a power of two, it will be increased to the next
// power of two. Returns an error if passed a negative value.
func MakeDequeWithCapacity[T any](capacity int) (*Deque[T], error) {
	if capacity < 0 {
		return nil, ErrNegativeCapacity
	}
	c := uint(capacity)
	c = ceilPow2(max(1, c))
	buf := make([]T, c)
	return &Deque[T]{buf: buf, mask: c - 1}, nil
}

// CopySliceToDeque takes in a slice, allocates a new buffer rounding len(s) to
// the next power of two, and copies every element of the slice to the Deque.
// The slice's capacity is irrelevant to CopySliceToDeque, and memory is not
// shared.
func CopySliceToDeque[T any](s []T) *Deque[T] {
	d, _ := MakeDequeWithCapacity[T](len(s))
	copy(d.buf, s)
	return d
}

/*****************************************************************************
 * DEQUE API
 *****************************************************************************/

// Len returns the number of elements in the Deque or 0 if nil.
func (d *Deque[T]) Len() int {
	if d == nil {
		return 0
	}
	return int(d.len())
}
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
	return d.PeekBackUnsafe(), true
}

// PeekBackUnsafe returns the last element in the Deque. Does not panic, but
// worse: silently returns garbage.
func (d *Deque[T]) PeekBackUnsafe() T {
	return d.buf[(d.tail-1)&d.mask]
}

// PeekFront returns the first element in the Deque. If the Deque is empty, it
// returns false.
func (d *Deque[T]) PeekFront() (t T, ok bool) {
	if d.Empty() {
		return
	}
	return d.PeekFrontUnsafe(), true
}

// PeekFrontUnsafe returns the first element in the Deque. Does not panic, but
// worse: silently returns garbage.
func (d *Deque[T]) PeekFrontUnsafe() T {
	return d.buf[d.head&d.mask]
}

// PopBack removes the last element in the Deque and returns it. If it's empty,
// returns false. This does not zero the element, so references remain and the
// garbage collector does not free. If your elements have references, prefer
// PopBackZero. PopBack is mainly used for LIFO ordering in types with no
// references.
func (d *Deque[T]) PopBack() (t T, ok bool) {
	if t, ok = d.PeekBack(); ok {
		d.tail--
	}
	return
}

// PopBackZero removes the last element in the Deque, zeroes its slot, and
// returns it. If it's empty, returns false. This is useful to clear references
// that the underlying element might hold. If your elements have references,
// this is how you should use the Deque for LIFO ordering.
func (d *Deque[T]) PopBackZero() (t T, ok bool) {
	if t, ok = d.PopBack(); ok {
		var zero T
		d.buf[d.tail&d.mask] = zero
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

// PopBackUnsafe removes the last element in the Deque and returns it. It does
// not zero the element, which means if the type has references, they will leak
// memory. Prefer PopBackZeroUnsafe if your type has references. Calling
// this method with an empty Deque leads to undefined behavior from then on.
func (d *Deque[T]) PopBackUnsafe() T {
	result := d.PeekBackUnsafe()
	d.tail--
	return result
}

// PopBackZeroUnsafe removes the last element in the Deque and returns it. The
// element is zeroed, clearing references and making it available for garbage
// collection. If your elements don't have references, prefer PopFrontUnsafe.
// Calling this method with an empty Deque leads to undefined behavior from
// then on.
func (d *Deque[T]) PopBackZeroUnsafe() T {
	result := d.PopBackUnsafe()
	var zero T
	d.buf[d.tail&d.mask] = zero
	return result
}

// PopFront removes the first element in the Deque and returns it. If it's
// empty, returns false. This does not zero the element, so references remain
// and the garbage collector does not free. If your elements have references,
// prefer PopFrontZero. PopFront is mainly used for FIFO ordering in types with
// no references.
func (d *Deque[T]) PopFront() (t T, ok bool) {
	if t, ok = d.PeekFront(); ok {
		d.head++
	}
	return
}

// PopFrontZero removes the first element in the Deque, zeroes its slot, and
// returns it. If it's empty, returns false. This is useful to clear references
// that the underlying element might hold. If your elements have references,
// this is how you should use the Deque for FIFO ordering.
func (d *Deque[T]) PopFrontZero() (t T, ok bool) {
	if t, ok = d.PopFront(); ok {
		var zero T
		d.buf[(d.head-1)&d.mask] = zero
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

// PopFrontUnsafe removes the first element in the Deque and returns it. The
// element is not zeroed, avoiding garbage collection of elements containing
// references. If your elements have references, prefer PopFrontZeroUnsafe.
// Calling this method with an empty Deque leads to undefined behavior from
// then on.
func (d *Deque[T]) PopFrontUnsafe() T {
	result := d.PeekFrontUnsafe()
	d.head++
	return result
}

// PopFrontZeroUnsafe removes the first element in the Deque and returns it.
// The element is zeroed, clearing references and making it available for
// garbage collection. If your elements don't have references, prefer
// PopFrontUnsafe. Calling this method with an empty Deque leads to undefined
// behavior from then on.
func (d *Deque[T]) PopFrontZeroUnsafe() T {
	results := d.PopFrontUnsafe()
	var zero T
	d.buf[(d.head-1)&d.mask] = zero
	return results
}

// DropFront removes the n first elements of the deque in O(1), but doesn't
// clear references. If the Deque has fewer than n elements, it drops every
// element. If n is negative, no element is dropped. If your elements have
// references, prefer DropFrontZero, which takes O(n).
func (d *Deque[T]) DropFront(n int) {
	if n >= 0 {
		d.head += min(uint(n), d.len())
	}
}

// DropFrontZero removes the n first elements of the deque in O(n) and clears
// references, allowing garbage collection to occur. If the Deque has fewer
// than n elements, it drops every element. If your elements don't have
// references, prefer DropFront, which takes O(1).
func (d *Deque[T]) DropFrontZero(n int) {
	if n >= 0 {
		n := min(uint(n), d.len())
		bound := d.head + n
		var zero T
		for i := d.head; i < bound; i++ {
			d.buf[i&d.mask] = zero
		}
		d.head += n
	}
}

// DropBack removes the n last elements of the deque in O(1), but doesn't
// clear references. If the Deque has fewer than n elements, it drops every
// element. If n is negative, no element is dropped. If your elements have
// references, prefer DropBackZero, which takes O(n).
func (d *Deque[T]) DropBack(n int) {
	if n >= 0 {
		d.tail -= min(uint(n), d.len())
	}
}

// DropBackZero removes the n last elements of the deque in O(n) and clears
// references, allowing garbage collection to occur. If the Deque has fewer
// than n elements, it drops every element. If your elements don't have
// references, prefer DropBack, which takes O(1).
func (d *Deque[T]) DropBackZero(n int) {
	if n >= 0 {
		n := min(uint(n), d.len())
		var zero T
		for i := d.tail - n; i < d.tail; i++ {
			d.buf[i&d.mask] = zero
		}
		d.tail -= n
	}
}

/*****************************************************************************
 * SLICE API
 *****************************************************************************/

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

// Reserve ensures there's enough capacity to add at least n more elements to
// the Deque, reallocating if necessary. It returns an error if n is negative.
func (d *Deque[T]) Reserve(n int) error {
	if n < 0 {
		return ErrNegativeCapacity
	}
	// Calling Reserve and not resizing is not an error, so ignore the return.
	_ = d.resize(ceilPow2(d.len() + uint(n)))
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
	if d == nil || d.Empty() {
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

// MakeSliceIndexCopyWithCapacity allocates a slice and copies the contents
// from the start index (inclusive) to the end index (non-inclusive). This is
// regular slice semantics, except it's a copy, and doesn't share memory with
// the Deque. This means it also panics with invalid indexes.
//
// Use this method when you need to append to the slice after copying it.
// Prefer passing a subslice of a buffer to CopyToSlice for memory reuse.
func (d *Deque[T]) MakeSliceIndexCopyWithCapacity(start, end, capacity int) []T {
	s := make([]T, capacity)
	// This copies (end-start) elements to the same memory and keeps the extra
	// capacity filled with zeroes.
	_ = d.CopySlice(start, s[:end-start])
	return s
}

// CopySlice has the same semantics as the copy() built-in function. It copies
// elements in the Deque starting at the start index up until the buffer is
// full or the Deque is over, whichever happens first.
//
// CopySlice returns the number of elements copied, which will be the minimum
// of len(buf) and d.Len().
func (d *Deque[T]) CopySlice(start int, buf []T) int {
	s1, s2 := d.slices()
	L1 := len(s1)
	if start < L1 {
		result := copy(buf, s1[start:])
		end := start + len(buf)
		if end > L1 {
			result += copy(buf[L1-start:], s2)
		}
		return result
	}
	return copy(buf, s2[start-L1:])
}

// At indexes into the i-th position in the Deque. Panics if out of bounds.
func (d *Deque[T]) At(i int) T {
	d.checkBounds(i)
	return d.AtUnsafe(i)
}

// AtUnsafe indexes into the i-th position in the Deque. It never panics, but
// returns garbage if i is out of bounds.
func (d *Deque[T]) AtUnsafe(i int) T {
	return d.buf[(d.head+uint(i))&d.mask]
}

// Set writes t to the i-th position in the Deque. Panics if out of bounds.
func (d *Deque[T]) Set(i int, t T) {
	d.checkBounds(i)
	d.SetUnsafe(i, t)
}

// SetUnsafe writes t to the i-th position in the Deque. It never panics, but
// writes to another index inside the deque if out of bounds.
func (d *Deque[T]) SetUnsafe(i int, t T) {
	d.buf[(d.head+uint(i))&d.mask] = t
}

// Swap swaps the elements in the i-th and j-th indexes. Panics if out of
// bounds.
func (d *Deque[T]) Swap(i, j int) {
	d.checkBounds(i)
	d.checkBounds(j)
	d.SwapUnsafe(i, j)
}

// SwapUnsafe swaps the elements in the i-th and j-th indexes. It never panics,
// but swaps the wrong elements if indexes are out of bounds.
func (d *Deque[T]) SwapUnsafe(i, j int) {
	a, b := d.AtUnsafe(i), d.AtUnsafe(j)
	d.SetUnsafe(i, b)
	d.SetUnsafe(j, a)
}

// ClearLazy empties the Deque in O(1), but does not zero the elements. If
// references remain, the memory they point to will not be garbage collected.
// Capacity is retained. This is useful for reusing a Deque with no references.
func (d *Deque[T]) ClearLazy() { d.head, d.tail = 0, 0 }

// ClearEager empties the Deque in O(d.Len()), zeroing existing elements and
// maintaining capacity. This is useful for reusing a Deque with references.
func (d *Deque[T]) ClearEager() {
	var zero T
	for i := d.head; i < d.tail; i++ {
		d.buf[i&d.mask] = zero
	}
	d.head, d.tail = 0, 0
}

// Contains returns whether the element is in the Deque. This must not be a
// method, otherwise Deque would be constrained to comparable elements. It has
// the same semantics as slices.Contains.
func Contains[T comparable](d *Deque[T], t T) bool {
	a, b := d.slices()
	return slices.Contains(a, t) || slices.Contains(b, t)
}

// ContainsFunc returns whether an element satisfying f is in the Deque. It has
// the same semantics as slices.ContainsFunc.
func (d *Deque[T]) ContainsFunc(f func(T) bool) bool {
	a, b := d.slices()
	return slices.ContainsFunc(a, f) || slices.ContainsFunc(b, f)
}

// Equal returns whether both Deques have the same length and the same elements
// in the same order. Two nil Deques are equal, but an empty Deque and nil are
// not. This must not be a method, otherwise Deque would be constrained to
// comparable elements. Equal's semantics differs from slices.Equal in the nil
// vs empty comparison.
func Equal[T comparable](d1 *Deque[T], d2 *Deque[T]) bool {
	if d1 == nil || d2 == nil {
		return d1 == d2
	}

	if d1.len() != d2.len() {
		return false
	}

	s11, s12 := d1.slices()
	s21, s22 := d2.slices()

	if len(s11) <= len(s12) {
		return slices.Equal(s11, s12[:len(s11)]) &&
			slices.Equal(s12[len(s11):], s21[:len(s12)-len(s11)]) &&
			slices.Equal(s21[len(s21)-len(s22):], s22)
	} else {
		return slices.Equal(s12, s11[:len(s12)]) &&
			slices.Equal(s11[len(s12):], s22[:len(s11)-len(s12)]) &&
			slices.Equal(s22[len(s22)-len(s21):], s21)
	}
}

// EqualFunc returns whether both Deques have the same length and the same
// elements in the same order. Two nil Deques are equal, but an empty Deque and
// nil are not. EqualFunc's semantics differs from slices.EqualFunc in the nil
// vs empty comparison.
func (d1 *Deque[T]) EqualFunc(d2 *Deque[T], f func(T, T) bool) bool {
	if d1 == nil || d2 == nil {
		return d1 == d2
	}

	if d1.len() != d2.len() {
		return false
	}

	s11, s12 := d1.slices()
	s21, s22 := d2.slices()

	if len(s11) <= len(s12) {
		return slices.EqualFunc(s11, s12[:len(s11)], f) &&
			slices.EqualFunc(s12[len(s11):], s21[:len(s12)-len(s11)], f) &&
			slices.EqualFunc(s21[len(s21)-len(s22):], s22, f)
	} else {
		return slices.EqualFunc(s12, s11[:len(s12)], f) &&
			slices.EqualFunc(s11[len(s12):], s22[:len(s11)-len(s12)], f) &&
			slices.EqualFunc(s22[len(s22)-len(s21):], s21, f)
	}
}

// Index returns the index of the first ocurrence of t in the Deque or -1 if
// absent. It cannot be a method, otherwise Deque would be constrained to
// comparable elements only. Index has the same semantics as slices.Index.
func Index[T comparable](d *Deque[T], t T) int {
	s1, s2 := d.slices()
	i := slices.Index(s1, t)
	if i != -1 {
		return i
	}
	i = slices.Index(s2, t)
	if i != -1 {
		return i + len(s1)
	}
	return -1
}

// IndexFunc returns the index of the first element that satisfies f in the
// Deque or -1 if none do. IndexFunc has the same semantics as
// slices.IndexFunc.
func (d *Deque[T]) IndexFunc(f func(T) bool) int {
	s1, s2 := d.slices()
	i := slices.IndexFunc(s1, f)
	if i != -1 {
		return i
	}
	i = slices.IndexFunc(s2, f)
	if i != -1 {
		return i + len(s1)
	}
	return -1
}

// Max returns the maximum element in the queue. It must not be a method,
// otherwise Deque would be constrained to comparable elements only. It has the
// same semantics as slices.Max, so it panics on an empty Deque.
func Max[T cmp.Ordered](d *Deque[T]) T {
	s1, s2 := d.slices()
	result := slices.Max(s1)
	// slices.Max panics on an empty slice, so handle this edge case.
	if s2 != nil {
		result = max(result, slices.Max(s2))
	}
	return result
}

// MaxFunc returns the maximum element in the queue. It must not be a method,
// otherwise Deque would be constrained to comparable elements only. It has the
// same semantics as slices.MaxFunc, so it panics on an empty Deque.
func MaxFunc[T cmp.Ordered](d *Deque[T], cmp func(T, T) int) T {
	s1, s2 := d.slices()
	result := slices.MaxFunc(s1, cmp)
	// slices.MaxFunc panics on an empty slice, so handle this edge case.
	if s2 != nil {
		result = max(result, slices.MaxFunc(s2, cmp))
	}
	return result
}

// Min returns the minimum element in the queue. It must not be a method,
// otherwise Deque would be constrained to comparable elements only. It has the
// same semantics as slices.Min, so it panics on an empty Deque.
func Min[T cmp.Ordered](d *Deque[T]) T {
	s1, s2 := d.slices()
	result := slices.Min(s1)
	// slices.Min panics on an empty slice, so handle this edge case.
	if s2 != nil {
		result = min(result, slices.Min(s2))
	}
	return result
}

// MinFunc returns the minimum element in the queue. It must not be a method,
// otherwise Deque would be constrained to comparable elements only. It has the
// same semantics as slices.MinFunc, so it panics on an empty Deque.
func MinFunc[T cmp.Ordered](d *Deque[T], cmp func(T, T) int) T {
	s1, s2 := d.slices()
	result := slices.MinFunc(s1, cmp)
	// slices.MinFunc panics on an empty slice, so handle this edge case.
	if s2 != nil {
		result = min(result, slices.MinFunc(s2, cmp))
	}
	return result
}

// ForEach takes in a function that returns a bool and calls it in order for
// every element in the queue, or until the first call that returns false.
func (d *Deque[T]) ForEach(f func(T) bool) {
	s1, s2 := d.slices()
	for _, t := range s1 {
		if !f(t) {
			return
		}
	}
	for _, t := range s2 {
		if !f(t) {
			return
		}
	}
}

// All returns an iterator over index-value pairs in order. It has the same
// semantics as slices.All. If you don't need indexes, use Iter instead.
// Does not panic if modified during iteration.
func (d *Deque[T]) All() iter.Seq2[int, T] {
	// TODO: replace this with slices.All?
	return func(yield func(int, T) bool) {
		if d == nil {
			return
		}
		s1, s2 := d.slices()
		var i int
		var t T
		for i, t = range s1 {
			if !yield(i, t) {
				return
			}
		}
		for _, t = range s2 {
			if !yield(i, t) {
				return
			}
			i++
		}
	}
}

// TODO: Rotate,  more of the slices package?

/*****************************************************************************
 * ITER API
 *****************************************************************************/

// Iter returns an iterator over values only in order. If you need indexes,
// use All instead. Does not panic if the Deque is modified during iteration.
func (d *Deque[T]) Iter() iter.Seq[T] {
	return func(yield func(T) bool) {
		if d == nil {
			return
		}
		s1, s2 := d.slices()
		for _, t := range s1 {
			if !yield(t) {
				return
			}
		}
		for _, t := range s2 {
			if !yield(t) {
				return
			}
		}
	}
}

// TODO: RIter, IterPopFront, IterPopBack, IterPopFrontZero, IterPopBackZero

/*****************************************************************************
 * SENTINEL ERRORS
 *****************************************************************************/

// ErrSameCapacity is returned when trying to resize a Deque to its current
// capacity.
var ErrSameCapacity = errors.New("already at asked capacity")

// ErrNotEnoughCapacity is returned when trying to resize a Deque to a capacity
// that cannot hold its existing elements.
var ErrNotEnoughCapacity = errors.New("cannot hold existing elements in asked capacity")

// ErrNegativeCapacity is returned when trying to resize a Deque to a negative
// capacity.
var ErrNegativeCapacity = errors.New("capacity cannot be negative")

/*****************************************************************************
 * HELPERS
 *****************************************************************************/

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
