// Copyright (c) Tailscale Inc & AUTHORS
// SPDX-License-Identifier: BSD-3-Clause

// Package views provides read-only accessors for commonly used
// value types.
package views

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"maps"
	"reflect"
	"slices"

	"go4.org/mem"
)

func unmarshalSliceFromJSON[T any](b []byte, x *[]T) error {
	if *x != nil {
		return errors.New("already initialized")
	}
	if len(b) == 0 {
		return nil
	}
	return json.Unmarshal(b, x)
}

// ByteSlice is a read-only accessor for types that are backed by a []byte.
type ByteSlice[T ~[]byte] struct {
	// ж is the underlying mutable value, named with a hard-to-type
	// character that looks pointy like a pointer.
	// It is named distinctively to make you think of how dangerous it is to escape
	// to callers. You must not let callers be able to mutate it.
	ж T
}

// ByteSliceOf returns a ByteSlice for the provided slice.
func ByteSliceOf[T ~[]byte](x T) ByteSlice[T] {
	return ByteSlice[T]{x}
}

// MapKey returns a unique key for a slice, based on its address and length.
func (v ByteSlice[T]) MapKey() SliceMapKey[byte] { return mapKey(v.ж) }

// Len returns the length of the slice.
func (v ByteSlice[T]) Len() int {
	return len(v.ж)
}

// IsNil reports whether the underlying slice is nil.
func (v ByteSlice[T]) IsNil() bool {
	return v.ж == nil
}

// Mem returns a read-only view of the underlying slice.
func (v ByteSlice[T]) Mem() mem.RO {
	return mem.B(v.ж)
}

// Equal reports whether the underlying slice is equal to b.
func (v ByteSlice[T]) Equal(b T) bool {
	return bytes.Equal(v.ж, b)
}

// EqualView reports whether the underlying slice is equal to b.
func (v ByteSlice[T]) EqualView(b ByteSlice[T]) bool {
	return bytes.Equal(v.ж, b.ж)
}

// AsSlice returns a copy of the underlying slice.
func (v ByteSlice[T]) AsSlice() T {
	return v.AppendTo(v.ж[:0:0])
}

// AppendTo appends the underlying slice values to dst.
func (v ByteSlice[T]) AppendTo(dst T) T {
	return append(dst, v.ж...)
}

// At returns the byte at index `i` of the slice.
func (v ByteSlice[T]) At(i int) byte { return v.ж[i] }

// SliceFrom returns v[i:].
func (v ByteSlice[T]) SliceFrom(i int) ByteSlice[T] { return ByteSlice[T]{v.ж[i:]} }

// SliceTo returns v[:i]
func (v ByteSlice[T]) SliceTo(i int) ByteSlice[T] { return ByteSlice[T]{v.ж[:i]} }

// Slice returns v[i:j]
func (v ByteSlice[T]) Slice(i, j int) ByteSlice[T] { return ByteSlice[T]{v.ж[i:j]} }

// MarshalJSON implements json.Marshaler.
func (v ByteSlice[T]) MarshalJSON() ([]byte, error) { return json.Marshal(v.ж) }

// UnmarshalJSON implements json.Unmarshaler.
func (v *ByteSlice[T]) UnmarshalJSON(b []byte) error {
	if v.ж != nil {
		return errors.New("already initialized")
	}
	return json.Unmarshal(b, &v.ж)
}

// StructView represents the corresponding StructView of a Viewable. The concrete types are
// typically generated by tailscale.com/cmd/viewer.
type StructView[T any] interface {
	// Valid reports whether the underlying Viewable is nil.
	Valid() bool
	// AsStruct returns a deep-copy of the underlying value.
	// It returns nil, if Valid() is false.
	AsStruct() T
}

// Cloner is any type that has a Clone function returning a deep-clone of the receiver.
type Cloner[T any] interface {
	// Clone returns a deep-clone of the receiver.
	// It returns nil, when the receiver is nil.
	Clone() T
}

// ViewCloner is any type that has had View and Clone funcs generated using
// tailscale.com/cmd/viewer.
type ViewCloner[T any, V StructView[T]] interface {
	// View returns a read-only view of Viewable.
	// If Viewable is nil, View().Valid() reports false.
	View() V
	// Clone returns a deep-clone of Viewable.
	// It returns nil, when Viewable is nil.
	Clone() T
}

// SliceOfViews returns a ViewSlice for x.
func SliceOfViews[T ViewCloner[T, V], V StructView[T]](x []T) SliceView[T, V] {
	return SliceView[T, V]{x}
}

// SliceView wraps []T to provide accessors which return an immutable view V of
// T. It is used to provide the equivalent of SliceOf([]V) without having to
// allocate []V from []T.
type SliceView[T ViewCloner[T, V], V StructView[T]] struct {
	// ж is the underlying mutable value, named with a hard-to-type
	// character that looks pointy like a pointer.
	// It is named distinctively to make you think of how dangerous it is to escape
	// to callers. You must not let callers be able to mutate it.
	ж []T
}

// All returns an iterator over v.
func (v SliceView[T, V]) All() iter.Seq2[int, V] {
	return func(yield func(int, V) bool) {
		for i := range v.ж {
			if !yield(i, v.ж[i].View()) {
				return
			}
		}
	}
}

// MarshalJSON implements json.Marshaler.
func (v SliceView[T, V]) MarshalJSON() ([]byte, error) { return json.Marshal(v.ж) }

// UnmarshalJSON implements json.Unmarshaler.
func (v *SliceView[T, V]) UnmarshalJSON(b []byte) error { return unmarshalSliceFromJSON(b, &v.ж) }

// IsNil reports whether the underlying slice is nil.
func (v SliceView[T, V]) IsNil() bool { return v.ж == nil }

// Len returns the length of the slice.
func (v SliceView[T, V]) Len() int { return len(v.ж) }

// At returns a View of the element at index `i` of the slice.
func (v SliceView[T, V]) At(i int) V { return v.ж[i].View() }

// SliceFrom returns v[i:].
func (v SliceView[T, V]) SliceFrom(i int) SliceView[T, V] { return SliceView[T, V]{v.ж[i:]} }

// SliceTo returns v[:i]
func (v SliceView[T, V]) SliceTo(i int) SliceView[T, V] { return SliceView[T, V]{v.ж[:i]} }

// Slice returns v[i:j]
func (v SliceView[T, V]) Slice(i, j int) SliceView[T, V] { return SliceView[T, V]{v.ж[i:j]} }

// SliceMapKey represents a comparable unique key for a slice, based on its
// address and length. It can be used to key maps by slices but should only be
// used when the underlying slice is immutable.
//
// Empty and nil slices have different keys.
type SliceMapKey[T any] struct {
	// t is the address of the first element, or nil if the slice is nil or
	// empty.
	t *T
	// n is the length of the slice, or -1 if the slice is nil.
	n int
}

// MapKey returns a unique key for a slice, based on its address and length.
func (v SliceView[T, V]) MapKey() SliceMapKey[T] { return mapKey(v.ж) }

// AppendTo appends the underlying slice values to dst.
func (v SliceView[T, V]) AppendTo(dst []V) []V {
	for _, x := range v.ж {
		dst = append(dst, x.View())
	}
	return dst
}

// AsSlice returns a copy of underlying slice.
func (v SliceView[T, V]) AsSlice() []V {
	return v.AppendTo(nil)
}

// Slice is a read-only accessor for a slice.
type Slice[T any] struct {
	// ж is the underlying mutable value, named with a hard-to-type
	// character that looks pointy like a pointer.
	// It is named distinctively to make you think of how dangerous it is to escape
	// to callers. You must not let callers be able to mutate it.
	ж []T
}

// All returns an iterator over v.
func (v Slice[T]) All() iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		for i, v := range v.ж {
			if !yield(i, v) {
				return
			}
		}
	}
}

// MapKey returns a unique key for a slice, based on its address and length.
func (v Slice[T]) MapKey() SliceMapKey[T] { return mapKey(v.ж) }

// mapKey returns a unique key for a slice, based on its address and length.
func mapKey[T any](x []T) SliceMapKey[T] {
	if x == nil {
		return SliceMapKey[T]{nil, -1}
	}
	if len(x) == 0 {
		return SliceMapKey[T]{nil, 0}
	}
	return SliceMapKey[T]{&x[0], len(x)}
}

// SliceOf returns a Slice for the provided slice for immutable values.
// It is the caller's responsibility to make sure V is immutable.
func SliceOf[T any](x []T) Slice[T] {
	return Slice[T]{x}
}

// MarshalJSON implements json.Marshaler.
func (v Slice[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.ж)
}

// UnmarshalJSON implements json.Unmarshaler.
func (v *Slice[T]) UnmarshalJSON(b []byte) error {
	return unmarshalSliceFromJSON(b, &v.ж)
}

// IsNil reports whether the underlying slice is nil.
func (v Slice[T]) IsNil() bool { return v.ж == nil }

// Len returns the length of the slice.
func (v Slice[T]) Len() int { return len(v.ж) }

// At returns the element at index `i` of the slice.
func (v Slice[T]) At(i int) T { return v.ж[i] }

// SliceFrom returns v[i:].
func (v Slice[T]) SliceFrom(i int) Slice[T] { return Slice[T]{v.ж[i:]} }

// SliceTo returns v[:i]
func (v Slice[T]) SliceTo(i int) Slice[T] { return Slice[T]{v.ж[:i]} }

// Slice returns v[i:j]
func (v Slice[T]) Slice(i, j int) Slice[T] { return Slice[T]{v.ж[i:j]} }

// AppendTo appends the underlying slice values to dst.
func (v Slice[T]) AppendTo(dst []T) []T {
	return append(dst, v.ж...)
}

// AsSlice returns a copy of underlying slice.
func (v Slice[T]) AsSlice() []T {
	return v.AppendTo(v.ж[:0:0])
}

// IndexFunc returns the first index of an element in v satisfying f(e),
// or -1 if none do.
//
// As it runs in O(n) time, use with care.
func (v Slice[T]) IndexFunc(f func(T) bool) int {
	for i := range v.Len() {
		if f(v.At(i)) {
			return i
		}
	}
	return -1
}

// ContainsFunc reports whether any element in v satisfies f(e).
//
// As it runs in O(n) time, use with care.
func (v Slice[T]) ContainsFunc(f func(T) bool) bool {
	return slices.ContainsFunc(v.ж, f)
}

// AppendStrings appends the string representation of each element in v to dst.
func AppendStrings[T fmt.Stringer](dst []string, v Slice[T]) []string {
	for _, x := range v.ж {
		dst = append(dst, x.String())
	}
	return dst
}

// SliceContains reports whether v contains element e.
//
// As it runs in O(n) time, use with care.
func SliceContains[T comparable](v Slice[T], e T) bool {
	return slices.Contains(v.ж, e)
}

// SliceEqual is like the standard library's slices.Equal, but for two views.
func SliceEqual[T comparable](a, b Slice[T]) bool {
	return slices.Equal(a.ж, b.ж)
}

// SliceEqualAnyOrder reports whether a and b contain the same elements, regardless of order.
// The underlying slices for a and b can be nil.
func SliceEqualAnyOrder[T comparable](a, b Slice[T]) bool {
	if a.Len() != b.Len() {
		return false
	}

	var diffStart int // beginning index where a and b differ
	for n := a.Len(); diffStart < n; diffStart++ {
		if a.At(diffStart) != b.At(diffStart) {
			break
		}
	}
	if diffStart == a.Len() {
		return true
	}

	// count the occurrences of remaining values and compare
	valueCount := make(map[T]int)
	for i, n := diffStart, a.Len(); i < n; i++ {
		valueCount[a.At(i)]++
		valueCount[b.At(i)]--
	}
	for _, count := range valueCount {
		if count != 0 {
			return false
		}
	}
	return true
}

// MapSlice is a view over a map whose values are slices.
type MapSlice[K comparable, V any] struct {
	// ж is the underlying mutable value, named with a hard-to-type
	// character that looks pointy like a pointer.
	// It is named distinctively to make you think of how dangerous it is to escape
	// to callers. You must not let callers be able to mutate it.
	ж map[K][]V
}

// MapSliceOf returns a MapSlice for the provided map. It is the caller's
// responsibility to make sure V is immutable.
func MapSliceOf[K comparable, V any](m map[K][]V) MapSlice[K, V] {
	return MapSlice[K, V]{m}
}

// Contains reports whether k has an entry in the map.
func (m MapSlice[K, V]) Contains(k K) bool {
	_, ok := m.ж[k]
	return ok
}

// IsNil reports whether the underlying map is nil.
func (m MapSlice[K, V]) IsNil() bool {
	return m.ж == nil
}

// Len returns the number of elements in the map.
func (m MapSlice[K, V]) Len() int { return len(m.ж) }

// Get returns the element with key k.
func (m MapSlice[K, V]) Get(k K) Slice[V] {
	return SliceOf(m.ж[k])
}

// GetOk returns the element with key k and a bool representing whether the key
// is in map.
func (m MapSlice[K, V]) GetOk(k K) (Slice[V], bool) {
	v, ok := m.ж[k]
	return SliceOf(v), ok
}

// MarshalJSON implements json.Marshaler.
func (m MapSlice[K, V]) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.ж)
}

// UnmarshalJSON implements json.Unmarshaler.
// It should only be called on an uninitialized Map.
func (m *MapSlice[K, V]) UnmarshalJSON(b []byte) error {
	if m.ж != nil {
		return errors.New("already initialized")
	}
	return json.Unmarshal(b, &m.ж)
}

// Range calls f for every k,v pair in the underlying map.
// It stops iteration immediately if f returns false.
func (m MapSlice[K, V]) Range(f MapRangeFn[K, Slice[V]]) {
	for k, v := range m.ж {
		if !f(k, SliceOf(v)) {
			return
		}
	}
}

// AsMap returns a shallow-clone of the underlying map.
//
// If V is a pointer type, it is the caller's responsibility to make sure the
// values are immutable. The map and slices are cloned, but the values are not.
func (m MapSlice[K, V]) AsMap() map[K][]V {
	if m.ж == nil {
		return nil
	}
	out := maps.Clone(m.ж)
	for k, v := range out {
		out[k] = slices.Clone(v)
	}
	return out
}

// All returns an iterator iterating over the keys and values of m.
func (m MapSlice[K, V]) All() iter.Seq2[K, Slice[V]] {
	return func(yield func(K, Slice[V]) bool) {
		for k, v := range m.ж {
			if !yield(k, SliceOf(v)) {
				return
			}
		}
	}
}

// Map provides a read-only view of a map. It is the caller's responsibility to
// make sure V is immutable.
type Map[K comparable, V any] struct {
	// ж is the underlying mutable value, named with a hard-to-type
	// character that looks pointy like a pointer.
	// It is named distinctively to make you think of how dangerous it is to escape
	// to callers. You must not let callers be able to mutate it.
	ж map[K]V
}

// MapOf returns a view over m. It is the caller's responsibility to make sure V
// is immutable.
func MapOf[K comparable, V any](m map[K]V) Map[K, V] {
	return Map[K, V]{m}
}

// Has reports whether k has an entry in the map.
// Deprecated: use Contains instead.
func (m Map[K, V]) Has(k K) bool {
	return m.Contains(k)
}

// Contains reports whether k has an entry in the map.
func (m Map[K, V]) Contains(k K) bool {
	_, ok := m.ж[k]
	return ok
}

// IsNil reports whether the underlying map is nil.
func (m Map[K, V]) IsNil() bool {
	return m.ж == nil
}

// Len returns the number of elements in the map.
func (m Map[K, V]) Len() int { return len(m.ж) }

// Get returns the element with key k.
func (m Map[K, V]) Get(k K) V {
	return m.ж[k]
}

// GetOk returns the element with key k and a bool representing whether the key
// is in map.
func (m Map[K, V]) GetOk(k K) (V, bool) {
	v, ok := m.ж[k]
	return v, ok
}

// MarshalJSON implements json.Marshaler.
func (m Map[K, V]) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.ж)
}

// UnmarshalJSON implements json.Unmarshaler.
// It should only be called on an uninitialized Map.
func (m *Map[K, V]) UnmarshalJSON(b []byte) error {
	if m.ж != nil {
		return errors.New("already initialized")
	}
	return json.Unmarshal(b, &m.ж)
}

// AsMap returns a shallow-clone of the underlying map.
// If V is a pointer type, it is the caller's responsibility to make sure
// the values are immutable.
func (m Map[K, V]) AsMap() map[K]V {
	if m.ж == nil {
		return nil
	}
	return maps.Clone(m.ж)
}

// MapRangeFn is the func called from a Map.Range call.
// Implementations should return false to stop range.
type MapRangeFn[K comparable, V any] func(k K, v V) (cont bool)

// Range calls f for every k,v pair in the underlying map.
// It stops iteration immediately if f returns false.
func (m Map[K, V]) Range(f MapRangeFn[K, V]) {
	for k, v := range m.ж {
		if !f(k, v) {
			return
		}
	}
}

// All returns an iterator iterating over the keys
// and values of m.
func (m Map[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for k, v := range m.ж {
			if !yield(k, v) {
				return
			}
		}
	}
}

// MapFnOf returns a MapFn for m.
func MapFnOf[K comparable, T any, V any](m map[K]T, f func(T) V) MapFn[K, T, V] {
	return MapFn[K, T, V]{
		ж:     m,
		wrapv: f,
	}
}

// MapFn is like Map but with a func to convert values from T to V.
// It is used to provide map of slices and views.
type MapFn[K comparable, T any, V any] struct {
	// ж is the underlying mutable value, named with a hard-to-type
	// character that looks pointy like a pointer.
	// It is named distinctively to make you think of how dangerous it is to escape
	// to callers. You must not let callers be able to mutate it.
	ж     map[K]T
	wrapv func(T) V
}

// Has reports whether k has an entry in the map.
// Deprecated: use Contains instead.
func (m MapFn[K, T, V]) Has(k K) bool {
	return m.Contains(k)
}

// Contains reports whether k has an entry in the map.
func (m MapFn[K, T, V]) Contains(k K) bool {
	_, ok := m.ж[k]
	return ok
}

// Get returns the element with key k.
func (m MapFn[K, T, V]) Get(k K) V {
	return m.wrapv(m.ж[k])
}

// IsNil reports whether the underlying map is nil.
func (m MapFn[K, T, V]) IsNil() bool {
	return m.ж == nil
}

// Len returns the number of elements in the map.
func (m MapFn[K, T, V]) Len() int { return len(m.ж) }

// GetOk returns the element with key k and a bool representing whether the key
// is in map.
func (m MapFn[K, T, V]) GetOk(k K) (V, bool) {
	v, ok := m.ж[k]
	return m.wrapv(v), ok
}

// Range calls f for every k,v pair in the underlying map.
// It stops iteration immediately if f returns false.
func (m MapFn[K, T, V]) Range(f MapRangeFn[K, V]) {
	for k, v := range m.ж {
		if !f(k, m.wrapv(v)) {
			return
		}
	}
}

// All returns an iterator iterating over the keys and value views of m.
func (m MapFn[K, T, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for k, v := range m.ж {
			if !yield(k, m.wrapv(v)) {
				return
			}
		}
	}
}

// ContainsPointers reports whether T contains any pointers,
// either explicitly or implicitly.
// It has special handling for some types that contain pointers
// that we know are free from memory aliasing/mutation concerns.
func ContainsPointers[T any]() bool {
	return containsPointers(reflect.TypeFor[T]())
}

func containsPointers(typ reflect.Type) bool {
	switch typ.Kind() {
	case reflect.Pointer, reflect.UnsafePointer:
		return true
	case reflect.Chan, reflect.Map, reflect.Slice:
		return true
	case reflect.Array:
		return containsPointers(typ.Elem())
	case reflect.Interface, reflect.Func:
		return true // err on the safe side.
	case reflect.Struct:
		if isWellKnownImmutableStruct(typ) {
			return false
		}
		for i := range typ.NumField() {
			if containsPointers(typ.Field(i).Type) {
				return true
			}
		}
	}
	return false
}

func isWellKnownImmutableStruct(typ reflect.Type) bool {
	switch typ.String() {
	case "time.Time":
		// time.Time contains a pointer that does not need copying
		return true
	case "netip.Addr", "netip.Prefix", "netip.AddrPort":
		return true
	default:
		return false
	}
}