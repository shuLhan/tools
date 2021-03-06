// Copyright 2019, Shulhan <ms@kilabit.info>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package memfs

import (
	"fmt"
	"os"
	"strings"
)

const (
	DefaultGenPackageName = "main"
	DefaultGenVarName     = "memFS"
	DefaultGenGoFileName  = "memfs_generate.go"
)

type generateData struct {
	Opts    *Options
	VarName string
	Node    *Node
	Nodes   map[string]*Node
}

//
// GoGenerate write the tree nodes as Go generated source file.
//
// If pkgName is not defined it will be default to "main".
//
// varName is the global variable name with type *memfs.MemFS which will be
// initialize by generated Go source code on init().
// The varName default to "memFS" if its empty.
//
// If out is not defined it will be default to "memfs_generate.go" and saved
// in current directory from where its called.
//
// If contentEncoding is not empty, it will encode the content of node and set
// the node ContentEncoding.
// List of available encoding is "gzip".
// For example, if contentEncoding is "gzip" it will compress the content of
// file using gzip and set Node.ContentEncoding to "gzip".
//
func (mfs *MemFS) GoGenerate(pkgName, varName, out, contentEncoding string) (err error) {
	if len(pkgName) == 0 {
		pkgName = DefaultGenPackageName
	}
	if len(varName) == 0 {
		varName = DefaultGenVarName
	}
	if len(out) == 0 {
		out = DefaultGenGoFileName
	}
	genData := &generateData{
		Opts:    mfs.Opts,
		VarName: varName,
		Nodes:   mfs.PathNodes.v,
	}

	tmpl, err := generateTemplate()
	if err != nil {
		return err
	}

	f, err := os.Create(out)
	if err != nil {
		return fmt.Errorf("memfs: GoGenerate: %w", err)
	}

	if len(contentEncoding) > 0 {
		err = mfs.ContentEncode(contentEncoding)
		if err != nil {
			return fmt.Errorf("GoGenerate: %w", err)
		}
	}

	names := mfs.ListNames()

	err = tmpl.ExecuteTemplate(f, templateNameHeader, pkgName)
	if err != nil {
		goto fail
	}

	for x := 0; x < len(names); x++ {
		// Ignore and delete the file from map if its the output
		// itself.
		if strings.HasSuffix(names[x], out) {
			delete(mfs.PathNodes.v, names[x])
			continue
		}

		genData.Node = mfs.PathNodes.v[names[x]]

		err = tmpl.ExecuteTemplate(f, templateNameGenerateNode, genData)
		if err != nil {
			goto fail
		}
	}

	err = tmpl.ExecuteTemplate(f, templateNamePathFuncs, genData)
	if err != nil {
		goto fail
	}

	err = f.Sync()
	if err != nil {
		goto fail
	}

	err = f.Close()
	if err != nil {
		return fmt.Errorf("memfs: GoGenerate: %w", err)
	}

	return nil
fail:
	_ = f.Close()
	return fmt.Errorf("memfs: GoGenerate: %w", err)
}
