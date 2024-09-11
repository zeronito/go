// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "../../../../../runtime/textflag.h"

TEXT asmtest(SB),DUPOK|NOSPLIT,$0
lable1:
	BFPT	1(PC)			// 00050048
	BFPT	lable1	// BFPT 2	//1ffdff4b

lable2:
	BFPF	1(PC)			// 00040048
	BFPF	lable2	// BFPF 4 	// 1ffcff4b

	// relocation in play so the assembled offset should be 0
	JMP	foo(SB)			// 00000050

	JMP	(R4)			// 8000004c
	JMP	1(PC)			// 00040050
	MOVW	$65536, R4		// 04020014
	MOVW	$4096, R4		// 24000014
	MOVV	$65536, R4		// 04020014
	MOVV	$4096, R4		// 24000014
	MOVW	R4, R5			// 85001700
	MOVV	R4, R5			// 85001500
	MOVBU	R4, R5			// 85fc4303
	SUB	R4, R5, R6		// a6101100
	SUBV	R4, R5, R6		// a6901100
	ADD	R4, R5, R6		// a6101000
	ADDV	R4, R5, R6		// a6901000
	AND	R4, R5, R6		// a6901400
	SUB	R4, R5			// a5101100
	SUBV	R4, R5			// a5901100
	ADD	R4, R5			// a5101000
	ADDV	R4, R5			// a5901000
	AND	R4, R5			// a5901400
	NEGW	R4, R5			// 05101100
	NEGV	R4, R5			// 05901100
	SLL	R4, R5			// a5101700
	SLL	R4, R5, R6		// a6101700
	SRL	R4, R5			// a5901700
	SRL	R4, R5, R6	 	// a6901700
	SRA	R4, R5			// a5101800
	SRA	R4, R5, R6	 	// a6101800
	ROTR	R4, R5			// a5101b00
	ROTR	R4, R5, R6		// a6101b00
	SLLV	R4, R5			// a5901800
	SLLV	R4, R5, R6		// a6901800
	ROTRV	R4, R5			// a5901b00
	ROTRV	R4, R5, R6		// a6901b00
	CLO	R4, R5			// 85100000
	CLZ	R4, R5			// 85140000
	CPUCFG	R4, R5			// 856c0000
	ADDF	F4, F5			// a5900001
	ADDF	F4, F5, F6		// a6900001
	ABSF	F4, F5			// 85041401
	MOVVF	F4, F5			// 85181d01
	MOVF	F4, F5			// 85941401
	MOVD	F4, F5			// 85981401
	MOVW	R4, result+16(FP)	// 64608029
	MOVWU	R4, result+16(FP)	// 64608029
	MOVV	R4, result+16(FP)	// 6460c029
	MOVB	R4, result+16(FP)	// 64600029
	MOVBU	R4, result+16(FP)	// 64600029
	MOVW	R4, 1(R5)		// a4048029
	MOVWU	R4, 1(R5)		// a4048029
	MOVV	R4, 1(R5)		// a404c029
	MOVB	R4, 1(R5)		// a4040029
	MOVBU	R4, 1(R5)		// a4040029
	SC	R4, 1(R5)		// a4040021
	SCV	R4, 1(R5)		// a4040023
	MOVW	y+8(FP), R4		// 64408028
	MOVWU	y+8(FP), R4		// 6440802a
	MOVV	y+8(FP), R4		// 6440c028
	MOVB	y+8(FP), R4		// 64400028
	MOVBU	y+8(FP), R4		// 6440002a
	MOVW	1(R5), R4		// a4048028
	MOVWU	1(R5), R4		// a404802a
	MOVV	1(R5), R4		// a404c028
	MOVB	1(R5), R4		// a4040028
	MOVBU	1(R5), R4		// a404002a
	LL	1(R5), R4		// a4040020
	LLV	1(R5), R4		// a4040022
	MOVW	$4(R4), R5		// 8510c002
	MOVV	$4(R4), R5		// 8510c002
	MOVW	$-1, R4			// 04fcff02
	MOVV	$-1, R4			// 04fcff02
	MOVW	$1, R4			// 0404c002
	MOVV	$1, R4			// 0404c002
	ADD	$-1, R4, R5		// 85fcbf02
	ADD	$-1, R4			// 84fcbf02
	ADDV	$-1, R4, R5		// 85fcff02
	ADDV	$-1, R4			// 84fcff02
	AND	$1, R4, R5		// 85044003
	AND	$1, R4			// 84044003
	SLL	$4, R4, R5		// 85904000
	SLL	$4, R4			// 84904000
	SRL	$4, R4, R5		// 85904400
	SRL	$4, R4			// 84904400
	SRA	$4, R4, R5		// 85904800
	SRA	$4, R4			// 84904800
	ROTR	$4, R4, R5		// 85904c00
	ROTR	$4, R4			// 84904c00
	SLLV	$4, R4, R5		// 85104100
	SLLV	$4, R4			// 84104100
	ROTRV	$4, R4, R5		// 85104d00
	ROTRV	$4, R4			// 84104d00
	SYSCALL				// 00002b00
	BEQ	R4, R5, 1(PC)		// 85040058
	BEQ	R4, 1(PC)		// 80040040
	BEQ	R4, R0, 1(PC)		// 80040040
	BEQ	R0, R4, 1(PC)		// 80040040
	BNE	R4, R5, 1(PC)		// 8504005c
	BNE	R4, 1(PC)		// 80040044
	BNE	R4, R0, 1(PC)		// 80040044
	BNE	R0, R4, 1(PC)		// 80040044
	BLTU	R4, 1(PC)		// 80040068
	MOVF	y+8(FP), F4		// 6440002b
	MOVD	y+8(FP), F4		// 6440802b
	MOVF	1(F5), F4		// a404002b
	MOVD	1(F5), F4		// a404802b
	MOVF	F4, result+16(FP)	// 6460402b
	MOVD	F4, result+16(FP)	// 6460c02b
	MOVF	F4, 1(F5)		// a404402b
	MOVD	F4, 1(F5)		// a404c02b
	MOVW	R4, F5			// 85a41401
	MOVW	F4, R5			// 85b41401
	MOVV	R4, F5			// 85a81401
	MOVV	F4, R5			// 85b81401
	WORD	$74565			// 45230100
	BREAK				// 00002a00
	UNDEF				// 00002a00

	ANDN	R4, R5, R6		// a6901600
	ANDN	R4, R5			// a5901600
	ORN	R4, R5, R6		// a6101600
	ORN	R4, R5			// a5101600

	// mul
	MUL	R4, R5	  		// a5101c00
	MUL	R4, R5, R6	  	// a6101c00
	MULV	R4, R5	   		// a5901d00
	MULV	R4, R5, R6	   	// a6901d00
	MULVU	R4, R5			// a5901d00
	MULVU	R4, R5, R6		// a6901d00
	MULHV	R4, R5			// a5101e00
	MULHV	R4, R5, R6		// a6101e00
	MULHVU	R4, R5			// a5901e00
	MULHVU	R4, R5, R6	 	// a6901e00
	REMV	R4, R5	   		// a5902200
	REMV	R4, R5, R6	   	// a6902200
	REMVU	R4, R5			// a5902300
	REMVU	R4, R5, R6		// a6902300
	DIVV	R4, R5			// a5102200
	DIVV	R4, R5, R6	   	// a6102200
	DIVVU	R4, R5	 		// a5102300
	DIVVU	R4, R5, R6		// a6102300

	MOVH	R4, result+16(FP)	// 64604029
	MOVH	R4, 1(R5)		// a4044029
	MOVH	y+8(FP), R4		// 64404028
	MOVH	1(R5), R4		// a4044028
	MOVHU	R4, R5			// 8500cf00
	MOVHU	R4, result+16(FP)	// 64604029
	MOVHU	R4, 1(R5)		// a4044029
	MOVHU	y+8(FP), R4		// 6440402a
	MOVHU	1(R5), R4		// a404402a
	MULU	R4, R5	   		// a5101c00
	MULU	R4, R5, R6		// a6101c00
	MULH	R4, R5	   		// a5901c00
	MULH	R4, R5, R6	   	// a6901c00
	MULHU	R4, R5			// a5101d00
	MULHU	R4, R5, R6		// a6101d00
	REM	R4, R5	  		// a5902000
	REM	R4, R5, R6	  	// a6902000
	REMU	R4, R5	   		// a5902100
	REMU	R4, R5, R6	   	// a6902100
	DIV	R4, R5	  		// a5102000
	DIV	R4, R5, R6	  	// a6102000
	DIVU	R4, R5	   		// a5102100
	DIVU	R4, R5, R6	   	// a6102100
	SRLV	R4, R5 			// a5101900
	SRLV	R4, R5, R6 		// a6101900
	SRLV	$4, R4, R5		// 85104500
	SRLV	$4, R4			// 84104500
	SRLV	$32, R4, R5 		// 85804500
	SRLV	$32, R4			// 84804500

	MASKEQZ	R4, R5, R6		// a6101300
	MASKNEZ	R4, R5, R6		// a6901300

	MOVFD	F4, F5			// 85241901
	MOVDF	F4, F5			// 85181901
	MOVWF	F4, F5			// 85101d01
	MOVFW	F4, F5			// 85041b01
	MOVWD	F4, F5			// 85201d01
	MOVDW	F4, F5			// 85081b01
	NEGF	F4, F5			// 85141401
	NEGD	F4, F5			// 85181401
	ABSD	F4, F5			// 85081401
	TRUNCDW	F4, F5			// 85881a01
	TRUNCFW	F4, F5			// 85841a01
	SQRTF	F4, F5			// 85441401
	SQRTD	F4, F5			// 85481401

	DBAR	 			// 00007238
	NOOP	 			// 00004003

	CMPEQF	F4, F5, FCC0		// a010120c
	CMPGTF	F4, F5, FCC1 		// a190110c
	CMPGTD	F4, F5, FCC2 		// a290210c
	CMPGEF	F4, F5, FCC3		// a390130c
	CMPGED	F4, F5, FCC4		// a490230c
	CMPEQD	F4, F5, FCC5		// a510220c

	RDTIMELW R4, R0			// 80600000
	RDTIMEHW R4, R0			// 80640000
	RDTIMED  R4, R5			// 85680000

	MOVV	R4, FCSR3		// 83c01401
	MOVV	FCSR3, R4		// 64c81401
	MOVV	F4, FCC0		// 80d01401
	MOVV	FCC0, F4		// 04d41401
	MOVV    FCC0, R4		// 04dc1401
	MOVV    R4, FCC0		// 80d81401

	// Loong64 atomic memory access instructions
	AMSWAPB		R14, (R13), R12 // ac395c38
	AMSWAPH		R14, (R13), R12 // acb95c38
	AMSWAPW		R14, (R13), R12 // ac396038
	AMSWAPV		R14, (R13), R12 // acb96038
	AMCASB		R14, (R13), R12 // ac395838
	AMCASH		R14, (R13), R12 // acb95838
	AMCASW		R14, (R13), R12 // ac395938
	AMCASV		R14, (R13), R12 // acb95938
	AMADDW		R14, (R13), R12 // ac396138
	AMADDV		R14, (R13), R12 // acb96138
	AMANDW		R14, (R13), R12 // ac396238
	AMANDV		R14, (R13), R12 // acb96238
	AMORW		R14, (R13), R12 // ac396338
	AMORV		R14, (R13), R12 // acb96338
	AMXORW		R14, (R13), R12 // ac396438
	AMXORV		R14, (R13), R12 // acb96438
	AMMAXW		R14, (R13), R12 // ac396538
	AMMAXV		R14, (R13), R12 // acb96538
	AMMINW		R14, (R13), R12 // ac396638
	AMMINV		R14, (R13), R12 // acb96638
	AMMAXWU		R14, (R13), R12 // ac396738
	AMMAXVU		R14, (R13), R12 // acb96738
	AMMINWU		R14, (R13), R12 // ac396838
	AMMINVU		R14, (R13), R12 // acb96838
	AMSWAPDBB	R14, (R13), R12 // ac395e38
	AMSWAPDBH	R14, (R13), R12 // acb95e38
	AMSWAPDBW	R14, (R13), R12 // ac396938
	AMSWAPDBV	R14, (R13), R12 // acb96938
	AMCASDBB	R14, (R13), R12 // ac395a38
	AMCASDBH	R14, (R13), R12 // acb95a38
	AMCASDBW	R14, (R13), R12 // ac395b38
	AMCASDBV	R14, (R13), R12 // acb95b38
	AMADDDBW	R14, (R13), R12 // ac396a38
	AMADDDBV	R14, (R13), R12 // acb96a38
	AMANDDBW	R14, (R13), R12 // ac396b38
	AMANDDBV	R14, (R13), R12 // acb96b38
	AMORDBW		R14, (R13), R12 // ac396c38
	AMORDBV		R14, (R13), R12 // acb96c38
	AMXORDBW	R14, (R13), R12 // ac396d38
	AMXORDBV	R14, (R13), R12 // acb96d38
	AMMAXDBW	R14, (R13), R12 // ac396e38
	AMMAXDBV	R14, (R13), R12 // acb96e38
	AMMINDBW	R14, (R13), R12 // ac396f38
	AMMINDBV	R14, (R13), R12 // acb96f38
	AMMAXDBWU	R14, (R13), R12 // ac397038
	AMMAXDBVU	R14, (R13), R12 // acb97038
	AMMINDBWU	R14, (R13), R12 // ac397138
	AMMINDBVU	R14, (R13), R12 // acb97138

	FMINF	F4, F5, F6		// a6900a01
	FMINF	F4, F5			// a5900a01
	FMIND	F4, F5, F6		// a6100b01
	FMIND	F4, F5			// a5100b01
	FMAXF	F4, F5, F6		// a6900801
	FMAXF	F4, F5			// a5900801
	FMAXD	F4, F5, F6		// a6100901
	FMAXD	F4, F5			// a5100901

	FCOPYSGF	F4, F5, F6	// a6901201
	FCOPYSGD	F4, F5, F6	// a6101301
	FCLASSF		F4, F5		// 85341401
	FCLASSD		F4, F5		// 85381401

	FFINTFW		F0, F1		// 01101d01
	FFINTFV		F0, F1		// 01181d01
	FFINTDW		F0, F1		// 01201d01
	FFINTDV		F0, F1		// 01281d01
	FTINTWF		F0, F1		// 01041b01
	FTINTWD		F0, F1		// 01081b01
	FTINTVF		F0, F1		// 01241b01
	FTINTVD		F0, F1		// 01281b01

	FTINTRMWF	F0, F2		// 02041a01
	FTINTRMWD	F0, F2		// 02081a01
	FTINTRMVF	F0, F2		// 02241a01
	FTINTRMVD	F0, F2		// 02281a01
	FTINTRPWF	F0, F2		// 02441a01
	FTINTRPWD	F0, F2		// 02481a01
	FTINTRPVF	F0, F2		// 02641a01
	FTINTRPVD	F0, F2		// 02681a01
	FTINTRZWF	F0, F2		// 02841a01
	FTINTRZWD	F0, F2		// 02881a01
	FTINTRZVF	F0, F2		// 02a41a01
	FTINTRZVD	F0, F2		// 02a81a01
	FTINTRNEWF	F0, F2		// 02c41a01
	FTINTRNEWD	F0, F2		// 02c81a01
	FTINTRNEVF	F0, F2		// 02e41a01
	FTINTRNEVD	F0, F2		// 02e81a01

	// LDX.{B,BU,H,HU,W,WU,D} instructions
	MOVB		(R14)(R13), R12	// cc350038
	MOVBU		(R14)(R13), R12	// cc352038
	MOVH		(R14)(R13), R12	// cc350438
	MOVHU		(R14)(R13), R12	// cc352438
	MOVW		(R14)(R13), R12	// cc350838
	MOVWU		(R14)(R13), R12	// cc352838
	MOVV		(R14)(R13), R12	// cc350c38

	// STX.{B,H,W,D} instructions
	MOVB		R12, (R14)(R13)	// cc351038
	MOVH		R12, (R14)(R13)	// cc351438
	MOVW		R12, (R14)(R13)	// cc351838
	MOVV		R12, (R14)(R13)	// cc351c38

	// FLDX.{S,D} instructions
	MOVF		(R14)(R13), F2	// c2353038
	MOVD		(R14)(R13), F2	// c2353438

	// FSTX.{S,D} instructions
	MOVF		F2, (R14)(R13)	// c2353838
	MOVD		F2, (R14)(R13)	// c2353c38

	BSTRINSW	$0, R4, $0, R5	// 85006000
	BSTRINSW	$31, R4, $0, R5	// 85007f00
	BSTRINSW	$15, R4, $6, R5	// 85186f00
	BSTRINSV	$0, R4, $0, R5	// 85008000
	BSTRINSV	$63, R4, $0, R5	// 8500bf00
	BSTRINSV	$15, R4, $6, R5	// 85188f00

	BSTRPICKW	$0, R4, $0, R5	// 85806000
	BSTRPICKW	$31, R4, $0, R5	// 85807f00
	BSTRPICKW	$15, R4, $6, R5	// 85986f00
	BSTRPICKV	$0, R4, $0, R5	// 8500c000
	BSTRPICKV	$63, R4, $0, R5	// 8500ff00
	BSTRPICKV	$15, R4, $6, R5	// 8518cf00
