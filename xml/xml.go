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

package xml

import (
	"bytes"
	"encoding/xml"
	"io"
	"iter"
	"unsafe"

	"github.com/goplus/dql/stream"
	"github.com/goplus/dql/util"
)

// Value represents an attribute value or an error.
type Value = util.Value[string]

// ValueSet represents a set of attribute Values.
type ValueSet = util.ValueSet[string]

// -----------------------------------------------------------------------------

// Node represents a generic XML node with its name, attributes, and children.
type Node struct {
	Name     xml.Name
	Attr     []xml.Attr
	Children []any // can be *Node or xml.CharData
}

// UnmarshalXML implements the xml.Unmarshaler interface for the Node struct.
func (n *Node) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	n.Name = start.Name
	n.Attr = start.Attr
	for {
		token, err := d.Token()
		if err != nil {
			return err
		}

		switch t := token.(type) {
		case xml.StartElement:
			child := &Node{}
			if err := d.DecodeElement(child, &t); err != nil {
				return err
			}
			n.Children = append(n.Children, child)

		case xml.CharData:
			n.Children = append(n.Children, t)

		case xml.EndElement:
			return nil
		}
	}
}

// -----------------------------------------------------------------------------

// NodeSet represents a set of XML nodes.
type NodeSet struct {
	Data iter.Seq[*Node]
	Err  error
}

// New parses the XML document from the provided reader and returns a NodeSet
// containing the root node. If there is an error during parsing, the NodeSet's
// Err field is set.
func New(r io.Reader) NodeSet {
	var doc Node
	err := xml.NewDecoder(r).Decode(&doc)
	if err != nil {
		return NodeSet{Err: err}
	}
	return NodeSet{
		Data: func(yield func(*Node) bool) {
			yield(&doc)
		},
	}
}

// Source creates a NodeSet from various types of sources:
// - string: treated as an URL to read XML content from.
// - []byte: treated as raw XML content.
// - io.Reader: reads XML content from the reader.
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
		panic("dql/xml.Source: unsupport source type")
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
				return yieldNode(node, name, yield)
			})
		},
	}
}

// yieldNode yields the child node with the specified name if it exists.
func yieldNode(n *Node, name string, yield func(*Node) bool) bool {
	for _, c := range n.Children {
		if child, ok := c.(*Node); ok {
			if child.Name.Local == name {
				if !yield(child) {
					return false
				}
			}
		}
	}
	return true
}

// XGo_Child returns a NodeSet containing all child nodes of the nodes in the NodeSet.
func (p NodeSet) XGo_Child() NodeSet {
	if p.Err != nil {
		return p
	}
	return NodeSet{
		Data: func(yield func(*Node) bool) {
			p.Data(func(n *Node) bool {
				return rangeChildNodes(n, yield)
			})
		},
	}
}

// rangeChildNodes yields all child nodes of the given node.
func rangeChildNodes(n *Node, yield func(*Node) bool) bool {
	for _, c := range n.Children {
		if child, ok := c.(*Node); ok {
			if !yield(child) {
				return false
			}
		}
	}
	return true
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
	for _, c := range n.Children {
		if child, ok := c.(*Node); ok {
			if !rangeAnyNodes(child, yield) {
				return false
			}
		}
	}
	return true
}

// XGo_Attr returns a ValueSet containing the values of the specified attribute
// for each node in the NodeSet. If a node does not have the specified attribute,
// the Value will contain ErrNotFound.
// If the attribute name is "_text", it retrieves the text content of the node.
func (p NodeSet) XGo_Attr(name string) ValueSet {
	if p.Err != nil {
		return ValueSet{Err: p.Err}
	}
	return ValueSet{
		Data: func(yield func(Value) bool) {
			p.Data(func(node *Node) bool {
				return yieldAttr(node, name, yield)
			})
		},
	}
}

// yieldAttr retrieves the value of the specified attribute from the node.
// If the attribute is not found, it yields a Value with ErrNotFound.
// If the attribute name is "_text", it retrieves the text content of
// the node.
func yieldAttr(node *Node, name string, yield func(Value) bool) bool {
	switch name {
	case "_text":
		for _, c := range node.Children {
			if data, ok := c.(xml.CharData); ok { // only first matched textNode returned
				text := unsafe.String(unsafe.SliceData(data), len(data))
				return yield(Value{X_0: text})
			}
		}
	default:
		for _, attr := range node.Attr {
			if attr.Name.Local == name {
				return yield(Value{X_0: attr.Value})
			}
		}
	}
	return yield(Value{X_1: util.ErrNotFound})
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
