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

package reflects

import (
	"iter"
	"reflect"

	"github.com/goplus/dql"
)

// capitalize capitalizes the first letter of the given name.
func capitalize(name string) string {
	if name != "" {
		if c := name[0]; c >= 'a' && c <= 'z' {
			return string(c-'a'+'A') + name[1:]
		}
	}
	return name
}

// uncapitalize uncapitalizes the first letter of the given name.
func uncapitalize(name string) string {
	if name != "" {
		if c := name[0]; c >= 'A' && c <= 'Z' {
			return string(c-'A'+'a') + name[1:]
		}
	}
	return name
}

// -----------------------------------------------------------------------------

// Node represents a reflect.Value node.
type Node = reflect.Value

// NodeSet represents a set of reflect.Value nodes.
type NodeSet struct {
	Data iter.Seq2[string, Node]
	Err  error
}

// New creates a NodeSet containing a single provided node.
func New(doc Node) NodeSet {
	return NodeSet{
		Data: func(yield func(string, Node) bool) {
			yield("", doc)
		},
	}
}

// Source creates a NodeSet from various types of sources:
// - reflect.Value: creates a NodeSet containing the single provided node.
// - iter.Seq2[string, Node]: directly uses the provided sequence of nodes.
// - NodeSet: returns the provided NodeSet as is.
// - any other type: uses reflect.ValueOf to create a NodeSet.
func Source(r any) (ret NodeSet) {
	switch v := r.(type) {
	case reflect.Value:
		return New(v)
	case iter.Seq2[string, Node]:
		return NodeSet{Data: v}
	case NodeSet:
		return v
	default:
		return New(reflect.ValueOf(r))
	}
}

// XGo_Enum returns an iterator over the nodes in the NodeSet.
func (p NodeSet) XGo_Enum() iter.Seq2[string, Node] {
	if p.Err != nil {
		return dql.NopIter2[Node]
	}
	return p.Data
}

// XGo_Select returns a NodeSet containing the nodes with the specified name.
//   - @name
//   - @"element-name"
func (p NodeSet) XGo_Select(name string) NodeSet {
	if p.Err != nil {
		return p
	}
	return NodeSet{
		Data: func(yield func(string, Node) bool) {
			p.Data(func(key string, node Node) bool {
				if key == name {
					return yield(key, node)
				}
				return true
			})
		},
	}
}

// XGo_Elem returns a NodeSet containing the child nodes with the specified name.
//   - .name
//   - .“element-name”
func (p NodeSet) XGo_Elem(name string) NodeSet {
	if p.Err != nil {
		return p
	}
	return NodeSet{
		Data: func(yield func(string, Node) bool) {
			p.Data(func(_ string, node Node) bool {
				return yieldNode(node, name, yield)
			})
		},
	}
}

// yieldNode yields the child node with the specified name if it exists.
func yieldNode(node Node, name string, yield func(string, Node) bool) bool {
	if v := lookup(node, name); isNode(v) {
		return yield(name, v)
	}
	return true
}

func lookup(node Node, name string) (ret Node) {
	kind := node.Kind()
	switch kind {
	case reflect.Pointer, reflect.Interface:
		node = node.Elem()
		kind = node.Kind()
	}
	switch kind {
	case reflect.Struct:
		ret = node.FieldByName(capitalize(name))
	case reflect.Map:
		ret = node.MapIndex(reflect.ValueOf(name))
	}
	return
}

func isNode(v reflect.Value) bool {
	kind := v.Kind()
	switch kind {
	case reflect.Invalid:
		return false
	case reflect.Pointer, reflect.Interface:
		v = v.Elem()
		kind = v.Kind()
	}
	return kind == reflect.Struct || kind == reflect.Map
}

func rangeChildNodes(node Node, yield func(string, Node) bool) bool {
	kind := node.Kind()
	switch kind {
	case reflect.Pointer, reflect.Interface:
		node = node.Elem()
		kind = node.Kind()
	}
	switch kind {
	case reflect.Struct:
		typ := node.Type()
		for i := 0; i < typ.NumField(); i++ {
			v := node.Field(i)
			if isNode(v) {
				if !yield(uncapitalize(typ.Field(i).Name), v) {
					return false
				}
			}
		}
	case reflect.Map:
		typ := node.Type()
		if typ.Key().Kind() != reflect.String {
			return true // only string keys are supported
		}
		it := node.MapRange()
		for it.Next() {
			v := it.Value()
			if isNode(v) {
				if !yield(it.Key().String(), v) {
					return false
				}
			}
		}
	}
	return true
}

// rangeAnyNodes recursively yields the node and all its descendant nodes.
func rangeAnyNodes(key string, node Node, yield func(string, Node) bool) bool {
	if !yield(key, node) {
		return false
	}
	return rangeChildNodes(node, func(k string, n Node) bool {
		return rangeAnyNodes(k, n, yield)
	})
}

// XGo_Child returns a NodeSet containing all child nodes of the nodes in the NodeSet.
func (p NodeSet) XGo_Child() NodeSet {
	if p.Err != nil {
		return p
	}
	return NodeSet{
		Data: func(yield func(string, Node) bool) {
			p.Data(func(_ string, node Node) bool {
				return rangeChildNodes(node, yield)
			})
		},
	}
}

// XGo_Any returns a NodeSet containing all descendant nodes of the nodes in
// the NodeSet, including the nodes themselves.
func (p NodeSet) XGo_Any() NodeSet {
	if p.Err != nil {
		return p
	}
	return NodeSet{
		Data: func(yield func(string, Node) bool) {
			p.Data(func(key string, node Node) bool {
				return rangeAnyNodes(key, node, yield)
			})
		},
	}
}

// XGo_Attr returns the value of the first specified attribute found in the NodeSet.
//   - $name
//   - $“attr-name”
func (p NodeSet) XGo_Attr(name string) (val any, err error) {
	if p.Err != nil {
		return "", p.Err
	}
	err = dql.ErrNotFound
	p.Data(func(_ string, node Node) bool {
		if v := lookup(node, name); v.IsValid() {
			val, err = v.Interface(), nil
			return false
		}
		return true
	})
	return
}

// -----------------------------------------------------------------------------
