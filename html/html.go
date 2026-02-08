package html

import (
	"bytes"
	"errors"
	"io"
	"iter"

	"github.com/goplus/dql/stream"
	"golang.org/x/net/html"
)

var (
	ErrNotFound      = errors.New("entity not found")
	ErrMultiEntities = errors.New("too many entities found")
)

// nopIter is a no-operation iterator that yields no values.
func nopIter[T any](yield func(T) bool) {}

// -----------------------------------------------------------------------------

// Value represents an attribute value or an error.
type Value = struct {
	X_0 string
	X_1 error
}

// ValueSet represents a set of attribute Values.
type ValueSet struct {
	Data iter.Seq[Value]
	Err  error
}

// XGo_Enum returns an iterator over the Values in the ValueSet.
func (p ValueSet) XGo_Enum() iter.Seq[Value] {
	if p.Err != nil {
		return nopIter[Value]
	}
	return p.Data
}

// XGo_0 returns the first value in the ValueSet, or ErrNotFound if the set is empty.
func (p ValueSet) XGo_0() (val string, err error) {
	if p.Err != nil {
		return "", p.Err
	}
	err = ErrNotFound
	p.Data(func(v Value) bool {
		val, err = v.X_0, v.X_1
		return false
	})
	return
}

// XGo_1 returns the first value in the ValueSet, or ErrNotFound if the set is empty.
// If there is more than one value in the set, ErrMultiEntities is returned.
func (p ValueSet) XGo_1() (val string, err error) {
	if p.Err != nil {
		return "", p.Err
	}
	first := true
	err = ErrNotFound
	p.Data(func(v Value) bool {
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
		return nopIter[*Node]
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
			ok := true
			p.Data(func(node *Node) bool {
				node.ChildNodes()(func(c *Node) bool {
					ok = yield(c)
					return ok
				})
				return ok
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
			ok := true
			p.Data(func(node *Node) bool {
				if ok = yield(node); ok {
					node.Descendants()(func(c *Node) bool {
						ok = yield(c)
						return ok
					})
				}
				return ok
			})
		},
	}
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
			p.Data(func(node *Node) bool {
				for _, attr := range node.Attr {
					if attr.Key == name {
						return yield(Value{X_0: attr.Val})
					}
				}
				yield(Value{X_1: ErrNotFound})
				return true
			})
		},
	}
}

// XGo_0 returns the first node in the NodeSet, or ErrNotFound if the set is empty.
func (p NodeSet) XGo_0() (val *Node, err error) {
	if p.Err != nil {
		return nil, p.Err
	}
	err = ErrNotFound
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
	err = ErrNotFound
	p.Data(func(n *Node) bool {
		if first {
			val, err = n, nil
			first = false
			return true
		}
		err = ErrMultiEntities
		return false
	})
	return
}

// -----------------------------------------------------------------------------
