// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !arm64
// +build !arm64

package math

const haveArchAbs = false

func archAbs(x float64) float64 {
	panic("not implemented")
}
