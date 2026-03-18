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

package model

import (
	"encoding/json"
	"strconv"
	"strings"
	"unsafe"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	dqlhtml "github.com/goplus/xgo/dql/html"
)

type NodeComments = map[string]string // "x.y.z" => "comment"

type codeComments struct {
	comments NodeComments
	names    []string
}

func newCodeComments() *codeComments {
	return &codeComments{
		comments: make(NodeComments),
	}
}

func (p *codeComments) TextNodeData(node *html.Node) string {
	const prefix = " boolean"
	data := node.Data
	if strings.HasPrefix(data, prefix) {
		data = ` "boolean"` + data[len(prefix):]
	}
	return data
}

func (p *codeComments) Filter(node *html.Node) bool {
	if node.DataAtom != atom.Span {
		return true
	}
	for _, a := range node.Attr {
		if a.Key == "class" {
			switch a.Val {
			case "hljs-attr":
				name := textOf(node)
				if v, e := strconv.Unquote(name); e == nil {
					name = v
				}
				p.names[len(p.names)-1] = name
				return true
			case "hljs-comment":
				names := p.names
				if last := len(names) - 1; names[last] == "" {
					names = names[:last]
				}
				name := strings.Join(names, ".")
				p.comments[name] = textOf(node)
				return false
			case "hljs-punctuation":
				switch textOf(node) {
				case "{":
					p.names = append(p.names, "")
				case "}":
					p.names = p.names[:len(p.names)-1]
				}
			}
		}
	}
	return true
}

func textOf(node *html.Node) string {
	c := node.FirstChild
	if c != nil && c.Type == html.TextNode {
		return c.Data
	}
	return ""
}

func parseCommentedJson(ns dqlhtml.NodeSet) (doc any, comments NodeComments, err error) {
	cc := newCodeComments()
	text, err := dqlhtml.Text(ns, false, cc)
	if err != nil {
		return
	}
	comments = cc.comments
	err = json.Unmarshal(unsafe.Slice(unsafe.StringData(text), len(text)), &doc)
	return
}
