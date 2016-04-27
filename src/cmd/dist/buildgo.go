// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"sort"
)

/*
 * Helpers for building cmd/go and cmd/cgo.
 */

// mkzdefaultcc writes zdefaultcc.go:
//
//	package main
//	const defaultCC = <defaultcc>
//	const defaultCXX = <defaultcxx>
//
// It is invoked to write cmd/go/zdefaultcc.go
// but we also write cmd/cgo/zdefaultcc.go
func mkzdefaultcc(dir, file string) {
	var out string

	out = fmt.Sprintf(
		"// auto generated by go tool dist\n"+
			"\n"+
			"package main\n"+
			"\n"+
			"const defaultCC = `%s`\n"+
			"const defaultCXX = `%s`\n",
		defaultcctarget, defaultcxxtarget)

	writefile(out, file, writeSkipSame)

	// Convert file name to replace: turn go into cgo.
	i := len(file) - len("go/zdefaultcc.go")
	file = file[:i] + "c" + file[i:]
	writefile(out, file, writeSkipSame)
}

// mkzcgo writes zcgo.go for go/build package:
//
//	package build
//  var cgoEnabled = map[string]bool{}
//
// It is invoked to write go/build/zcgo.go.
func mkzcgo(dir, file string) {
	// sort for deterministic zcgo.go file
	var list []string
	for plat, hasCgo := range cgoEnabled {
		if hasCgo {
			list = append(list, plat)
		}
	}
	sort.Strings(list)

	var buf bytes.Buffer

	fmt.Fprintf(&buf,
		"// auto generated by go tool dist\n"+
			"\n"+
			"package build\n"+
			"\n"+
			"var cgoEnabled = map[string]bool{\n")
	for _, plat := range list {
		fmt.Fprintf(&buf, "\t%q: true,\n", plat)
	}
	fmt.Fprintf(&buf, "}")

	writefile(buf.String(), file, writeSkipSame)
}
