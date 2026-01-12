# deque

[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Blazingly fast ring-buffer deque ([double-ended queue](https://en.wikipedia.org/wiki/Double-ended_queue)) with both low level and high level APIs. Gives you full control.

## Installation

```
$ go get github.com/lucasgdosr/deque
```

## Deque data structure

Deques generalize queues, stacks, and arrays. Adding or removing elements from both ends are O(1) operations, as is indexing into any position. [Queue](https://en.wikipedia.org/wiki/Queue_(abstract_data_type)) (FIFO) operations are supported using `PushBack*` and `PopFront*`. [Stack](https://en.wikipedia.org/wiki/Stack_(abstract_data_type)) (LIFO) operations are supported using `PushBack*` and `PopBack*`. [Array](https://en.wikipedia.org/wiki/Array_(data_structure)) indexing operations are supported using `At*` and `Set*`. Much of the `slices` package functionality is implemented either as methods or functions which take `*Deque` as arguments.

## Ring-buffer implementation

The implementation consists of an underlying generic slice (`[]T`) with a power of two length with a head and a tail, which can also be thought of as the count of pushed and popped elements in a queue. This slice wraps around as elements are pushed and popped, reusing memory as long as it fits all elements, and reallocating to the next power of two if it doesn't.

Powers of two have a special property in binary. They're all of the form `0*10*`, that is, leading zeroes, a single one, and trailing zeroes. This also means a power of two minus one has the form `0*1*` - the original one is flipped to zero and all originally trailing zeroes are flipped to ones. In this way, `% len(slice)` is equivalent to `& (len(slice) - 1)`. This allows us to monotonically increase the `head` and `tail` counts and bitwise-and them with a mask that's just the length of the slice minus one to get the index. We can avoid maintaining a count of the number of elements and maintaining head and tail within the index bounds of the slice with [an array + two unmasked indices](https://www.snellman.net/blog/archive/2016-12-13-ring-buffers/), also known as a [virtual stream](https://fgiesen.wordpress.com/2010/12/14/ring-buffers-and-queues/).

## API

### Construction

Every method takes a pointer as its receiver. To instantiate a deque and get its pointer, call `deque.MakeDeque()`. If you already have an upper bound on the number of elements that may be stored in the deque, prefer `deque.MakeDequeWithCapacity(capacity)` to avoid reallocations. You do not need to worry about passing in a power of two, but be aware that whatever capacity you pass will be rounded up to a power of two. Alternatively, you may instantiate a deque out of an existing slice with `deque.CopySliceToDeque(s)`. Note this is a copy, and does not reuse the passed slice. The deque owns its underlying slice, and does not share memory.

### Pushing, peeking, and popping

Pushing can be done to either end of the deque with `PushFront` and `PushBack`. Conventionally, `PushBack` is generally preferred. These methods take in a variable number of elements, so pushing an entire slice onto a deque can be done with a single call to `d.PushBack(s...)`. They may reallocate into a larger slice if there is no space left. You may use `d.Reserve(n)` to ensure there's enough space to hold at least n more elements, reallocating if needed. You may also use `d.Resize(newCapacity)` if you prefer to specify the number of total elements. Do not worry about passing in a power of two, as the input is rounded up to a power of two.

Peeking is how you access the ends of the deque without removing the elements. There is `PeekFront*` and `PeekBack*`, with safe and `Unsafe` variants. Safe variants return a bool indicating whether the deque had any elements, and the unsafe variants do not check whether the deque is empty and always return something, which might be a previously popped element if the deque is empty. Only ever call the unsafe versions if you're certain the deque isn't empty.

Popping is how you remove elements from the deque. There are safe and `Unsafe` variants just like in `Peek*`, with the same bool mechanism. Do not call the `Unsafe` version unless you are absolutely sure the deque is not empty. Results are catastrophic. There are also `Zero` variants and `Shrink` variants. `Zero` variants overwrite the popped element with their zero value, effectively allowing garbage collection to happen for elements that hold pointers. Prefer the `Zero` variants if your elements are pointers or are structs that hold pointers, or you might leak memory until the element is overwritten with another push. The `Shrink` variants are designed to give you the option to reallocate the underlying slice if it is ever needlessly large. Do not favor the `Shrink` variants if many pushes might follow. You can also call `Resize` explicitly when you're sure you're done pushing and want to claim back memory using `d.Resize(d.Len())`.

### Clear and drop

If you ever need to get rid of the elements in the deque but don't care about their contents, instead of calling multiple `Pop`s and ignoring their return, favor `Clear*` and `Drop*`. These methods keep the original capacity and the underlying slice. Both of them have `Zero` and regular variants, which are useful for elements with and without pointers, respectivelly, just as `Pop`. `ClearEager` is the `Zero` variant, and `ClearLazy` is the regular variant. The regular variants have O(1) cost, as they just update the head and tail, while the zero versions have O(n) cost, as they actually need to overwrite the deque's contents. `Clear` clears every element, and `Drop(Front/Back)*` drops n elements.

### Slices

You may access any element in the deque by index using `At*` and `Set` for read / write operations. The head is the zeroth index, and the tail is the `d.Len() - 1`th index. There are both safe and `Unsafe` variants. Unlike regular slices, the `Unsafe` variants to not panic, but they return the contents of another index (possibly of a previously popped element) or set the wrong index. Only call the `Unsafe` variants if you are absolutely sure they are within bounds. These operations may be combined into `Swap*`, with safe and `Unsafe` versions.

If you don't actually want to go through specific indexes, but rather through all of them, prefer `ForEach`, which applies a function to every element until it returns false, or `All`, which returns an iterator over index-value pairs, or `Iter`, which returns an iterator over values. TODO: `RIter`, `IterPop(Front/Back)(Zero)*`.

Other functionality from the `slices` package is available, such as `Contains*`, `Equal*`, `Index*`, `Min*`, `Max*`, with the regular and `Func` variants. The `Func` variants are generally methods, while the regular variants are functions that take in `*Deque` as arguments due to generic limitations. `MinFunc` and `MaxFunc` are also functions.

If you actually need explicit slices, you can get a shallow copy of the deque's elements. These slices do not share memory with the deque. Generally the best way is to pass your own slice to `d.CopySlice(start, buf)` and have it filled with copies of the elements in the deque. It has the same semantics as the `copy` built-in function, copying elements up until one of the slices is over. This allows you to reuse buffers. If you actually want to allocate new slices, there're three options. `d.MakeSliceCopy()` allocates a new slice with just enough capacity to hold every element in the deque, fills it with copies, and returns it. If you don't want every element, only a subset of them, call `d.MakeSliceIndexCopy(start, end)`. This is equivalent to `s[start:end]` in regular slice syntax, except it's a copy. If you want the resulting slice to have extra capacity, use `d.MakeSliceIndexCopyWithCapacity(start, end, capacity)`, and the returned slice will still have room for more elements to be appended.
