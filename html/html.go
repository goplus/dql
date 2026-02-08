package html

import (
	"iter"

	"golang.org/x/net/html"
)

// -----------------------------------------------------------------------------

// Node represents an HTML node.
type Node = html.Node

// NodeSet represents a set of HTML nodes.
type NodeSet struct {
	Data iter.Seq[*Node]
	Err  error
}

// -----------------------------------------------------------------------------
