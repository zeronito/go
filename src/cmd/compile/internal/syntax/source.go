// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements source, a buffered rune reader
// which is specialized for the needs of the Go scanner:
// Contiguous sequences of runes (literals) are extracted
// directly as []byte without the need to re-encode the
// runes in UTF-8 (as would be necessary with bufio.Reader).

package syntax

import (
	"io"
	"unicode/utf8"
)

// buf [...read...|...|...unread...|s|...free...]
//         ^      ^   ^            ^
//         |      |   |            |
//        suf     r0  r            w

type source struct {
	src   io.Reader
	errh  ErrorHandler
	first error // first error encountered

	// source buffer
	buf         [4 << 10]byte
	offs        int   // source offset of buf
	r0, r, w    int   // previous/current read and write buf positions, excluding sentinel
	line0, line uint  // previous/current line
	col0, col   uint  // previous/current column
	err         error // pending io error

	// literal buffer
	lit []byte // literal prefix
	suf int    // literal suffix; suf >= 0 means we are scanning a literal
}

func (s *source) init(src io.Reader, errh ErrorHandler) {
	s.src = src
	s.errh = errh
	s.first = nil

	s.buf[0] = utf8.RuneSelf // terminate with sentinel
	s.offs = 0
	s.r0, s.r, s.w = 0, 0, 0
	s.line0, s.line = 1, 1
	s.col0, s.col = 1, 1
	s.err = nil

	s.lit = s.lit[:0]
	s.suf = -1
}

func (s *source) error(msg string) {
	s.error_at(s.line0, s.col0, msg)
}

func (s *source) error_at(line, col uint, msg string) {
	err := Error{line, col, msg}
	if s.first == nil {
		s.first = err
	}
	if s.errh == nil {
		panic(s.first)
	}
	s.errh(err)
}

// ungetr ungets the most recently read rune.
func (s *source) ungetr() {
	s.r, s.line, s.col = s.r0, s.line0, s.col0
}

// ungetr2 is like ungetr but enables a 2nd ungetr.
// It must not be called if one of the runes seen
// was a newline.
func (s *source) ungetr2() {
	s.ungetr()
	// line must not have changed
	s.r0--
	s.col0--
}

func (s *source) getr() rune {
redo:
	s.r0, s.line0, s.col0 = s.r, s.line, s.col

	// We could avoid at least one test that is always taken in the
	// for loop below by duplicating the common case code (ASCII)
	// here since we always have at least the sentinel (utf8.RuneSelf)
	// in the buffer. Measure and optimize if necessary.

	// make sure we have at least one rune in buffer, or we are at EOF
	for s.r+utf8.UTFMax > s.w && !utf8.FullRune(s.buf[s.r:s.w]) && s.err == nil && s.w-s.r < len(s.buf) {
		s.fill() // s.w-s.r < len(s.buf) => buffer is not full
	}

	// common case: ASCII and enough bytes
	// (invariant: s.buf[s.w] == utf8.RuneSelf)
	if b := s.buf[s.r]; b < utf8.RuneSelf {
		s.r++
		s.col++
		if b == 0 {
			s.error("invalid NUL character")
			goto redo
		}
		if b == '\n' {
			s.line++
			s.col = 1
		}
		return rune(b)
	}

	// EOF
	if s.r == s.w {
		if s.err != io.EOF {
			s.error(s.err.Error())
		}
		return -1
	}

	// uncommon case: not ASCII
	r, w := utf8.DecodeRune(s.buf[s.r:s.w])
	s.r += w
	s.col++

	if r == utf8.RuneError && w == 1 {
		s.error("invalid UTF-8 encoding")
		goto redo
	}

	// BOM's are only allowed as the first character in a file
	const BOM = 0xfeff
	if r == BOM {
		if s.r0 > 0 { // s.r0 is always > 0 after 1st character (fill will set it to 1)
			s.error("invalid BOM in the middle of the file")
		}
		goto redo
	}

	return r
}

func (s *source) fill() {
	// Slide unread bytes to beginning but preserve last read char
	// (for one ungetr call) plus one extra byte (for a 2nd ungetr
	// call, only for ".." character sequence and float literals
	// starting with ".").
	if s.r0 > 1 {
		// save literal prefix, if any
		// (We see at most one ungetr call while reading
		// a literal, so make sure s.r0 remains in buf.)
		if s.suf >= 0 {
			s.lit = append(s.lit, s.buf[s.suf:s.r0]...)
			s.suf = 1 // == s.r0 after slide below
		}
		s.offs += s.r0 - 1
		r := s.r - s.r0 + 1 // last read char plus one byte
		s.w = r + copy(s.buf[r:], s.buf[s.r:s.w])
		s.r = r
		s.r0 = 1
	}

	// read more data: try a limited number of times
	for i := 100; i > 0; i-- {
		n, err := s.src.Read(s.buf[s.w : len(s.buf)-1]) // -1 to leave space for sentinel
		if n < 0 {
			panic("negative read") // incorrect underlying io.Reader implementation
		}
		s.w += n
		if n > 0 || err != nil {
			s.buf[s.w] = utf8.RuneSelf // sentinel
			if err != nil {
				s.err = err
			}
			return
		}
	}

	s.err = io.ErrNoProgress
}

func (s *source) startLit() {
	s.suf = s.r0
	s.lit = s.lit[:0] // reuse lit
}

func (s *source) stopLit() []byte {
	lit := s.buf[s.suf:s.r]
	if len(s.lit) > 0 {
		lit = append(s.lit, lit...)
	}
	s.suf = -1 // no pending literal
	return lit
}
