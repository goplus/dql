/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package util

import (
	"errors"
	"iter"
)

var (
	ErrNotFound      = errors.New("entity not found")
	ErrMultiEntities = errors.New("too many entities found")
)

// -----------------------------------------------------------------------------

// NopIter is a no-operation iterator that yields no values.
func NopIter[T any](yield func(T) bool) {}

// NopIter2 is a no-operation iterator that yields no values.
func NopIter2[T any](yield func(string, T) bool) {}

// -----------------------------------------------------------------------------

// Value represents an attribute value or an error.
type Value[T any] = struct {
	X_0 T
	X_1 error
}

// ValueSet represents a set of attribute Values.
type ValueSet[T any] struct {
	Data iter.Seq[Value[T]]
	Err  error
}

// XGo_Enum returns an iterator over the Values in the ValueSet.
func (p ValueSet[T]) XGo_Enum() iter.Seq[Value[T]] {
	if p.Err != nil {
		return NopIter[Value[T]]
	}
	return p.Data
}

// XGo_0 returns the first value in the ValueSet, or ErrNotFound if the set is empty.
func (p ValueSet[T]) XGo_0() (val T, err error) {
	if p.Err != nil {
		err = p.Err
		return
	}
	err = ErrNotFound
	p.Data(func(v Value[T]) bool {
		val, err = v.X_0, v.X_1
		return false
	})
	return
}

// XGo_1 returns the first value in the ValueSet, or ErrNotFound if the set is empty.
// If there is more than one value in the set, ErrMultiEntities is returned.
func (p ValueSet[T]) XGo_1() (val T, err error) {
	if p.Err != nil {
		err = p.Err
		return
	}
	first := true
	err = ErrNotFound
	p.Data(func(v Value[T]) bool {
		if first {
			val, err = v.X_0, v.X_1
			first = false
			return true
		}
		err = ErrMultiEntities
		return false
	})
	return
}

// -----------------------------------------------------------------------------
