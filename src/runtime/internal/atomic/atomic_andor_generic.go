//go:build s390x || loong64 || mips || mipsle || mips64 || mips64le

// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package atomic

//go:nosplit
//go:noinline
func And32(ptr *uint32, val uint32) uint32 {
	for {
		old := *ptr
		if Cas(ptr, old, old&val) {
			return old
		}
	}
}

//go:nosplit
//go:noinline
func Or32(ptr *uint32, val uint32) uint32 {
	for {
		old := *ptr
		if Cas(ptr, old, old|val) {
			return old
		}
	}
}

//go:nosplit
//go:noinline
func And64(ptr *uint64, val uint64) uint64 {
	for {
		old := *ptr
		if Cas64(ptr, old, old&val) {
			return old
		}
	}
}

//go:nosplit
//go:noinline
func Or64(ptr *uint64, val uint64) uint64 {
	for {
		old := *ptr
		if Cas64(ptr, old, old|val) {
			return old
		}
	}
}

//go:nosplit
//go:noinline
func Anduintptr(ptr *uintptr, val uintptr) uintptr {
	for {
		old := *ptr
		if Casuintptr(ptr, old, old&val) {
			return old
		}
	}
}

//go:nosplit
//go:noinline
func Oruintptr(ptr *uintptr, val uintptr) uintptr {
	for {
		old := *ptr
		if Casuintptr(ptr, old, old|val) {
			return old
		}
	}
}
