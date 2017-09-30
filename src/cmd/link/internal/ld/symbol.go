// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ld

import (
	"cmd/internal/objabi"
	"cmd/internal/sys"
	"debug/elf"
	"fmt"
	"log"
)

// Symbol is an entry in the symbol table.
type Symbol struct {
	Name        string
	Extname     string
	Type        SymKind
	Version     int16
	Attr        Attribute
	Localentry  uint8
	Dynid       int32
	Plt         int32
	Got         int32
	Align       int32
	Elfsym      int32
	LocalElfsym int32
	Value       int64
	Size        int64
	// ElfType is set for symbols read from shared libraries by ldshlibsyms. It
	// is not set for symbols defined by the packages being linked or by symbols
	// read by ldelf (and so is left as elf.STT_NOTYPE).
	ElfType     elf.SymType
	Sub         *Symbol
	Outer       *Symbol
	Gotype      *Symbol
	Reachparent *Symbol
	File        string
	Dynimplib   string
	Dynimpvers  string
	Sect        *Section
	FuncInfo    *FuncInfo
	// P contains the raw symbol data.
	P []byte
	R []Reloc
}

func (s *Symbol) String() string {
	if s.Version == 0 {
		return s.Name
	}
	return fmt.Sprintf("%s<%d>", s.Name, s.Version)
}

func (s *Symbol) ElfsymForReloc() int32 {
	// If putelfsym created a local version of this symbol, use that in all
	// relocations.
	if s.LocalElfsym != 0 {
		return s.LocalElfsym
	} else {
		return s.Elfsym
	}
}

func (s *Symbol) Len() int64 {
	return s.Size
}

func (s *Symbol) Grow(siz int64) {
	if int64(int(siz)) != siz {
		log.Fatalf("symgrow size %d too long", siz)
	}
	if int64(len(s.P)) >= siz {
		return
	}
	if cap(s.P) < int(siz) {
		p := make([]byte, 2*(siz+1))
		s.P = append(p[:0], s.P...)
	}
	s.P = s.P[:siz]
}

func (s *Symbol) AddBytes(bytes []byte) int64 {
	if s.Type == 0 {
		s.Type = SDATA
	}
	s.Attr |= AttrReachable
	s.P = append(s.P, bytes...)
	s.Size = int64(len(s.P))

	return s.Size
}

func (s *Symbol) AddUint8(v uint8) int64 {
	off := s.Size
	if s.Type == 0 {
		s.Type = SDATA
	}
	s.Attr |= AttrReachable
	s.Size++
	s.P = append(s.P, v)

	return off
}

func (s *Symbol) AddUint16(arch *sys.Arch, v uint16) int64 {
	return s.addUintXX(arch, uint64(v), 2)
}

func (s *Symbol) AddUint32(arch *sys.Arch, v uint32) int64 {
	return s.addUintXX(arch, uint64(v), 4)
}

func (s *Symbol) AddUint64(arch *sys.Arch, v uint64) int64 {
	return s.addUintXX(arch, v, 8)
}

func (s *Symbol) AddUint(arch *sys.Arch, v uint64) int64 {
	return s.addUintXX(arch, v, arch.PtrSize)
}

func (s *Symbol) SetUint8(arch *sys.Arch, r int64, v uint8) int64 {
	return s.setUintXX(arch, r, uint64(v), 1)
}

func (s *Symbol) SetUint32(arch *sys.Arch, r int64, v uint32) int64 {
	return s.setUintXX(arch, r, uint64(v), 4)
}

func (s *Symbol) SetUint(arch *sys.Arch, r int64, v uint64) int64 {
	return s.setUintXX(arch, r, v, int64(arch.PtrSize))
}

func (s *Symbol) AddAddrPlus(arch *sys.Arch, t *Symbol, add int64) int64 {
	if s.Type == 0 {
		s.Type = SDATA
	}
	s.Attr |= AttrReachable
	i := s.Size
	s.Size += int64(arch.PtrSize)
	s.Grow(s.Size)
	r := s.AddRel()
	r.Sym = t
	r.Off = int32(i)
	r.Siz = uint8(arch.PtrSize)
	r.Type = objabi.R_ADDR
	r.Add = add
	return i + int64(r.Siz)
}

func (s *Symbol) AddPCRelPlus(arch *sys.Arch, t *Symbol, add int64) int64 {
	if s.Type == 0 {
		s.Type = SDATA
	}
	s.Attr |= AttrReachable
	i := s.Size
	s.Size += 4
	s.Grow(s.Size)
	r := s.AddRel()
	r.Sym = t
	r.Off = int32(i)
	r.Add = add
	r.Type = objabi.R_PCREL
	r.Siz = 4
	if arch.Family == sys.S390X {
		r.Variant = RV_390_DBL
	}
	return i + int64(r.Siz)
}

func (s *Symbol) AddAddr(arch *sys.Arch, t *Symbol) int64 {
	return s.AddAddrPlus(arch, t, 0)
}

func (s *Symbol) SetAddrPlus(arch *sys.Arch, off int64, t *Symbol, add int64) int64 {
	if s.Type == 0 {
		s.Type = SDATA
	}
	s.Attr |= AttrReachable
	if off+int64(arch.PtrSize) > s.Size {
		s.Size = off + int64(arch.PtrSize)
		s.Grow(s.Size)
	}

	r := s.AddRel()
	r.Sym = t
	r.Off = int32(off)
	r.Siz = uint8(arch.PtrSize)
	r.Type = objabi.R_ADDR
	r.Add = add
	return off + int64(r.Siz)
}

func (s *Symbol) SetAddr(arch *sys.Arch, off int64, t *Symbol) int64 {
	return s.SetAddrPlus(arch, off, t, 0)
}

func (s *Symbol) AddSize(arch *sys.Arch, t *Symbol) int64 {
	if s.Type == 0 {
		s.Type = SDATA
	}
	s.Attr |= AttrReachable
	i := s.Size
	s.Size += int64(arch.PtrSize)
	s.Grow(s.Size)
	r := s.AddRel()
	r.Sym = t
	r.Off = int32(i)
	r.Siz = uint8(arch.PtrSize)
	r.Type = objabi.R_SIZE
	return i + int64(r.Siz)
}

func (s *Symbol) AddAddrPlus4(t *Symbol, add int64) int64 {
	if s.Type == 0 {
		s.Type = SDATA
	}
	s.Attr |= AttrReachable
	i := s.Size
	s.Size += 4
	s.Grow(s.Size)
	r := s.AddRel()
	r.Sym = t
	r.Off = int32(i)
	r.Siz = 4
	r.Type = objabi.R_ADDR
	r.Add = add
	return i + int64(r.Siz)
}

func (s *Symbol) AddRel() *Reloc {
	s.R = append(s.R, Reloc{})
	return &s.R[len(s.R)-1]
}

func (s *Symbol) addUintXX(arch *sys.Arch, v uint64, wid int) int64 {
	off := s.Size
	s.setUintXX(arch, off, v, int64(wid))
	return off
}

func (s *Symbol) setUintXX(arch *sys.Arch, off int64, v uint64, wid int64) int64 {
	if s.Type == 0 {
		s.Type = SDATA
	}
	s.Attr |= AttrReachable
	if s.Size < off+wid {
		s.Size = off + wid
		s.Grow(s.Size)
	}

	switch wid {
	case 1:
		s.P[off] = uint8(v)
	case 2:
		arch.ByteOrder.PutUint16(s.P[off:], uint16(v))
	case 4:
		arch.ByteOrder.PutUint32(s.P[off:], uint32(v))
	case 8:
		arch.ByteOrder.PutUint64(s.P[off:], v)
	}

	return off + wid
}
