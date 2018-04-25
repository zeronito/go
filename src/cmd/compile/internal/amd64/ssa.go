// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package amd64

import (
	"fmt"
	"math"

	"cmd/compile/internal/gc"
	"cmd/compile/internal/ssa"
	"cmd/compile/internal/types"
	"cmd/internal/obj"
	"cmd/internal/obj/x86"
)

// markMoves marks any MOVXconst ops that need to avoid clobbering flags.
func ssaMarkMoves(s *gc.SSAGenState, b *ssa.Block) {
	flive := b.FlagsLiveAtEnd
	if b.Control != nil && b.Control.Type.IsFlags() {
		flive = true
	}
	for i := len(b.Values) - 1; i >= 0; i-- {
		v := b.Values[i]
		if flive && (v.Op == ssa.OpAMD64MOVLconst || v.Op == ssa.OpAMD64MOVQconst) {
			// The "mark" is any non-nil Aux value.
			v.Aux = v
		}
		if v.Type.IsFlags() {
			flive = false
		}
		for _, a := range v.Args {
			if a.Type.IsFlags() {
				flive = true
			}
		}
	}
}

// loadByType returns the load instruction of the given type.
func loadByType(t *types.Type) obj.As {
	// Avoid partial register write
	if !t.IsFloat() && t.Size() <= 2 {
		if t.Size() == 1 {
			return x86.AMOVBLZX
		} else {
			return x86.AMOVWLZX
		}
	}
	// Otherwise, there's no difference between load and store opcodes.
	return storeByType(t)
}

// storeByType returns the store instruction of the given type.
func storeByType(t *types.Type) obj.As {
	width := t.Size()
	if t.IsFloat() {
		switch width {
		case 4:
			return x86.AMOVSS
		case 8:
			return x86.AMOVSD
		}
	} else {
		switch width {
		case 1:
			return x86.AMOVB
		case 2:
			return x86.AMOVW
		case 4:
			return x86.AMOVL
		case 8:
			return x86.AMOVQ
		}
	}
	panic("bad store type")
}

// moveByType returns the reg->reg move instruction of the given type.
func moveByType(t *types.Type) obj.As {
	if t.IsFloat() {
		// Moving the whole sse2 register is faster
		// than moving just the correct low portion of it.
		// There is no xmm->xmm move with 1 byte opcode,
		// so use movups, which has 2 byte opcode.
		return x86.AMOVUPS
	} else {
		switch t.Size() {
		case 1:
			// Avoids partial register write
			return x86.AMOVL
		case 2:
			return x86.AMOVL
		case 4:
			return x86.AMOVL
		case 8:
			return x86.AMOVQ
		case 16:
			return x86.AMOVUPS // int128s are in SSE registers
		default:
			panic(fmt.Sprintf("bad int register width %d:%s", t.Size(), t))
		}
	}
}

// opregreg emits instructions for
//     dest := dest(To) op src(From)
// and also returns the created obj.Prog so it
// may be further adjusted (offset, scale, etc).
func opregreg(s *gc.SSAGenState, op obj.As, dest, src int16) *obj.Prog {
	p := s.Prog(op)
	p.From.Type = obj.TYPE_REG
	p.To.Type = obj.TYPE_REG
	p.To.Reg = dest
	p.From.Reg = src
	return p
}

// DUFFZERO consists of repeated blocks of 4 MOVUPSs + LEAQ,
// See runtime/mkduff.go.
func duffStart(size int64) int64 {
	x, _ := duff(size)
	return x
}
func duffAdj(size int64) int64 {
	_, x := duff(size)
	return x
}

// duff returns the offset (from duffzero, in bytes) and pointer adjust (in bytes)
// required to use the duffzero mechanism for a block of the given size.
func duff(size int64) (int64, int64) {
	if size < 32 || size > 1024 || size%dzClearStep != 0 {
		panic("bad duffzero size")
	}
	steps := size / dzClearStep
	blocks := steps / dzBlockLen
	steps %= dzBlockLen
	off := dzBlockSize * (dzBlocks - blocks)
	var adj int64
	if steps != 0 {
		off -= dzLeaqSize
		off -= dzMovSize * steps
		adj -= dzClearStep * (dzBlockLen - steps)
	}
	return off, adj
}

func ssaGenValue(s *gc.SSAGenState, v *ssa.Value) {
	switch v.Op {
	case ssa.OpAMD64ADDQ, ssa.OpAMD64ADDL:
		r := v.Reg()
		r1 := v.Args[0].Reg()
		r2 := v.Args[1].Reg()
		switch {
		case r == r1:
			p := s.Prog(v.Op.Asm())
			p.From.Type = obj.TYPE_REG
			p.From.Reg = r2
			p.To.Type = obj.TYPE_REG
			p.To.Reg = r
		case r == r2:
			p := s.Prog(v.Op.Asm())
			p.From.Type = obj.TYPE_REG
			p.From.Reg = r1
			p.To.Type = obj.TYPE_REG
			p.To.Reg = r
		default:
			var asm obj.As
			if v.Op == ssa.OpAMD64ADDQ {
				asm = x86.ALEAQ
			} else {
				asm = x86.ALEAL
			}
			p := s.Prog(asm)
			p.From.Type = obj.TYPE_MEM
			p.From.Reg = r1
			p.From.Scale = 1
			p.From.Index = r2
			p.To.Type = obj.TYPE_REG
			p.To.Reg = r
		}
	// 2-address opcode arithmetic
	case ssa.OpAMD64SUBQ, ssa.OpAMD64SUBL,
		ssa.OpAMD64MULQ, ssa.OpAMD64MULL,
		ssa.OpAMD64ANDQ, ssa.OpAMD64ANDL,
		ssa.OpAMD64ORQ, ssa.OpAMD64ORL,
		ssa.OpAMD64XORQ, ssa.OpAMD64XORL,
		ssa.OpAMD64SHLQ, ssa.OpAMD64SHLL,
		ssa.OpAMD64SHRQ, ssa.OpAMD64SHRL, ssa.OpAMD64SHRW, ssa.OpAMD64SHRB,
		ssa.OpAMD64SARQ, ssa.OpAMD64SARL, ssa.OpAMD64SARW, ssa.OpAMD64SARB,
		ssa.OpAMD64ROLQ, ssa.OpAMD64ROLL, ssa.OpAMD64ROLW, ssa.OpAMD64ROLB,
		ssa.OpAMD64RORQ, ssa.OpAMD64RORL, ssa.OpAMD64RORW, ssa.OpAMD64RORB,
		ssa.OpAMD64ADDSS, ssa.OpAMD64ADDSD, ssa.OpAMD64SUBSS, ssa.OpAMD64SUBSD,
		ssa.OpAMD64MULSS, ssa.OpAMD64MULSD, ssa.OpAMD64DIVSS, ssa.OpAMD64DIVSD,
		ssa.OpAMD64PXOR,
		ssa.OpAMD64BTSL, ssa.OpAMD64BTSQ,
		ssa.OpAMD64BTCL, ssa.OpAMD64BTCQ,
		ssa.OpAMD64BTRL, ssa.OpAMD64BTRQ:
		r := v.Reg()
		if r != v.Args[0].Reg() {
			v.Fatalf("input[0] and output not in same register %s", v.LongString())
		}
		opregreg(s, v.Op.Asm(), r, v.Args[1].Reg())

	case ssa.OpAMD64DIVQU, ssa.OpAMD64DIVLU, ssa.OpAMD64DIVWU:
		// Arg[0] (the dividend) is in AX.
		// Arg[1] (the divisor) can be in any other register.
		// Result[0] (the quotient) is in AX.
		// Result[1] (the remainder) is in DX.
		r := v.Args[1].Reg()

		// Zero extend dividend.
		c := s.Prog(x86.AXORL)
		c.From.Type = obj.TYPE_REG
		c.From.Reg = x86.REG_DX
		c.To.Type = obj.TYPE_REG
		c.To.Reg = x86.REG_DX

		// Issue divide.
		p := s.Prog(v.Op.Asm())
		p.From.Type = obj.TYPE_REG
		p.From.Reg = r

	case ssa.OpAMD64DIVQ, ssa.OpAMD64DIVL, ssa.OpAMD64DIVW:
		// Arg[0] (the dividend) is in AX.
		// Arg[1] (the divisor) can be in any other register.
		// Result[0] (the quotient) is in AX.
		// Result[1] (the remainder) is in DX.
		r := v.Args[1].Reg()

		// CPU faults upon signed overflow, which occurs when the most
		// negative int is divided by -1. Handle divide by -1 as a special case.
		var c *obj.Prog
		switch v.Op {
		case ssa.OpAMD64DIVQ:
			c = s.Prog(x86.ACMPQ)
		case ssa.OpAMD64DIVL:
			c = s.Prog(x86.ACMPL)
		case ssa.OpAMD64DIVW:
			c = s.Prog(x86.ACMPW)
		}
		c.From.Type = obj.TYPE_REG
		c.From.Reg = r
		c.To.Type = obj.TYPE_CONST
		c.To.Offset = -1
		j1 := s.Prog(x86.AJEQ)
		j1.To.Type = obj.TYPE_BRANCH

		// Sign extend dividend.
		switch v.Op {
		case ssa.OpAMD64DIVQ:
			s.Prog(x86.ACQO)
		case ssa.OpAMD64DIVL:
			s.Prog(x86.ACDQ)
		case ssa.OpAMD64DIVW:
			s.Prog(x86.ACWD)
		}

		// Issue divide.
		p := s.Prog(v.Op.Asm())
		p.From.Type = obj.TYPE_REG
		p.From.Reg = r

		// Skip over -1 fixup code.
		j2 := s.Prog(obj.AJMP)
		j2.To.Type = obj.TYPE_BRANCH

		// Issue -1 fixup code.
		// n / -1 = -n
		var n1 *obj.Prog
		switch v.Op {
		case ssa.OpAMD64DIVQ:
			n1 = s.Prog(x86.ANEGQ)
		case ssa.OpAMD64DIVL:
			n1 = s.Prog(x86.ANEGL)
		case ssa.OpAMD64DIVW:
			n1 = s.Prog(x86.ANEGW)
		}
		n1.To.Type = obj.TYPE_REG
		n1.To.Reg = x86.REG_AX

		// n % -1 == 0
		n2 := s.Prog(x86.AXORL)
		n2.From.Type = obj.TYPE_REG
		n2.From.Reg = x86.REG_DX
		n2.To.Type = obj.TYPE_REG
		n2.To.Reg = x86.REG_DX

		// TODO(khr): issue only the -1 fixup code we need.
		// For instance, if only the quotient is used, no point in zeroing the remainder.

		j1.To.Val = n1
		j2.To.Val = s.Pc()

	case ssa.OpAMD64HMULQ, ssa.OpAMD64HMULL, ssa.OpAMD64HMULQU, ssa.OpAMD64HMULLU:
		// the frontend rewrites constant division by 8/16/32 bit integers into
		// HMUL by a constant
		// SSA rewrites generate the 64 bit versions

		// Arg[0] is already in AX as it's the only register we allow
		// and DX is the only output we care about (the high bits)
		p := s.Prog(v.Op.Asm())
		p.From.Type = obj.TYPE_REG
		p.From.Reg = v.Args[1].Reg()

		// IMULB puts the high portion in AH instead of DL,
		// so move it to DL for consistency
		if v.Type.Size() == 1 {
			m := s.Prog(x86.AMOVB)
			m.From.Type = obj.TYPE_REG
			m.From.Reg = x86.REG_AH
			m.To.Type = obj.TYPE_REG
			m.To.Reg = x86.REG_DX
		}

	case ssa.OpAMD64MULQU2:
		// Arg[0] is already in AX as it's the only register we allow
		// results hi in DX, lo in AX
		p := s.Prog(v.Op.Asm())
		p.From.Type = obj.TYPE_REG
		p.From.Reg = v.Args[1].Reg()

	case ssa.OpAMD64DIVQU2:
		// Arg[0], Arg[1] are already in Dx, AX, as they're the only registers we allow
		// results q in AX, r in DX
		p := s.Prog(v.Op.Asm())
		p.From.Type = obj.TYPE_REG
		p.From.Reg = v.Args[2].Reg()

	case ssa.OpAMD64AVGQU:
		// compute (x+y)/2 unsigned.
		// Do a 64-bit add, the overflow goes into the carry.
		// Shift right once and pull the carry back into the 63rd bit.
		r := v.Reg()
		if r != v.Args[0].Reg() {
			v.Fatalf("input[0] and output not in same register %s", v.LongString())
		}
		p := s.Prog(x86.AADDQ)
		p.From.Type = obj.TYPE_REG
		p.To.Type = obj.TYPE_REG
		p.To.Reg = r
		p.From.Reg = v.Args[1].Reg()
		p = s.Prog(x86.ARCRQ)
		p.From.Type = obj.TYPE_CONST
		p.From.Offset = 1
		p.To.Type = obj.TYPE_REG
		p.To.Reg = r

	case ssa.OpAMD64ADDQconst, ssa.OpAMD64ADDLconst:
		r := v.Reg()
		a := v.Args[0].Reg()
		if r == a {
			if v.AuxInt == 1 {
				var asm obj.As
				// Software optimization manual recommends add $1,reg.
				// But inc/dec is 1 byte smaller. ICC always uses inc
				// Clang/GCC choose depending on flags, but prefer add.
				// Experiments show that inc/dec is both a little faster
				// and make a binary a little smaller.
				if v.Op == ssa.OpAMD64ADDQconst {
					asm = x86.AINCQ
				} else {
					asm = x86.AINCL
				}
				p := s.Prog(asm)
				p.To.Type = obj.TYPE_REG
				p.To.Reg = r
				return
			}
			if v.AuxInt == -1 {
				var asm obj.As
				if v.Op == ssa.OpAMD64ADDQconst {
					asm = x86.ADECQ
				} else {
					asm = x86.ADECL
				}
				p := s.Prog(asm)
				p.To.Type = obj.TYPE_REG
				p.To.Reg = r
				return
			}
			p := s.Prog(v.Op.Asm())
			p.From.Type = obj.TYPE_CONST
			p.From.Offset = v.AuxInt
			p.To.Type = obj.TYPE_REG
			p.To.Reg = r
			return
		}
		var asm obj.As
		if v.Op == ssa.OpAMD64ADDQconst {
			asm = x86.ALEAQ
		} else {
			asm = x86.ALEAL
		}
		p := s.Prog(asm)
		p.From.Type = obj.TYPE_MEM
		p.From.Reg = a
		p.From.Offset = v.AuxInt
		p.To.Type = obj.TYPE_REG
		p.To.Reg = r

	case ssa.OpAMD64CMOVQEQ, ssa.OpAMD64CMOVLEQ, ssa.OpAMD64CMOVWEQ,
		ssa.OpAMD64CMOVQLT, ssa.OpAMD64CMOVLLT, ssa.OpAMD64CMOVWLT,
		ssa.OpAMD64CMOVQNE, ssa.OpAMD64CMOVLNE, ssa.OpAMD64CMOVWNE,
		ssa.OpAMD64CMOVQGT, ssa.OpAMD64CMOVLGT, ssa.OpAMD64CMOVWGT,
		ssa.OpAMD64CMOVQLE, ssa.OpAMD64CMOVLLE, ssa.OpAMD64CMOVWLE,
		ssa.OpAMD64CMOVQGE, ssa.OpAMD64CMOVLGE, ssa.OpAMD64CMOVWGE,
		ssa.OpAMD64CMOVQHI, ssa.OpAMD64CMOVLHI, ssa.OpAMD64CMOVWHI,
		ssa.OpAMD64CMOVQLS, ssa.OpAMD64CMOVLLS, ssa.OpAMD64CMOVWLS,
		ssa.OpAMD64CMOVQCC, ssa.OpAMD64CMOVLCC, ssa.OpAMD64CMOVWCC,
		ssa.OpAMD64CMOVQCS, ssa.OpAMD64CMOVLCS, ssa.OpAMD64CMOVWCS,
		ssa.OpAMD64CMOVQGTF, ssa.OpAMD64CMOVLGTF, ssa.OpAMD64CMOVWGTF,
		ssa.OpAMD64CMOVQGEF, ssa.OpAMD64CMOVLGEF, ssa.OpAMD64CMOVWGEF:
		r := v.Reg()
		if r != v.Args[0].Reg() {
			v.Fatalf("input[0] and output not in same register %s", v.LongString())
		}
		p := s.Prog(v.Op.Asm())
		p.From.Type = obj.TYPE_REG
		p.From.Reg = v.Args[1].Reg()
		p.To.Type = obj.TYPE_REG
		p.To.Reg = r

	case ssa.OpAMD64CMOVQNEF, ssa.OpAMD64CMOVLNEF, ssa.OpAMD64CMOVWNEF:
		r := v.Reg()
		if r != v.Args[0].Reg() {
			v.Fatalf("input[0] and output not in same register %s", v.LongString())
		}
		// Flag condition: ^ZERO || PARITY
		// Generate:
		//   CMOV*NE  SRC,DST
		//   CMOV*PS  SRC,DST
		p := s.Prog(v.Op.Asm())
		p.From.Type = obj.TYPE_REG
		p.From.Reg = v.Args[1].Reg()
		p.To.Type = obj.TYPE_REG
		p.To.Reg = r
		var q *obj.Prog
		if v.Op == ssa.OpAMD64CMOVQNEF {
			q = s.Prog(x86.ACMOVQPS)
		} else if v.Op == ssa.OpAMD64CMOVLNEF {
			q = s.Prog(x86.ACMOVLPS)
		} else {
			q = s.Prog(x86.ACMOVWPS)
		}
		q.From.Type = obj.TYPE_REG
		q.From.Reg = v.Args[1].Reg()
		q.To.Type = obj.TYPE_REG
		q.To.Reg = r

	case ssa.OpAMD64CMOVQEQF, ssa.OpAMD64CMOVLEQF, ssa.OpAMD64CMOVWEQF:
		r := v.Reg()
		if r != v.Args[0].Reg() {
			v.Fatalf("input[0] and output not in same register %s", v.LongString())
		}

		// Flag condition: ZERO && !PARITY
		// Generate:
		//   MOV      SRC,AX
		//   CMOV*NE  DST,AX
		//   CMOV*PC  AX,DST
		//
		// TODO(rasky): we could generate:
		//   CMOV*NE  DST,SRC
		//   CMOV*PC  SRC,DST
		// But this requires a way for regalloc to know that SRC might be
		// clobbered by this instruction.
		if v.Args[1].Reg() != x86.REG_AX {
			opregreg(s, moveByType(v.Type), x86.REG_AX, v.Args[1].Reg())
		}
		p := s.Prog(v.Op.Asm())
		p.From.Type = obj.TYPE_REG
		p.From.Reg = r
		p.To.Type = obj.TYPE_REG
		p.To.Reg = x86.REG_AX
		var q *obj.Prog
		if v.Op == ssa.OpAMD64CMOVQEQF {
			q = s.Prog(x86.ACMOVQPC)
		} else if v.Op == ssa.OpAMD64CMOVLEQF {
			q = s.Prog(x86.ACMOVLPC)
		} else {
			q = s.Prog(x86.ACMOVWPC)
		}
		q.From.Type = obj.TYPE_REG
		q.From.Reg = x86.REG_AX
		q.To.Type = obj.TYPE_REG
		q.To.Reg = r

	case ssa.OpAMD64MULQconst, ssa.OpAMD64MULLconst:
		r := v.Reg()
		p := s.Prog(v.Op.Asm())
		p.From.Type = obj.TYPE_CONST
		p.From.Offset = v.AuxInt
		p.To.Type = obj.TYPE_REG
		p.To.Reg = r
		p.SetFrom3(obj.Addr{Type: obj.TYPE_REG, Reg: v.Args[0].Reg()})

	case ssa.OpAMD64SUBQconst, ssa.OpAMD64SUBLconst,
		ssa.OpAMD64ANDQconst, ssa.OpAMD64ANDLconst,
		ssa.OpAMD64ORQconst, ssa.OpAMD64ORLconst,
		ssa.OpAMD64XORQconst, ssa.OpAMD64XORLconst,
		ssa.OpAMD64SHLQconst, ssa.OpAMD64SHLLconst,
		ssa.OpAMD64SHRQconst, ssa.OpAMD64SHRLconst, ssa.OpAMD64SHRWconst, ssa.OpAMD64SHRBconst,
		ssa.OpAMD64SARQconst, ssa.OpAMD64SARLconst, ssa.OpAMD64SARWconst, ssa.OpAMD64SARBconst,
		ssa.OpAMD64ROLQconst, ssa.OpAMD64ROLLconst, ssa.OpAMD64ROLWconst, ssa.OpAMD64ROLBconst:
		r := v.Reg()
		if r != v.Args[0].Reg() {
			v.Fatalf("input[0] and output not in same register %s", v.LongString())
		}
		p := s.Prog(v.Op.Asm())
		p.From.Type = obj.TYPE_CONST
		p.From.Offset = v.AuxInt
		p.To.Type = obj.TYPE_REG
		p.To.Reg = r
	case ssa.OpAMD64SBBQcarrymask, ssa.OpAMD64SBBLcarrymask:
		r := v.Reg()
		p := s.Prog(v.Op.Asm())
		p.From.Type = obj.TYPE_REG
		p.From.Reg = r
		p.To.Type = obj.TYPE_REG
		p.To.Reg = r
	case ssa.OpAMD64LEAQ1, ssa.OpAMD64LEAQ2, ssa.OpAMD64LEAQ4, ssa.OpAMD64LEAQ8,
		ssa.OpAMD64LEAL1, ssa.OpAMD64LEAL2, ssa.OpAMD64LEAL4, ssa.OpAMD64LEAL8,
		ssa.OpAMD64LEAW1, ssa.OpAMD64LEAW2, ssa.OpAMD64LEAW4, ssa.OpAMD64LEAW8:
		r := v.Args[0].Reg()
		i := v.Args[1].Reg()
		p := s.Prog(v.Op.Asm())
		switch v.Op {
		case ssa.OpAMD64LEAQ1, ssa.OpAMD64LEAL1, ssa.OpAMD64LEAW1:
			p.From.Scale = 1
			if i == x86.REG_SP {
				r, i = i, r
			}
		case ssa.OpAMD64LEAQ2, ssa.OpAMD64LEAL2, ssa.OpAMD64LEAW2:
			p.From.Scale = 2
		case ssa.OpAMD64LEAQ4, ssa.OpAMD64LEAL4, ssa.OpAMD64LEAW4:
			p.From.Scale = 4
		case ssa.OpAMD64LEAQ8, ssa.OpAMD64LEAL8, ssa.OpAMD64LEAW8:
			p.From.Scale = 8
		}
		p.From.Type = obj.TYPE_MEM
		p.From.Reg = r
		p.From.Index = i
		gc.AddAux(&p.From, v)
		p.To.Type = obj.TYPE_REG
		p.To.Reg = v.Reg()
	case ssa.OpAMD64LEAQ, ssa.OpAMD64LEAL, ssa.OpAMD64LEAW:
		p := s.Prog(v.Op.Asm())
		p.From.Type = obj.TYPE_MEM
		p.From.Reg = v.Args[0].Reg()
		gc.AddAux(&p.From, v)
		p.To.Type = obj.TYPE_REG
		p.To.Reg = v.Reg()
	case ssa.OpAMD64CMPQ, ssa.OpAMD64CMPL, ssa.OpAMD64CMPW, ssa.OpAMD64CMPB,
		ssa.OpAMD64TESTQ, ssa.OpAMD64TESTL, ssa.OpAMD64TESTW, ssa.OpAMD64TESTB,
		ssa.OpAMD64BTL, ssa.OpAMD64BTQ:
		opregreg(s, v.Op.Asm(), v.Args[1].Reg(), v.Args[0].Reg())
	case ssa.OpAMD64UCOMISS, ssa.OpAMD64UCOMISD:
		// Go assembler has swapped operands for UCOMISx relative to CMP,
		// must account for that right here.
		opregreg(s, v.Op.Asm(), v.Args[0].Reg(), v.Args[1].Reg())
	case ssa.OpAMD64CMPQconst, ssa.OpAMD64CMPLconst, ssa.OpAMD64CMPWconst, ssa.OpAMD64CMPBconst:
		p := s.Prog(v.Op.Asm())
		p.From.Type = obj.TYPE_REG
		p.From.Reg = v.Args[0].Reg()
		p.To.Type = obj.TYPE_CONST
		p.To.Offset = v.AuxInt
	case ssa.OpAMD64BTLconst, ssa.OpAMD64BTQconst:
		op := v.Op
		if op == ssa.OpAMD64BTQconst && v.AuxInt < 32 {
			// Emit 32-bit version because it's shorter
			op = ssa.OpAMD64BTLconst
		}
		p := s.Prog(op.Asm())
		p.From.Type = obj.TYPE_CONST
		p.From.Offset = v.AuxInt
		p.To.Type = obj.TYPE_REG
		p.To.Reg = v.Args[0].Reg()
	case ssa.OpAMD64TESTQconst, ssa.OpAMD64TESTLconst, ssa.OpAMD64TESTWconst, ssa.OpAMD64TESTBconst,
		ssa.OpAMD64BTSLconst, ssa.OpAMD64BTSQconst,
		ssa.OpAMD64BTCLconst, ssa.OpAMD64BTCQconst,
		ssa.OpAMD64BTRLconst, ssa.OpAMD64BTRQconst:
		p := s.Prog(v.Op.Asm())
		p.From.Type = obj.TYPE_CONST
		p.From.Offset = v.AuxInt
		p.To.Type = obj.TYPE_REG
		p.To.Reg = v.Args[0].Reg()
	case ssa.OpAMD64CMPQmem, ssa.OpAMD64CMPLmem, ssa.OpAMD64CMPWmem, ssa.OpAMD64CMPBmem:
		p := s.Prog(v.Op.Asm())
		p.From.Type = obj.TYPE_MEM
		p.From.Reg = v.Args[0].Reg()
		gc.AddAux(&p.From, v)
		p.To.Type = obj.TYPE_REG
		p.To.Reg = v.Args[1].Reg()
	case ssa.OpAMD64CMPQconstmem, ssa.OpAMD64CMPLconstmem, ssa.OpAMD64CMPWconstmem, ssa.OpAMD64CMPBconstmem:
		sc := v.AuxValAndOff()
		p := s.Prog(v.Op.Asm())
		p.From.Type = obj.TYPE_MEM
		p.From.Reg = v.Args[0].Reg()
		gc.AddAux2(&p.From, v, sc.Off())
		p.To.Type = obj.TYPE_CONST
		p.To.Offset = sc.Val()
	case ssa.OpAMD64MOVLconst, ssa.OpAMD64MOVQconst:
		x := v.Reg()

		// If flags aren't live (indicated by v.Aux == nil),
		// then we can rewrite MOV $0, AX into XOR AX, AX.
		if v.AuxInt == 0 && v.Aux == nil {
			p := s.Prog(x86.AXORL)
			p.From.Type = obj.TYPE_REG
			p.From.Reg = x
			p.To.Type = obj.TYPE_REG
			p.To.Reg = x
			break
		}

		asm := v.Op.Asm()
		// Use MOVL to move a small constant into a register
		// when the constant is positive and fits into 32 bits.
		if 0 <= v.AuxInt && v.AuxInt <= (1<<32-1) {
			// The upper 32bit are zeroed automatically when using MOVL.
			asm = x86.AMOVL
		}
		p := s.Prog(asm)
		p.From.Type = obj.TYPE_CONST
		p.From.Offset = v.AuxInt
		p.To.Type = obj.TYPE_REG
		p.To.Reg = x
	case ssa.OpAMD64MOVSSconst, ssa.OpAMD64MOVSDconst:
		x := v.Reg()
		p := s.Prog(v.Op.Asm())
		p.From.Type = obj.TYPE_FCONST
		p.From.Val = math.Float64frombits(uint64(v.AuxInt))
		p.To.Type = obj.TYPE_REG
		p.To.Reg = x
	case ssa.OpAMD64MOVQload, ssa.OpAMD64MOVSSload, ssa.OpAMD64MOVSDload, ssa.OpAMD64MOVLload, ssa.OpAMD64MOVWload, ssa.OpAMD64MOVBload, ssa.OpAMD64MOVBQSXload, ssa.OpAMD64MOVWQSXload, ssa.OpAMD64MOVLQSXload, ssa.OpAMD64MOVOload:
		p := s.Prog(v.Op.Asm())
		p.From.Type = obj.TYPE_MEM
		p.From.Reg = v.Args[0].Reg()
		gc.AddAux(&p.From, v)
		p.To.Type = obj.TYPE_REG
		p.To.Reg = v.Reg()
	case ssa.OpAMD64MOVQloadidx8, ssa.OpAMD64MOVSDloadidx8, ssa.OpAMD64MOVLloadidx8:
		p := s.Prog(v.Op.Asm())
		p.From.Type = obj.TYPE_MEM
		p.From.Reg = v.Args[0].Reg()
		gc.AddAux(&p.From, v)
		p.From.Scale = 8
		p.From.Index = v.Args[1].Reg()
		p.To.Type = obj.TYPE_REG
		p.To.Reg = v.Reg()
	case ssa.OpAMD64MOVLloadidx4, ssa.OpAMD64MOVSSloadidx4:
		p := s.Prog(v.Op.Asm())
		p.From.Type = obj.TYPE_MEM
		p.From.Reg = v.Args[0].Reg()
		gc.AddAux(&p.From, v)
		p.From.Scale = 4
		p.From.Index = v.Args[1].Reg()
		p.To.Type = obj.TYPE_REG
		p.To.Reg = v.Reg()
	case ssa.OpAMD64MOVWloadidx2:
		p := s.Prog(v.Op.Asm())
		p.From.Type = obj.TYPE_MEM
		p.From.Reg = v.Args[0].Reg()
		gc.AddAux(&p.From, v)
		p.From.Scale = 2
		p.From.Index = v.Args[1].Reg()
		p.To.Type = obj.TYPE_REG
		p.To.Reg = v.Reg()
	case ssa.OpAMD64MOVBloadidx1, ssa.OpAMD64MOVWloadidx1, ssa.OpAMD64MOVLloadidx1, ssa.OpAMD64MOVQloadidx1, ssa.OpAMD64MOVSSloadidx1, ssa.OpAMD64MOVSDloadidx1:
		r := v.Args[0].Reg()
		i := v.Args[1].Reg()
		if i == x86.REG_SP {
			r, i = i, r
		}
		p := s.Prog(v.Op.Asm())
		p.From.Type = obj.TYPE_MEM
		p.From.Reg = r
		p.From.Scale = 1
		p.From.Index = i
		gc.AddAux(&p.From, v)
		p.To.Type = obj.TYPE_REG
		p.To.Reg = v.Reg()
	case ssa.OpAMD64MOVQstore, ssa.OpAMD64MOVSSstore, ssa.OpAMD64MOVSDstore, ssa.OpAMD64MOVLstore, ssa.OpAMD64MOVWstore, ssa.OpAMD64MOVBstore, ssa.OpAMD64MOVOstore:
		p := s.Prog(v.Op.Asm())
		p.From.Type = obj.TYPE_REG
		p.From.Reg = v.Args[1].Reg()
		p.To.Type = obj.TYPE_MEM
		p.To.Reg = v.Args[0].Reg()
		gc.AddAux(&p.To, v)
	case ssa.OpAMD64MOVQstoreidx8, ssa.OpAMD64MOVSDstoreidx8, ssa.OpAMD64MOVLstoreidx8:
		p := s.Prog(v.Op.Asm())
		p.From.Type = obj.TYPE_REG
		p.From.Reg = v.Args[2].Reg()
		p.To.Type = obj.TYPE_MEM
		p.To.Reg = v.Args[0].Reg()
		p.To.Scale = 8
		p.To.Index = v.Args[1].Reg()
		gc.AddAux(&p.To, v)
	case ssa.OpAMD64MOVSSstoreidx4, ssa.OpAMD64MOVLstoreidx4:
		p := s.Prog(v.Op.Asm())
		p.From.Type = obj.TYPE_REG
		p.From.Reg = v.Args[2].Reg()
		p.To.Type = obj.TYPE_MEM
		p.To.Reg = v.Args[0].Reg()
		p.To.Scale = 4
		p.To.Index = v.Args[1].Reg()
		gc.AddAux(&p.To, v)
	case ssa.OpAMD64MOVWstoreidx2:
		p := s.Prog(v.Op.Asm())
		p.From.Type = obj.TYPE_REG
		p.From.Reg = v.Args[2].Reg()
		p.To.Type = obj.TYPE_MEM
		p.To.Reg = v.Args[0].Reg()
		p.To.Scale = 2
		p.To.Index = v.Args[1].Reg()
		gc.AddAux(&p.To, v)
	case ssa.OpAMD64MOVBstoreidx1, ssa.OpAMD64MOVWstoreidx1, ssa.OpAMD64MOVLstoreidx1, ssa.OpAMD64MOVQstoreidx1, ssa.OpAMD64MOVSSstoreidx1, ssa.OpAMD64MOVSDstoreidx1:
		r := v.Args[0].Reg()
		i := v.Args[1].Reg()
		if i == x86.REG_SP {
			r, i = i, r
		}
		p := s.Prog(v.Op.Asm())
		p.From.Type = obj.TYPE_REG
		p.From.Reg = v.Args[2].Reg()
		p.To.Type = obj.TYPE_MEM
		p.To.Reg = r
		p.To.Scale = 1
		p.To.Index = i
		gc.AddAux(&p.To, v)
	case ssa.OpAMD64ADDQconstmem, ssa.OpAMD64ADDLconstmem:
		sc := v.AuxValAndOff()
		off := sc.Off()
		val := sc.Val()
		if val == 1 {
			var asm obj.As
			if v.Op == ssa.OpAMD64ADDQconstmem {
				asm = x86.AINCQ
			} else {
				asm = x86.AINCL
			}
			p := s.Prog(asm)
			p.To.Type = obj.TYPE_MEM
			p.To.Reg = v.Args[0].Reg()
			gc.AddAux2(&p.To, v, off)
		} else {
			p := s.Prog(v.Op.Asm())
			p.From.Type = obj.TYPE_CONST
			p.From.Offset = val
			p.To.Type = obj.TYPE_MEM
			p.To.Reg = v.Args[0].Reg()
			gc.AddAux2(&p.To, v, off)
		}
	case ssa.OpAMD64MOVQstoreconst, ssa.OpAMD64MOVLstoreconst, ssa.OpAMD64MOVWstoreconst, ssa.OpAMD64MOVBstoreconst:
		p := s.Prog(v.Op.Asm())
		p.From.Type = obj.TYPE_CONST
		sc := v.AuxValAndOff()
		p.From.Offset = sc.Val()
		p.To.Type = obj.TYPE_MEM
		p.To.Reg = v.Args[0].Reg()
		gc.AddAux2(&p.To, v, sc.Off())
	case ssa.OpAMD64MOVQstoreconstidx1, ssa.OpAMD64MOVQstoreconstidx8, ssa.OpAMD64MOVLstoreconstidx1, ssa.OpAMD64MOVLstoreconstidx4, ssa.OpAMD64MOVWstoreconstidx1, ssa.OpAMD64MOVWstoreconstidx2, ssa.OpAMD64MOVBstoreconstidx1:
		p := s.Prog(v.Op.Asm())
		p.From.Type = obj.TYPE_CONST
		sc := v.AuxValAndOff()
		p.From.Offset = sc.Val()
		r := v.Args[0].Reg()
		i := v.Args[1].Reg()
		switch v.Op {
		case ssa.OpAMD64MOVBstoreconstidx1, ssa.OpAMD64MOVWstoreconstidx1, ssa.OpAMD64MOVLstoreconstidx1, ssa.OpAMD64MOVQstoreconstidx1:
			p.To.Scale = 1
			if i == x86.REG_SP {
				r, i = i, r
			}
		case ssa.OpAMD64MOVWstoreconstidx2:
			p.To.Scale = 2
		case ssa.OpAMD64MOVLstoreconstidx4:
			p.To.Scale = 4
		case ssa.OpAMD64MOVQstoreconstidx8:
			p.To.Scale = 8
		}
		p.To.Type = obj.TYPE_MEM
		p.To.Reg = r
		p.To.Index = i
		gc.AddAux2(&p.To, v, sc.Off())
	case ssa.OpAMD64MOVLQSX, ssa.OpAMD64MOVWQSX, ssa.OpAMD64MOVBQSX, ssa.OpAMD64MOVLQZX, ssa.OpAMD64MOVWQZX, ssa.OpAMD64MOVBQZX,
		ssa.OpAMD64CVTTSS2SL, ssa.OpAMD64CVTTSD2SL, ssa.OpAMD64CVTTSS2SQ, ssa.OpAMD64CVTTSD2SQ,
		ssa.OpAMD64CVTSS2SD, ssa.OpAMD64CVTSD2SS:
		opregreg(s, v.Op.Asm(), v.Reg(), v.Args[0].Reg())
	case ssa.OpAMD64CVTSL2SD, ssa.OpAMD64CVTSQ2SD, ssa.OpAMD64CVTSQ2SS, ssa.OpAMD64CVTSL2SS:
		r := v.Reg()
		// Break false dependency on destination register.
		opregreg(s, x86.AXORPS, r, r)
		opregreg(s, v.Op.Asm(), r, v.Args[0].Reg())
	case ssa.OpAMD64MOVQi2f, ssa.OpAMD64MOVQf2i:
		p := s.Prog(x86.AMOVQ)
		p.From.Type = obj.TYPE_REG
		p.From.Reg = v.Args[0].Reg()
		p.To.Type = obj.TYPE_REG
		p.To.Reg = v.Reg()
	case ssa.OpAMD64MOVLi2f, ssa.OpAMD64MOVLf2i:
		p := s.Prog(x86.AMOVL)
		p.From.Type = obj.TYPE_REG
		p.From.Reg = v.Args[0].Reg()
		p.To.Type = obj.TYPE_REG
		p.To.Reg = v.Reg()
	case ssa.OpAMD64ADDQmem, ssa.OpAMD64ADDLmem, ssa.OpAMD64SUBQmem, ssa.OpAMD64SUBLmem,
		ssa.OpAMD64ANDQmem, ssa.OpAMD64ANDLmem, ssa.OpAMD64ORQmem, ssa.OpAMD64ORLmem,
		ssa.OpAMD64XORQmem, ssa.OpAMD64XORLmem, ssa.OpAMD64ADDSDmem, ssa.OpAMD64ADDSSmem,
		ssa.OpAMD64SUBSDmem, ssa.OpAMD64SUBSSmem, ssa.OpAMD64MULSDmem, ssa.OpAMD64MULSSmem:
		p := s.Prog(v.Op.Asm())
		p.From.Type = obj.TYPE_MEM
		p.From.Reg = v.Args[1].Reg()
		gc.AddAux(&p.From, v)
		p.To.Type = obj.TYPE_REG
		p.To.Reg = v.Reg()
		if v.Reg() != v.Args[0].Reg() {
			v.Fatalf("input[0] and output not in same register %s", v.LongString())
		}
	case ssa.OpAMD64DUFFZERO:
		off := duffStart(v.AuxInt)
		adj := duffAdj(v.AuxInt)
		var p *obj.Prog
		if adj != 0 {
			p = s.Prog(x86.ALEAQ)
			p.From.Type = obj.TYPE_MEM
			p.From.Offset = adj
			p.From.Reg = x86.REG_DI
			p.To.Type = obj.TYPE_REG
			p.To.Reg = x86.REG_DI
		}
		p = s.Prog(obj.ADUFFZERO)
		p.To.Type = obj.TYPE_ADDR
		p.To.Sym = gc.Duffzero
		p.To.Offset = off
	case ssa.OpAMD64MOVOconst:
		if v.AuxInt != 0 {
			v.Fatalf("MOVOconst can only do constant=0")
		}
		r := v.Reg()
		opregreg(s, x86.AXORPS, r, r)
	case ssa.OpAMD64DUFFCOPY:
		p := s.Prog(obj.ADUFFCOPY)
		p.To.Type = obj.TYPE_ADDR
		p.To.Sym = gc.Duffcopy
		p.To.Offset = v.AuxInt

	case ssa.OpCopy: // TODO: use MOVQreg for reg->reg copies instead of OpCopy?
		if v.Type.IsMemory() {
			return
		}
		x := v.Args[0].Reg()
		y := v.Reg()
		if x != y {
			opregreg(s, moveByType(v.Type), y, x)
		}
	case ssa.OpLoadReg:
		if v.Type.IsFlags() {
			v.Fatalf("load flags not implemented: %v", v.LongString())
			return
		}
		p := s.Prog(loadByType(v.Type))
		gc.AddrAuto(&p.From, v.Args[0])
		p.To.Type = obj.TYPE_REG
		p.To.Reg = v.Reg()

	case ssa.OpStoreReg:
		if v.Type.IsFlags() {
			v.Fatalf("store flags not implemented: %v", v.LongString())
			return
		}
		p := s.Prog(storeByType(v.Type))
		p.From.Type = obj.TYPE_REG
		p.From.Reg = v.Args[0].Reg()
		gc.AddrAuto(&p.To, v)
	case ssa.OpAMD64LoweredGetClosurePtr:
		// Closure pointer is DX.
		gc.CheckLoweredGetClosurePtr(v)
	case ssa.OpAMD64LoweredGetG:
		r := v.Reg()
		// See the comments in cmd/internal/obj/x86/obj6.go
		// near CanUse1InsnTLS for a detailed explanation of these instructions.
		if x86.CanUse1InsnTLS(gc.Ctxt) {
			// MOVQ (TLS), r
			p := s.Prog(x86.AMOVQ)
			p.From.Type = obj.TYPE_MEM
			p.From.Reg = x86.REG_TLS
			p.To.Type = obj.TYPE_REG
			p.To.Reg = r
		} else {
			// MOVQ TLS, r
			// MOVQ (r)(TLS*1), r
			p := s.Prog(x86.AMOVQ)
			p.From.Type = obj.TYPE_REG
			p.From.Reg = x86.REG_TLS
			p.To.Type = obj.TYPE_REG
			p.To.Reg = r
			q := s.Prog(x86.AMOVQ)
			q.From.Type = obj.TYPE_MEM
			q.From.Reg = r
			q.From.Index = x86.REG_TLS
			q.From.Scale = 1
			q.To.Type = obj.TYPE_REG
			q.To.Reg = r
		}
	case ssa.OpAMD64CALLstatic, ssa.OpAMD64CALLclosure, ssa.OpAMD64CALLinter:
		s.Call(v)

	case ssa.OpAMD64LoweredGetCallerPC:
		p := s.Prog(x86.AMOVQ)
		p.From.Type = obj.TYPE_MEM
		p.From.Offset = -8 // PC is stored 8 bytes below first parameter.
		p.From.Name = obj.NAME_PARAM
		p.To.Type = obj.TYPE_REG
		p.To.Reg = v.Reg()

	case ssa.OpAMD64LoweredGetCallerSP:
		// caller's SP is the address of the first arg
		mov := x86.AMOVQ
		if gc.Widthptr == 4 {
			mov = x86.AMOVL
		}
		p := s.Prog(mov)
		p.From.Type = obj.TYPE_ADDR
		p.From.Offset = -gc.Ctxt.FixedFrameSize() // 0 on amd64, just to be consistent with other architectures
		p.From.Name = obj.NAME_PARAM
		p.To.Type = obj.TYPE_REG
		p.To.Reg = v.Reg()

	case ssa.OpAMD64LoweredWB:
		p := s.Prog(obj.ACALL)
		p.To.Type = obj.TYPE_MEM
		p.To.Name = obj.NAME_EXTERN
		p.To.Sym = v.Aux.(*obj.LSym)

	case ssa.OpAMD64NEGQ, ssa.OpAMD64NEGL,
		ssa.OpAMD64BSWAPQ, ssa.OpAMD64BSWAPL,
		ssa.OpAMD64NOTQ, ssa.OpAMD64NOTL:
		r := v.Reg()
		if r != v.Args[0].Reg() {
			v.Fatalf("input[0] and output not in same register %s", v.LongString())
		}
		p := s.Prog(v.Op.Asm())
		p.To.Type = obj.TYPE_REG
		p.To.Reg = r
	case ssa.OpAMD64BSFQ, ssa.OpAMD64BSRQ:
		p := s.Prog(v.Op.Asm())
		p.From.Type = obj.TYPE_REG
		p.From.Reg = v.Args[0].Reg()
		p.To.Type = obj.TYPE_REG
		p.To.Reg = v.Reg0()
	case ssa.OpAMD64BSFL, ssa.OpAMD64BSRL:
		p := s.Prog(v.Op.Asm())
		p.From.Type = obj.TYPE_REG
		p.From.Reg = v.Args[0].Reg()
		p.To.Type = obj.TYPE_REG
		p.To.Reg = v.Reg()
	case ssa.OpAMD64SQRTSD:
		p := s.Prog(v.Op.Asm())
		p.From.Type = obj.TYPE_REG
		p.From.Reg = v.Args[0].Reg()
		p.To.Type = obj.TYPE_REG
		p.To.Reg = v.Reg()
	case ssa.OpAMD64ROUNDSD:
		p := s.Prog(v.Op.Asm())
		val := v.AuxInt
		// 0 means math.RoundToEven, 1 Floor, 2 Ceil, 3 Trunc
		if val != 0 && val != 1 && val != 2 && val != 3 {
			v.Fatalf("Invalid rounding mode")
		}
		p.From.Offset = val
		p.From.Type = obj.TYPE_CONST
		p.SetFrom3(obj.Addr{Type: obj.TYPE_REG, Reg: v.Args[0].Reg()})
		p.To.Type = obj.TYPE_REG
		p.To.Reg = v.Reg()
	case ssa.OpAMD64POPCNTQ, ssa.OpAMD64POPCNTL:
		if v.Args[0].Reg() != v.Reg() {
			// POPCNT on Intel has a false dependency on the destination register.
			// Xor register with itself to break the dependency.
			p := s.Prog(x86.AXORQ)
			p.From.Type = obj.TYPE_REG
			p.From.Reg = v.Reg()
			p.To.Type = obj.TYPE_REG
			p.To.Reg = v.Reg()
		}
		p := s.Prog(v.Op.Asm())
		p.From.Type = obj.TYPE_REG
		p.From.Reg = v.Args[0].Reg()
		p.To.Type = obj.TYPE_REG
		p.To.Reg = v.Reg()

	case ssa.OpAMD64SETEQ, ssa.OpAMD64SETNE,
		ssa.OpAMD64SETL, ssa.OpAMD64SETLE,
		ssa.OpAMD64SETG, ssa.OpAMD64SETGE,
		ssa.OpAMD64SETGF, ssa.OpAMD64SETGEF,
		ssa.OpAMD64SETB, ssa.OpAMD64SETBE,
		ssa.OpAMD64SETORD, ssa.OpAMD64SETNAN,
		ssa.OpAMD64SETA, ssa.OpAMD64SETAE:
		p := s.Prog(v.Op.Asm())
		p.To.Type = obj.TYPE_REG
		p.To.Reg = v.Reg()

	case ssa.OpAMD64SETEQmem, ssa.OpAMD64SETNEmem,
		ssa.OpAMD64SETLmem, ssa.OpAMD64SETLEmem,
		ssa.OpAMD64SETGmem, ssa.OpAMD64SETGEmem,
		ssa.OpAMD64SETBmem, ssa.OpAMD64SETBEmem,
		ssa.OpAMD64SETAmem, ssa.OpAMD64SETAEmem:
		p := s.Prog(v.Op.Asm())
		p.To.Type = obj.TYPE_MEM
		p.To.Reg = v.Args[0].Reg()
		gc.AddAux(&p.To, v)

	case ssa.OpAMD64SETNEF:
		p := s.Prog(v.Op.Asm())
		p.To.Type = obj.TYPE_REG
		p.To.Reg = v.Reg()
		q := s.Prog(x86.ASETPS)
		q.To.Type = obj.TYPE_REG
		q.To.Reg = x86.REG_AX
		// ORL avoids partial register write and is smaller than ORQ, used by old compiler
		opregreg(s, x86.AORL, v.Reg(), x86.REG_AX)

	case ssa.OpAMD64SETEQF:
		p := s.Prog(v.Op.Asm())
		p.To.Type = obj.TYPE_REG
		p.To.Reg = v.Reg()
		q := s.Prog(x86.ASETPC)
		q.To.Type = obj.TYPE_REG
		q.To.Reg = x86.REG_AX
		// ANDL avoids partial register write and is smaller than ANDQ, used by old compiler
		opregreg(s, x86.AANDL, v.Reg(), x86.REG_AX)

	case ssa.OpAMD64InvertFlags:
		v.Fatalf("InvertFlags should never make it to codegen %v", v.LongString())
	case ssa.OpAMD64FlagEQ, ssa.OpAMD64FlagLT_ULT, ssa.OpAMD64FlagLT_UGT, ssa.OpAMD64FlagGT_ULT, ssa.OpAMD64FlagGT_UGT:
		v.Fatalf("Flag* ops should never make it to codegen %v", v.LongString())
	case ssa.OpAMD64AddTupleFirst32, ssa.OpAMD64AddTupleFirst64:
		v.Fatalf("AddTupleFirst* should never make it to codegen %v", v.LongString())
	case ssa.OpAMD64REPSTOSQ:
		s.Prog(x86.AREP)
		s.Prog(x86.ASTOSQ)
	case ssa.OpAMD64REPMOVSQ:
		s.Prog(x86.AREP)
		s.Prog(x86.AMOVSQ)
	case ssa.OpAMD64LoweredNilCheck:
		// Issue a load which will fault if the input is nil.
		// TODO: We currently use the 2-byte instruction TESTB AX, (reg).
		// Should we use the 3-byte TESTB $0, (reg) instead? It is larger
		// but it doesn't have false dependency on AX.
		// Or maybe allocate an output register and use MOVL (reg),reg2 ?
		// That trades clobbering flags for clobbering a register.
		p := s.Prog(x86.ATESTB)
		p.From.Type = obj.TYPE_REG
		p.From.Reg = x86.REG_AX
		p.To.Type = obj.TYPE_MEM
		p.To.Reg = v.Args[0].Reg()
		gc.AddAux(&p.To, v)
		if gc.Debug_checknil != 0 && v.Pos.Line() > 1 { // v.Pos.Line()==1 in generated wrappers
			gc.Warnl(v.Pos, "generated nil check")
		}
	case ssa.OpAMD64MOVLatomicload, ssa.OpAMD64MOVQatomicload:
		p := s.Prog(v.Op.Asm())
		p.From.Type = obj.TYPE_MEM
		p.From.Reg = v.Args[0].Reg()
		gc.AddAux(&p.From, v)
		p.To.Type = obj.TYPE_REG
		p.To.Reg = v.Reg0()
	case ssa.OpAMD64XCHGL, ssa.OpAMD64XCHGQ:
		r := v.Reg0()
		if r != v.Args[0].Reg() {
			v.Fatalf("input[0] and output[0] not in same register %s", v.LongString())
		}
		p := s.Prog(v.Op.Asm())
		p.From.Type = obj.TYPE_REG
		p.From.Reg = r
		p.To.Type = obj.TYPE_MEM
		p.To.Reg = v.Args[1].Reg()
		gc.AddAux(&p.To, v)
	case ssa.OpAMD64XADDLlock, ssa.OpAMD64XADDQlock:
		r := v.Reg0()
		if r != v.Args[0].Reg() {
			v.Fatalf("input[0] and output[0] not in same register %s", v.LongString())
		}
		s.Prog(x86.ALOCK)
		p := s.Prog(v.Op.Asm())
		p.From.Type = obj.TYPE_REG
		p.From.Reg = r
		p.To.Type = obj.TYPE_MEM
		p.To.Reg = v.Args[1].Reg()
		gc.AddAux(&p.To, v)
	case ssa.OpAMD64CMPXCHGLlock, ssa.OpAMD64CMPXCHGQlock:
		if v.Args[1].Reg() != x86.REG_AX {
			v.Fatalf("input[1] not in AX %s", v.LongString())
		}
		s.Prog(x86.ALOCK)
		p := s.Prog(v.Op.Asm())
		p.From.Type = obj.TYPE_REG
		p.From.Reg = v.Args[2].Reg()
		p.To.Type = obj.TYPE_MEM
		p.To.Reg = v.Args[0].Reg()
		gc.AddAux(&p.To, v)
		p = s.Prog(x86.ASETEQ)
		p.To.Type = obj.TYPE_REG
		p.To.Reg = v.Reg0()
	case ssa.OpAMD64ANDBlock, ssa.OpAMD64ORBlock:
		s.Prog(x86.ALOCK)
		p := s.Prog(v.Op.Asm())
		p.From.Type = obj.TYPE_REG
		p.From.Reg = v.Args[1].Reg()
		p.To.Type = obj.TYPE_MEM
		p.To.Reg = v.Args[0].Reg()
		gc.AddAux(&p.To, v)
	case ssa.OpClobber:
		p := s.Prog(x86.AMOVL)
		p.From.Type = obj.TYPE_CONST
		p.From.Offset = 0xdeaddead
		p.To.Type = obj.TYPE_MEM
		p.To.Reg = x86.REG_SP
		gc.AddAux(&p.To, v)
		p = s.Prog(x86.AMOVL)
		p.From.Type = obj.TYPE_CONST
		p.From.Offset = 0xdeaddead
		p.To.Type = obj.TYPE_MEM
		p.To.Reg = x86.REG_SP
		gc.AddAux(&p.To, v)
		p.To.Offset += 4
	default:
		v.Fatalf("genValue not implemented: %s", v.LongString())
	}
}

var blockJump = [...]struct {
	asm, invasm obj.As
}{
	ssa.BlockAMD64EQ:  {x86.AJEQ, x86.AJNE},
	ssa.BlockAMD64NE:  {x86.AJNE, x86.AJEQ},
	ssa.BlockAMD64LT:  {x86.AJLT, x86.AJGE},
	ssa.BlockAMD64GE:  {x86.AJGE, x86.AJLT},
	ssa.BlockAMD64LE:  {x86.AJLE, x86.AJGT},
	ssa.BlockAMD64GT:  {x86.AJGT, x86.AJLE},
	ssa.BlockAMD64ULT: {x86.AJCS, x86.AJCC},
	ssa.BlockAMD64UGE: {x86.AJCC, x86.AJCS},
	ssa.BlockAMD64UGT: {x86.AJHI, x86.AJLS},
	ssa.BlockAMD64ULE: {x86.AJLS, x86.AJHI},
	ssa.BlockAMD64ORD: {x86.AJPC, x86.AJPS},
	ssa.BlockAMD64NAN: {x86.AJPS, x86.AJPC},
}

var eqfJumps = [2][2]gc.FloatingEQNEJump{
	{{Jump: x86.AJNE, Index: 1}, {Jump: x86.AJPS, Index: 1}}, // next == b.Succs[0]
	{{Jump: x86.AJNE, Index: 1}, {Jump: x86.AJPC, Index: 0}}, // next == b.Succs[1]
}
var nefJumps = [2][2]gc.FloatingEQNEJump{
	{{Jump: x86.AJNE, Index: 0}, {Jump: x86.AJPC, Index: 1}}, // next == b.Succs[0]
	{{Jump: x86.AJNE, Index: 0}, {Jump: x86.AJPS, Index: 0}}, // next == b.Succs[1]
}

func ssaGenBlock(s *gc.SSAGenState, b, next *ssa.Block) {
	switch b.Kind {
	case ssa.BlockPlain:
		if b.Succs[0].Block() != next {
			p := s.Prog(obj.AJMP)
			p.To.Type = obj.TYPE_BRANCH
			s.Branches = append(s.Branches, gc.Branch{P: p, B: b.Succs[0].Block()})
		}
	case ssa.BlockDefer:
		// defer returns in rax:
		// 0 if we should continue executing
		// 1 if we should jump to deferreturn call
		p := s.Prog(x86.ATESTL)
		p.From.Type = obj.TYPE_REG
		p.From.Reg = x86.REG_AX
		p.To.Type = obj.TYPE_REG
		p.To.Reg = x86.REG_AX
		p = s.Prog(x86.AJNE)
		p.To.Type = obj.TYPE_BRANCH
		s.Branches = append(s.Branches, gc.Branch{P: p, B: b.Succs[1].Block()})
		if b.Succs[0].Block() != next {
			p := s.Prog(obj.AJMP)
			p.To.Type = obj.TYPE_BRANCH
			s.Branches = append(s.Branches, gc.Branch{P: p, B: b.Succs[0].Block()})
		}
	case ssa.BlockExit:
		s.Prog(obj.AUNDEF) // tell plive.go that we never reach here
	case ssa.BlockRet:
		s.Prog(obj.ARET)
	case ssa.BlockRetJmp:
		p := s.Prog(obj.ARET)
		p.To.Type = obj.TYPE_MEM
		p.To.Name = obj.NAME_EXTERN
		p.To.Sym = b.Aux.(*obj.LSym)

	case ssa.BlockAMD64EQF:
		s.FPJump(b, next, &eqfJumps)

	case ssa.BlockAMD64NEF:
		s.FPJump(b, next, &nefJumps)

	case ssa.BlockAMD64EQ, ssa.BlockAMD64NE,
		ssa.BlockAMD64LT, ssa.BlockAMD64GE,
		ssa.BlockAMD64LE, ssa.BlockAMD64GT,
		ssa.BlockAMD64ULT, ssa.BlockAMD64UGT,
		ssa.BlockAMD64ULE, ssa.BlockAMD64UGE:
		jmp := blockJump[b.Kind]
		switch next {
		case b.Succs[0].Block():
			s.Br(jmp.invasm, b.Succs[1].Block())
		case b.Succs[1].Block():
			s.Br(jmp.asm, b.Succs[0].Block())
		default:
			if b.Likely != ssa.BranchUnlikely {
				s.Br(jmp.asm, b.Succs[0].Block())
				s.Br(obj.AJMP, b.Succs[1].Block())
			} else {
				s.Br(jmp.invasm, b.Succs[1].Block())
				s.Br(obj.AJMP, b.Succs[0].Block())
			}
		}

	default:
		b.Fatalf("branch not implemented: %s. Control: %s", b.LongString(), b.Control.LongString())
	}
}
