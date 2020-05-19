// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build riscv64

#include "textflag.h"

/*
 * void crosscall2(void (*fn)(void*, int32, uintptr), void*, int32, uintptr)
 * Save registers and call fn with two arguments.
 */
TEXT crosscall2(SB),NOSPLIT|NOFRAME,$0
	/*
	 * Push arguments for fn (X10, X11, X12), along with all callee-save
	 * registers. Note that at procedure entry the first argument is at
	 * 8(X2).
	 */
	ADD	$(-8*31), X2
	MOV	X11, (8*1)(X2) // void*
	MOVW	X12, (8*2)(X2) // int32
	MOV	X13, (8*3)(X2) // uintptr
	MOV	X8, (8*4)(X2)
	MOV	X9, (8*5)(X2)
	MOV	X18, (8*6)(X2)
	MOV	X19, (8*7)(X2)
	MOV	X20, (8*8)(X2)
	MOV	X21, (8*9)(X2)
	MOV	X22, (8*10)(X2)
	MOV	X23, (8*11)(X2)
	MOV	X24, (8*12)(X2)
	MOV	X25, (8*13)(X2)
	MOV	X26, (8*14)(X2)
	MOV	X27, (8*15)(X2)
	MOV	X3, (8*16)(X2)
	MOV	X4, (8*17)(X2)
	MOV	X1, (8*18)(X2)
	MOVD	F8, (8*19)(X2)
	MOVD	F9, (8*20)(X2)
	MOVD	F18, (8*21)(X2)
	MOVD	F19, (8*22)(X2)
	MOVD	F20, (8*23)(X2)
	MOVD	F21, (8*24)(X2)
	MOVD	F22, (8*25)(X2)
	MOVD	F23, (8*26)(X2)
	MOVD	F24, (8*27)(X2)
	MOVD	F25, (8*28)(X2)
	MOVD	F26, (8*29)(X2)
	MOVD	F27, (8*30)(X2)

	// Initialize Go ABI environment
	// prepare SB register = PC & 0xffffffff00000000
	AUIPC	$0, X3
	SRL	$32, X3
	SLL	$32, X3
	CALL	runtime·load_g(SB)
	JALR	RA, X10

	MOV	(8*4)(X2), X8
	MOV	(8*5)(X2), X9
	MOV	(8*6)(X2), X18
	MOV	(8*7)(X2), X19
	MOV	(8*8)(X2), X20
	MOV	(8*9)(X2), X21
	MOV	(8*10)(X2), X22
	MOV	(8*11)(X2), X23
	MOV	(8*12)(X2), X24
	MOV	(8*13)(X2), X25
	MOV	(8*14)(X2), X26
	MOV	(8*15)(X2), X27
	MOV	(8*16)(X2), X3
	MOV	(8*17)(X2), X4
	MOV	(8*18)(X2), X1
	MOVD	(8*19)(X2), F8
	MOVD	(8*20)(X2), F9
	MOVD	(8*21)(X2), F18
	MOVD	(8*22)(X2), F19
	MOVD	(8*23)(X2), F20
	MOVD	(8*24)(X2), F21
	MOVD	(8*25)(X2), F22
	MOVD	(8*26)(X2), F23
	MOVD	(8*27)(X2), F24
	MOVD	(8*28)(X2), F25
	MOVD	(8*29)(X2), F26
	MOVD	(8*30)(X2), F27
	ADD	$(8*31), X2

	RET
