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

package maps

import (
	"iter"

	"github.com/goplus/dql/util"
)

// Value represents an attribute value or an error.
type Value = util.Value[any]

// ValueSet represents a set of attribute Values.
type ValueSet = util.ValueSet[any]

// -----------------------------------------------------------------------------

// Node represents a map[string]any node.
type Node = map[string]any

// NodeSet represents a set of map[string]any nodes.
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
// - map[string]any: creates a NodeSet containing the single provided node.
// - iter.Seq2[string, Node]: directly uses the provided sequence of nodes.
// - NodeSet: returns the provided NodeSet as is.
// If the source type is unsupported, it panics.
func Source(r any) (ret NodeSet) {
	switch v := r.(type) {
	case map[string]any:
		return New(v)
	case iter.Seq2[string, Node]:
		return NodeSet{Data: v}
	case NodeSet:
		return v
	default:
		panic("dql/maps.Source: unsupport source type")
	}
}

// XGo_Enum returns an iterator over the nodes in the NodeSet.
func (p NodeSet) XGo_Enum() iter.Seq2[string, Node] {
	if p.Err != nil {
		return util.NopIter2[Node]
	}
	return p.Data
}

// XGo_Node returns a NodeSet containing the child nodes with the specified name.
func (p NodeSet) XGo_Node(name string) NodeSet {
	if p.Err != nil {
		return p
	}
	return NodeSet{
		Data: func(yield func(string, Node) bool) {
			p.Data(func(_ string, node Node) bool {
				if v, ok := node[name].(map[string]any); ok {
					return yield(name, v)
				}
				return true
			})
		},
	}
}

// XGo_Child returns a NodeSet containing all child nodes of the nodes in the NodeSet.
func (p NodeSet) XGo_Child() NodeSet {
	if p.Err != nil {
		return p
	}
	return NodeSet{
		Data: func(yield func(string, Node) bool) {
			p.Data(func(_ string, node Node) bool {
				for k, v := range node {
					if child, ok := v.(map[string]any); ok {
						if !yield(k, child) {
							return false
						}
					}
				}
				return true
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

// rangeAnyNodes recursively yields the node and all its descendant nodes.
func rangeAnyNodes(key string, node Node, yield func(string, Node) bool) bool {
	if !yield(key, node) {
		return false
	}
	for k, v := range node {
		if child, ok := v.(map[string]any); ok {
			if !rangeAnyNodes(k, child, yield) {
				return false
			}
		}
	}
	return true
}

// XGo_Attr returns a ValueSet containing the values of the specified attribute
// for each node in the NodeSet. If a node does not have the specified attribute,
// the Value will contain ErrNotFound.
func (p NodeSet) XGo_Attr(name string) ValueSet {
	if p.Err != nil {
		return ValueSet{Err: p.Err}
	}
	return ValueSet{
		Data: func(yield func(Value) bool) {
			p.Data(func(_ string, node Node) bool {
				for k, v := range node {
					if k == name {
						return yield(Value{X_0: v})
					}
				}
				yield(Value{X_1: util.ErrNotFound})
				return true
			})
		},
	}
}

// XGo_0 returns the first node in the NodeSet, or ErrNotFound if the set is empty.
func (p NodeSet) XGo_0() (key string, val Node, err error) {
	if p.Err != nil {
		return "", nil, p.Err
	}
	err = util.ErrNotFound
	p.Data(func(k string, n Node) bool {
		key, val, err = k, n, nil
		return false
	})
	return
}

// XGo_1 returns the first node in the NodeSet, or ErrNotFound if the set is empty.
// If there is more than one node in the set, ErrMultiEntities is returned.
func (p NodeSet) XGo_1() (key string, val Node, err error) {
	if p.Err != nil {
		return "", nil, p.Err
	}
	first := true
	err = util.ErrNotFound
	p.Data(func(k string, n Node) bool {
		if first {
			key, val, err = k, n, nil
			first = false
			return true
		}
		err = util.ErrMultiEntities
		return false
	})
	return
}

// -----------------------------------------------------------------------------
