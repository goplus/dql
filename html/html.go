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

package html

import (
	"bytes"
	"io"
	"iter"

	"github.com/goplus/dql/stream"
	"github.com/goplus/dql/util"
	"golang.org/x/net/html"
)

// Value represents an attribute value or an error.
type Value = util.Value[string]

// ValueSet represents a set of attribute Values.
type ValueSet = util.ValueSet[string]

// -----------------------------------------------------------------------------

// Node represents an HTML node.
type Node = html.Node

// NodeSet represents a set of HTML nodes.
type NodeSet struct {
	Data iter.Seq[*Node]
	Err  error
}

// New parses the HTML document from the provided reader and returns a NodeSet
// containing the root node. If there is an error during parsing, the NodeSet's
// Err field is set.
func New(r io.Reader) NodeSet {
	doc, err := html.Parse(r)
	if err != nil {
		return NodeSet{Err: err}
	}
	return NodeSet{
		Data: func(yield func(*Node) bool) {
			yield(doc)
		},
	}
}

// Source creates a NodeSet from various types of sources:
// - string: treated as an URL to read HTML content from.
// - []byte: treated as raw HTML content.
// - io.Reader: reads HTML content from the reader.
// - iter.Seq[*Node]: directly uses the provided sequence of nodes.
// - NodeSet: returns the provided NodeSet as is.
// If the source type is unsupported, it panics.
func Source(r any) (ret NodeSet) {
	switch v := r.(type) {
	case string:
		f, err := stream.Open(v)
		if err != nil {
			return NodeSet{Err: err}
		}
		defer f.Close()
		return New(f)
	case []byte:
		r := bytes.NewReader(v)
		return New(r)
	case io.Reader:
		return New(v)
	case iter.Seq[*Node]:
		return NodeSet{Data: v}
	case NodeSet:
		return v
	default:
		panic("dql/html.Source: unsupport source type")
	}
}

// XGo_Enum returns an iterator over the nodes in the NodeSet.
func (p NodeSet) XGo_Enum() iter.Seq[*Node] {
	if p.Err != nil {
		return util.NopIter[*Node]
	}
	return p.Data
}

// XGo_Node returns a NodeSet containing the child nodes with the specified name.
func (p NodeSet) XGo_Node(name string) NodeSet {
	if p.Err != nil {
		return p
	}
	return NodeSet{
		Data: func(yield func(*Node) bool) {
			p.Data(func(node *Node) bool {
				if node.Type == html.ElementNode && node.Data == name {
					return yield(node)
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
		Data: func(yield func(*Node) bool) {
			p.Data(func(n *Node) bool {
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					if !yield(c) {
						return false
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
		Data: func(yield func(*Node) bool) {
			p.Data(func(node *Node) bool {
				return rangeAnyNodes(node, yield)
			})
		},
	}
}

// rangeAnyNodes recursively yields the node and all its descendant nodes.
func rangeAnyNodes(n *Node, yield func(*Node) bool) bool {
	if !yield(n) {
		return false
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if !rangeAnyNodes(c, yield) {
			return false
		}
	}
	return true
}

// XGo_Attr returns a ValueSet containing the values of the specified attribute
// for each node in the NodeSet. If a node does not have the specified attribute,
// the Value will contain ErrNotFound.
// Special attribute names "_text", "_comment", and "_doctype" can be used to
// retrieve the text content of text nodes, comment nodes, and doctype nodes
// respectively (only first matched node returned).
func (p NodeSet) XGo_Attr(name string) ValueSet {
	if p.Err != nil {
		return ValueSet{Err: p.Err}
	}
	return ValueSet{
		Data: func(yield func(Value) bool) {
			p.Data(func(node *Node) bool {
				return nodeAttr(node, name, yield)
			})
		},
	}
}

// nodeAttr retrieves the value of the specified attribute from the node.
// It handles special attribute names for text, comment, and doctype nodes.
// If the attribute is found, it yields the value; otherwise, it yields ErrNotFound.
// Returns true to continue iteration, false to stop.
func nodeAttr(node *Node, name string, yield func(Value) bool) bool {
	var typ html.NodeType
	switch name {
	case "_text":
		typ = html.TextNode
	case "_comment":
		typ = html.CommentNode
	case "_doctype":
		typ = html.DoctypeNode
	default:
		for _, attr := range node.Attr {
			if attr.Key == name {
				return yield(Value{X_0: attr.Val})
			}
		}
		goto notFound
	}
	for c := node.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == typ { // only first matched node returned
			return yield(Value{X_0: c.Data})
		}
	}
notFound:
	yield(Value{X_1: util.ErrNotFound})
	return true
}

// XGo_0 returns the first node in the NodeSet, or ErrNotFound if the set is empty.
func (p NodeSet) XGo_0() (val *Node, err error) {
	if p.Err != nil {
		return nil, p.Err
	}
	err = util.ErrNotFound
	p.Data(func(n *Node) bool {
		val, err = n, nil
		return false
	})
	return
}

// XGo_1 returns the first node in the NodeSet, or ErrNotFound if the set is empty.
// If there is more than one node in the set, ErrMultiEntities is returned.
func (p NodeSet) XGo_1() (val *Node, err error) {
	if p.Err != nil {
		return nil, p.Err
	}
	first := true
	err = util.ErrNotFound
	p.Data(func(n *Node) bool {
		if first {
			val, err = n, nil
			first = false
			return true
		}
		err = util.ErrMultiEntities
		return false
	})
	return
}

// -----------------------------------------------------------------------------
