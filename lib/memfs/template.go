// Copyright 2019, Shulhan <ms@kilabit.info>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package memfs

import (
	"fmt"
	"text/template"

	"github.com/shuLhan/share/lib/ascii"
	libbytes "github.com/shuLhan/share/lib/bytes"
)

const (
	templateNameHeader       = "HEADER"
	templateNameGenerateNode = "GENERATE_NODE"
	templateNamePathFuncs    = "PATH_FUNCS"
)

//
// generateTemplate generate the .go source template.
//
// The .go source template contains three sections: HEADER, GENERATE_NODE,
// and PATH_FUNCS.
//
// The HEADER section accept single parameter: the package name, as a string.
//
// The GENERATE_NODE section accept single parameter: the *Node, which then
// converted into function that return the *Node itself,
//
//	function generate{{ Node.Path}} (node *memfs.Node) {
//		node = &memfs.Node{
//			...
//		}
//	}
//
// Then Node itself then registered in memfs global variable
// "GeneratedPathNode".
//
// The PATH_FUNCS section generate the init() function that map each
// Node's Path with the function generated from GENERATE_NODE.
//
func generateTemplate() (tmpl *template.Template, err error) {
	var textTemplate = `{{ define "HEADER" -}}
// Code generated by github.com/shuLhan/share/lib/memfs DO NOT EDIT.

package {{.}}

import (
	"github.com/shuLhan/share/lib/memfs"
)
{{end}}
{{define "GENERATE_NODE"}}
func generate{{ funcname .Path | printf "%s"}}() *memfs.Node {
	node := &memfs.Node{
		SysPath:         "{{.SysPath}}",
		Path:            "{{.Path}}",
		ContentType:     "{{.ContentType}}",
		ContentEncoding: "{{.ContentEncoding}}",
{{- if .V }}
		V: []byte{
			{{range $x, $c := .V}}{{ if maxline $x }}{{ printf "\n\t\t\t" }}{{else if $x}} {{end}}{{ printf "%d," $c }}{{end}}
		},
{{- end }}
	}
	node.SetMode({{printf "%d" .Mode}})
	node.SetName("{{.Name}}")
	node.SetSize({{.Size}})
	return node
}
{{end}}
{{define "PATH_FUNCS"}}
func init() {
	memfs.GeneratedPathNode = memfs.NewPathNode()
{{- range $path, $node := .}}
	memfs.GeneratedPathNode.Set("{{$path}}", generate{{funcname $node.Path | printf "%s" }}())
{{- end}}
}
{{end}}
`
	tmplFuncs := template.FuncMap{
		"funcname": func(path string) []byte {
			return libbytes.InReplace([]byte(path), []byte(ascii.LettersNumber), '_')
		},
		"maxline": func(x int) bool {
			if x != 0 && x%16 == 0 {
				return true
			}
			return false
		},
	}

	tmpl, err = template.New("memfs").Funcs(tmplFuncs).Parse(textTemplate)
	if err != nil {
		return nil, fmt.Errorf("generateTemplate: %w", err)
	}

	return tmpl, nil
}
