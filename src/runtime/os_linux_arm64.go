// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

import "unsafe"

const (
	_AT_RANDOM = 25 // introduced in 2.6.29
)

var randomNumber uint32

func archauxv(tag, val uintptr) {
	switch tag {
	case _AT_RANDOM: // kernel provides a pointer to 16-bytes worth of random data
		startupRandomData = (*[16]byte)(unsafe.Pointer(val))[:]
		// the pointer provided may not be word aligned, so we must treat it
		// as a byte array.
		randomNumber = uint32(startupRandomData[4]) | uint32(startupRandomData[5])<<8 |
			uint32(startupRandomData[6])<<16 | uint32(startupRandomData[7])<<24
	}
}

//go:nosplit
func cputicks() int64 {
	// Currently cputicks() is used in blocking profiler and to seed fastrand1().
	// nanotime() is a poor approximation of CPU ticks that is enough for the profiler.
	// randomNumber provides better seeding of fastrand1.
	return nanotime() + int64(randomNumber)
}
