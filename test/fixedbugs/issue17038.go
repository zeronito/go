// errorcheck

// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

const A = complex(0()) // ERROR "cannot call non-function" "const initializer .* is not a constant" "not enough arguments"
