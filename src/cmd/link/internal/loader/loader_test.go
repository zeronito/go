// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package loader

import (
	"bytes"
	"cmd/internal/objabi"
	"cmd/internal/sys"
	"cmd/link/internal/sym"
	"fmt"
	"testing"
)

// dummyAddSym adds the named symbol to the loader as if it had been
// read from a Go object file. Note that it allocates a global
// index without creating an associated object reader, so one can't
// do anything interesting with this symbol (such as look at its
// data or relocations).
func addDummyObjSym(t *testing.T, ldr *Loader, or *oReader, name string) Sym {
	idx := ldr.max + 1
	ldr.max++
	if ok := ldr.AddSym(name, 0, idx, or, false, sym.SRODATA); !ok {
		t.Errorf("AddrSym failed for '" + name + "'")
	}

	return idx
}

func TestAddMaterializedSymbol(t *testing.T) {
	edummy := func(s *sym.Symbol, str string, off int) {}
	ldr := NewLoader(0, edummy)
	dummyOreader := oReader{version: -1}
	or := &dummyOreader

	// Create some syms from a dummy object file symbol to get things going.
	ts1 := addDummyObjSym(t, ldr, or, "type.uint8")
	ts2 := addDummyObjSym(t, ldr, or, "mumble")
	ts3 := addDummyObjSym(t, ldr, or, "type.string")

	// Create some external symbols.
	es1 := ldr.AddExtSym("extnew1", 0)
	if es1 == 0 {
		t.Fatalf("AddExtSym failed for extnew1")
	}
	es1x := ldr.AddExtSym("extnew1", 0)
	if es1x != 0 {
		t.Fatalf("AddExtSym lookup: expected 0 got %d for second lookup", es1x)
	}
	es2 := ldr.AddExtSym("go.info.type.uint8", 0)
	if es2 == 0 {
		t.Fatalf("AddExtSym failed for go.info.type.uint8")
	}
	// Create a nameless symbol
	es3 := ldr.CreateExtSym("")
	if es3 == 0 {
		t.Fatalf("CreateExtSym failed for nameless sym")
	}

	// Grab symbol builder pointers
	sb1, es1 := ldr.MakeSymbolUpdater(es1)
	sb2, es2 := ldr.MakeSymbolUpdater(es2)
	sb3, es3 := ldr.MakeSymbolUpdater(es3)

	// Check get/set symbol type
	es3typ := sb3.Type()
	if es3typ != sym.Sxxx {
		t.Errorf("SymType(es3): expected %d, got %d", sym.Sxxx, es3typ)
	}
	sb2.SetType(sym.SRODATA)
	es3typ = sb2.Type()
	if es3typ != sym.SRODATA {
		t.Errorf("SymType(es3): expected %d, got %d", sym.SRODATA, es3typ)
	}

	// New symbols should not initially be reachable.
	if ldr.AttrReachable(es1) || ldr.AttrReachable(es2) || ldr.AttrReachable(es3) {
		t.Errorf("newly materialized symbols should not be reachable")
	}

	// ... however it should be possible to set/unset their reachability.
	ldr.SetAttrReachable(es3, true)
	if !ldr.AttrReachable(es3) {
		t.Errorf("expected reachable symbol after update")
	}
	ldr.SetAttrReachable(es3, false)
	if ldr.AttrReachable(es3) {
		t.Errorf("expected unreachable symbol after update")
	}

	// Test expansion of attr bitmaps
	for idx := 0; idx < 36; idx++ {
		es := ldr.AddExtSym(fmt.Sprintf("zext%d", idx), 0)
		if ldr.AttrOnList(es) {
			t.Errorf("expected OnList after creation")
		}
		ldr.SetAttrOnList(es, true)
		if !ldr.AttrOnList(es) {
			t.Errorf("expected !OnList after update")
		}
		if ldr.AttrDuplicateOK(es) {
			t.Errorf("expected DupOK after creation")
		}
		ldr.SetAttrDuplicateOK(es, true)
		if !ldr.AttrDuplicateOK(es) {
			t.Errorf("expected !DupOK after update")
		}
	}

	sb1, es1 = ldr.MakeSymbolUpdater(es1)
	sb2, es2 = ldr.MakeSymbolUpdater(es2)

	// Get/set a few other attributes
	if ldr.AttrVisibilityHidden(es3) {
		t.Errorf("expected initially not hidden")
	}
	ldr.SetAttrVisibilityHidden(es3, true)
	if !ldr.AttrVisibilityHidden(es3) {
		t.Errorf("expected hidden after update")
	}

	// Test get/set symbol value.
	toTest := []Sym{ts2, es3}
	for i, s := range toTest {
		if v := ldr.SymValue(s); v != 0 {
			t.Errorf("ldr.Value(%d): expected 0 got %d\n", s, v)
		}
		nv := int64(i + 101)
		ldr.SetSymValue(s, nv)
		if v := ldr.SymValue(s); v != nv {
			t.Errorf("ldr.SetValue(%d,%d): expected %d got %d\n", s, nv, nv, v)
		}
	}

	// Check/set alignment
	es3al := ldr.SymAlign(es3)
	if es3al != 0 {
		t.Errorf("SymAlign(es3): expected 0, got %d", es3al)
	}
	ldr.SetSymAlign(es3, 128)
	es3al = ldr.SymAlign(es3)
	if es3al != 128 {
		t.Errorf("SymAlign(es3): expected 128, got %d", es3al)
	}

	// Add some relocations to the new symbols.
	r1 := Reloc{0, 1, objabi.R_ADDR, 0, ts1}
	r2 := Reloc{3, 8, objabi.R_CALL, 0, ts2}
	r3 := Reloc{7, 1, objabi.R_USETYPE, 0, ts3}
	sb1.AddReloc(r1)
	sb1.AddReloc(r2)
	sb2.AddReloc(r3)

	// Add some data to the symbols.
	d1 := []byte{1, 2, 3}
	d2 := []byte{4, 5, 6, 7}
	sb1.AddBytes(d1)
	sb2.AddBytes(d2)

	// Now invoke the usual loader interfaces to make sure
	// we're getting the right things back for these symbols.
	// First relocations...
	expRel := [][]Reloc{[]Reloc{r1, r2}, []Reloc{r3}}
	for k, sb := range []*SymbolBuilder{sb1, sb2} {
		rsl := sb.Relocs()
		exp := expRel[k]
		if !sameRelocSlice(rsl, exp) {
			t.Errorf("expected relocs %v, got %v", exp, rsl)
		}
		relocs := ldr.Relocs(sb.Sym())
		r0 := relocs.At(0)
		if r0 != exp[0] {
			t.Errorf("expected reloc %v, got %v", exp[0], r0)
		}
	}

	// ... then data.
	dat := sb2.Data()
	if bytes.Compare(dat, d2) != 0 {
		t.Errorf("expected es2 data %v, got %v", d2, dat)
	}

	// Nameless symbol should still be nameless.
	es3name := ldr.RawSymName(es3)
	if "" != es3name {
		t.Errorf("expected es3 name of '', got '%s'", es3name)
	}

	// Read value of materialized symbol.
	es1val := sb1.Value()
	if 0 != es1val {
		t.Errorf("expected es1 value of 0, got %v", es1val)
	}

	// Test other misc methods
	irm := ldr.IsReflectMethod(es1)
	if 0 != es1val {
		t.Errorf("expected IsReflectMethod(es1) value of 0, got %v", irm)
	}

	// Writing data to a materialized symbol should mark it reachable.
	if !sb1.Reachable() || !sb2.Reachable() {
		t.Fatalf("written-to materialized symbols should be reachable")
	}
}

func sameRelocSlice(s1 []Reloc, s2 []Reloc) bool {
	if len(s1) != len(s2) {
		return false
	}
	for i := 0; i < len(s1); i++ {
		if s1[i] != s2[i] {
			return false
		}
	}
	return true
}

type addFunc func(l *Loader, s Sym, s2 Sym) Sym

func TestAddDataMethods(t *testing.T) {
	edummy := func(s *sym.Symbol, str string, off int) {}
	ldr := NewLoader(0, edummy)
	dummyOreader := oReader{version: -1}
	or := &dummyOreader

	// Populate loader with some symbols.
	addDummyObjSym(t, ldr, or, "type.uint8")
	ldr.AddExtSym("hello", 0)

	arch := sys.ArchAMD64
	var testpoints = []struct {
		which       string
		addDataFunc addFunc
		expData     []byte
		expKind     sym.SymKind
		expRel      []Reloc
	}{
		{
			which: "AddUint8",
			addDataFunc: func(l *Loader, s Sym, _ Sym) Sym {
				sb, ns := l.MakeSymbolUpdater(s)
				sb.AddUint8('a')
				return ns
			},
			expData: []byte{'a'},
			expKind: sym.SDATA,
		},
		{
			which: "AddUintXX",
			addDataFunc: func(l *Loader, s Sym, _ Sym) Sym {
				sb, ns := l.MakeSymbolUpdater(s)
				sb.AddUintXX(arch, 25185, 2)
				return ns
			},
			expData: []byte{'a', 'b'},
			expKind: sym.SDATA,
		},
		{
			which: "SetUint8",
			addDataFunc: func(l *Loader, s Sym, _ Sym) Sym {
				sb, ns := l.MakeSymbolUpdater(s)
				sb.AddUint8('a')
				sb.AddUint8('b')
				sb.SetUint8(arch, 1, 'c')
				return ns
			},
			expData: []byte{'a', 'c'},
			expKind: sym.SDATA,
		},
		{
			which: "AddString",
			addDataFunc: func(l *Loader, s Sym, _ Sym) Sym {
				sb, ns := l.MakeSymbolUpdater(s)
				sb.Addstring("hello")
				return ns
			},
			expData: []byte{'h', 'e', 'l', 'l', 'o', 0},
			expKind: sym.SNOPTRDATA,
		},
		{
			which: "AddAddrPlus",
			addDataFunc: func(l *Loader, s Sym, s2 Sym) Sym {
				sb, ns := l.MakeSymbolUpdater(s)
				sb.AddAddrPlus(arch, s2, 3)
				return ns
			},
			expData: []byte{0, 0, 0, 0, 0, 0, 0, 0},
			expKind: sym.SDATA,
			expRel:  []Reloc{Reloc{Type: objabi.R_ADDR, Size: 8, Add: 3, Sym: 6}},
		},
		{
			which: "AddAddrPlus4",
			addDataFunc: func(l *Loader, s Sym, s2 Sym) Sym {
				sb, ns := l.MakeSymbolUpdater(s)
				sb.AddAddrPlus4(arch, s2, 3)
				return ns
			},
			expData: []byte{0, 0, 0, 0},
			expKind: sym.SDATA,
			expRel:  []Reloc{Reloc{Type: objabi.R_ADDR, Size: 4, Add: 3, Sym: 7}},
		},
		{
			which: "AddCURelativeAddrPlus",
			addDataFunc: func(l *Loader, s Sym, s2 Sym) Sym {
				sb, ns := l.MakeSymbolUpdater(s)
				sb.AddCURelativeAddrPlus(arch, s2, 7)
				return ns
			},
			expData: []byte{0, 0, 0, 0, 0, 0, 0, 0},
			expKind: sym.SDATA,
			expRel:  []Reloc{Reloc{Type: objabi.R_ADDRCUOFF, Size: 8, Add: 7, Sym: 8}},
		},
	}

	var pmi Sym
	for k, tp := range testpoints {
		name := fmt.Sprintf("new%d", k+1)
		mi := ldr.AddExtSym(name, 0)
		if mi == 0 {
			t.Fatalf("AddExtSym failed for '" + name + "'")
		}
		mi = tp.addDataFunc(ldr, mi, pmi)
		if ldr.SymType(mi) != tp.expKind {
			t.Errorf("testing Loader.%s: expected kind %s got %s",
				tp.which, tp.expKind, ldr.SymType(mi))
		}
		if bytes.Compare(ldr.Data(mi), tp.expData) != 0 {
			t.Errorf("testing Loader.%s: expected data %v got %v",
				tp.which, tp.expData, ldr.Data(mi))
		}
		if !ldr.AttrReachable(mi) {
			t.Fatalf("testing Loader.%s: sym updated should be reachable", tp.which)
		}
		relocs := ldr.Relocs(mi)
		rsl := relocs.ReadAll(nil)
		if !sameRelocSlice(rsl, tp.expRel) {
			t.Fatalf("testing Loader.%s: got relocslice %+v wanted %+v",
				tp.which, rsl, tp.expRel)
		}
		pmi = mi
	}
}

func TestOuterSub(t *testing.T) {
	edummy := func(s *sym.Symbol, str string, off int) {}
	ldr := NewLoader(0, edummy)
	dummyOreader := oReader{version: -1}
	or := &dummyOreader

	// Populate loader with some symbols.
	addDummyObjSym(t, ldr, or, "type.uint8")
	es1 := ldr.AddExtSym("outer", 0)
	es2 := ldr.AddExtSym("sub1", 0)
	es3 := ldr.AddExtSym("sub2", 0)
	es4 := ldr.AddExtSym("sub3", 0)
	es5 := ldr.AddExtSym("sub4", 0)
	es6 := ldr.AddExtSym("sub5", 0)

	// Should not have an outer sym initially
	if ldr.OuterSym(es1) != 0 {
		t.Errorf("es1 outer sym set ")
	}
	if ldr.SubSym(es2) != 0 {
		t.Errorf("es2 outer sym set ")
	}

	// Establish first outer/sub relationship
	ldr.PrependSub(es1, es2)
	if ldr.OuterSym(es1) != 0 {
		t.Errorf("ldr.OuterSym(es1) got %d wanted %d", ldr.OuterSym(es1), 0)
	}
	if ldr.OuterSym(es2) != es1 {
		t.Errorf("ldr.OuterSym(es2) got %d wanted %d", ldr.OuterSym(es2), es1)
	}
	if ldr.SubSym(es1) != es2 {
		t.Errorf("ldr.SubSym(es1) got %d wanted %d", ldr.SubSym(es1), es2)
	}
	if ldr.SubSym(es2) != 0 {
		t.Errorf("ldr.SubSym(es2) got %d wanted %d", ldr.SubSym(es2), 0)
	}

	// Establish second outer/sub relationship
	ldr.PrependSub(es1, es3)
	if ldr.OuterSym(es1) != 0 {
		t.Errorf("ldr.OuterSym(es1) got %d wanted %d", ldr.OuterSym(es1), 0)
	}
	if ldr.OuterSym(es2) != es1 {
		t.Errorf("ldr.OuterSym(es2) got %d wanted %d", ldr.OuterSym(es2), es1)
	}
	if ldr.OuterSym(es3) != es1 {
		t.Errorf("ldr.OuterSym(es3) got %d wanted %d", ldr.OuterSym(es3), es1)
	}
	if ldr.SubSym(es1) != es3 {
		t.Errorf("ldr.SubSym(es1) got %d wanted %d", ldr.SubSym(es1), es3)
	}
	if ldr.SubSym(es3) != es2 {
		t.Errorf("ldr.SubSym(es3) got %d wanted %d", ldr.SubSym(es3), es2)
	}

	// Some more
	ldr.PrependSub(es1, es4)
	ldr.PrependSub(es1, es5)
	ldr.PrependSub(es1, es6)

	// Set values.
	ldr.SetSymValue(es2, 7)
	ldr.SetSymValue(es3, 1)
	ldr.SetSymValue(es4, 13)
	ldr.SetSymValue(es5, 101)
	ldr.SetSymValue(es6, 3)

	// Sort
	news := ldr.SortSub(es1)
	if news != es3 {
		t.Errorf("ldr.SortSub leader got %d wanted %d", news, es3)
	}
	pv := int64(-1)
	count := 0
	for ss := ldr.SubSym(es1); ss != 0; ss = ldr.SubSym(ss) {
		v := ldr.SymValue(ss)
		if v <= pv {
			t.Errorf("ldr.SortSub sortfail at %d: val %d >= prev val %d",
				ss, v, pv)
		}
		pv = v
		count++
	}
	if count != 5 {
		t.Errorf("expected %d in sub list got %d", 5, count)
	}
}
