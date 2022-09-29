// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dwarf_test

import (
	. "debug/dwarf"
	"errors"
	"io"
	"strings"
	"testing"
)

var (
	file1C = &LineFile{Name: "/home/austin/go.dev/src/debug/dwarf/testdata/line1.c"}
	file1H = &LineFile{Name: "/home/austin/go.dev/src/debug/dwarf/testdata/line1.h"}
	file2C = &LineFile{Name: "/home/austin/go.dev/src/debug/dwarf/testdata/line2.c"}
)

func TestLineELFGCC(t *testing.T) {
	// Generated by:
	//   # gcc --version | head -n1
	//   gcc (Ubuntu 4.8.2-19ubuntu1) 4.8.2
	//   # gcc -g -o line-gcc.elf line*.c

	// Line table based on readelf --debug-dump=rawline,decodedline
	want := []LineEntry{
		{Address: 0x40059d, File: file1H, Line: 2, IsStmt: true},
		{Address: 0x4005a5, File: file1H, Line: 2, IsStmt: true},
		{Address: 0x4005b4, File: file1H, Line: 5, IsStmt: true},
		{Address: 0x4005bd, File: file1H, Line: 6, IsStmt: true, Discriminator: 2},
		{Address: 0x4005c7, File: file1H, Line: 5, IsStmt: true, Discriminator: 2},
		{Address: 0x4005cb, File: file1H, Line: 5, IsStmt: false, Discriminator: 1},
		{Address: 0x4005d1, File: file1H, Line: 7, IsStmt: true},
		{Address: 0x4005e7, File: file1C, Line: 6, IsStmt: true},
		{Address: 0x4005eb, File: file1C, Line: 7, IsStmt: true},
		{Address: 0x4005f5, File: file1C, Line: 8, IsStmt: true},
		{Address: 0x4005ff, File: file1C, Line: 9, IsStmt: true},
		{Address: 0x400601, EndSequence: true},

		{Address: 0x400601, File: file2C, Line: 4, IsStmt: true},
		{Address: 0x400605, File: file2C, Line: 5, IsStmt: true},
		{Address: 0x40060f, File: file2C, Line: 6, IsStmt: true},
		{Address: 0x400611, EndSequence: true},
	}
	files := [][]*LineFile{{nil, file1H, file1C}, {nil, file2C}}

	testLineTable(t, want, files, elfData(t, "testdata/line-gcc.elf"))
}

func TestLineGCCWindows(t *testing.T) {
	// Generated by:
	//   > gcc --version
	//   gcc (tdm64-1) 4.9.2
	//   > gcc -g -o line-gcc-win.bin line1.c C:\workdir\go\src\debug\dwarf\testdata\line2.c

	toWindows := func(lf *LineFile) *LineFile {
		lf2 := *lf
		lf2.Name = strings.Replace(lf2.Name, "/home/austin/go.dev/", "C:\\workdir\\go\\", -1)
		lf2.Name = strings.Replace(lf2.Name, "/", "\\", -1)
		return &lf2
	}
	file1C := toWindows(file1C)
	file1H := toWindows(file1H)
	file2C := toWindows(file2C)

	// Line table based on objdump --dwarf=rawline,decodedline
	want := []LineEntry{
		{Address: 0x401530, File: file1H, Line: 2, IsStmt: true},
		{Address: 0x401538, File: file1H, Line: 5, IsStmt: true},
		{Address: 0x401541, File: file1H, Line: 6, IsStmt: true, Discriminator: 3},
		{Address: 0x40154b, File: file1H, Line: 5, IsStmt: true, Discriminator: 3},
		{Address: 0x40154f, File: file1H, Line: 5, IsStmt: false, Discriminator: 1},
		{Address: 0x401555, File: file1H, Line: 7, IsStmt: true},
		{Address: 0x40155b, File: file1C, Line: 6, IsStmt: true},
		{Address: 0x401563, File: file1C, Line: 6, IsStmt: true},
		{Address: 0x401568, File: file1C, Line: 7, IsStmt: true},
		{Address: 0x40156d, File: file1C, Line: 8, IsStmt: true},
		{Address: 0x401572, File: file1C, Line: 9, IsStmt: true},
		{Address: 0x401578, EndSequence: true},

		{Address: 0x401580, File: file2C, Line: 4, IsStmt: true},
		{Address: 0x401588, File: file2C, Line: 5, IsStmt: true},
		{Address: 0x401595, File: file2C, Line: 6, IsStmt: true},
		{Address: 0x40159b, EndSequence: true},
	}
	files := [][]*LineFile{{nil, file1H, file1C}, {nil, file2C}}

	testLineTable(t, want, files, peData(t, "testdata/line-gcc-win.bin"))
}

func TestLineELFClang(t *testing.T) {
	// Generated by:
	//   # clang --version | head -n1
	//   Ubuntu clang version 3.4-1ubuntu3 (tags/RELEASE_34/final) (based on LLVM 3.4)
	//   # clang -g -o line-clang.elf line*.

	want := []LineEntry{
		{Address: 0x400530, File: file1C, Line: 6, IsStmt: true},
		{Address: 0x400534, File: file1C, Line: 7, IsStmt: true, PrologueEnd: true},
		{Address: 0x400539, File: file1C, Line: 8, IsStmt: true},
		{Address: 0x400545, File: file1C, Line: 9, IsStmt: true},
		{Address: 0x400550, File: file1H, Line: 2, IsStmt: true},
		{Address: 0x400554, File: file1H, Line: 5, IsStmt: true, PrologueEnd: true},
		{Address: 0x400568, File: file1H, Line: 6, IsStmt: true},
		{Address: 0x400571, File: file1H, Line: 5, IsStmt: true},
		{Address: 0x400581, File: file1H, Line: 7, IsStmt: true},
		{Address: 0x400583, EndSequence: true},

		{Address: 0x400590, File: file2C, Line: 4, IsStmt: true},
		{Address: 0x4005a0, File: file2C, Line: 5, IsStmt: true, PrologueEnd: true},
		{Address: 0x4005a7, File: file2C, Line: 6, IsStmt: true},
		{Address: 0x4005b0, EndSequence: true},
	}
	files := [][]*LineFile{{nil, file1C, file1H}, {nil, file2C}}

	testLineTable(t, want, files, elfData(t, "testdata/line-clang.elf"))
}

func TestLineRnglists(t *testing.T) {
	// Test a newer file, generated by clang.
	file := &LineFile{Name: "/usr/local/google/home/iant/foo.c"}
	want := []LineEntry{
		{Address: 0x401020, File: file, Line: 12, IsStmt: true},
		{Address: 0x401020, File: file, Line: 13, Column: 12, IsStmt: true, PrologueEnd: true},
		{Address: 0x401022, File: file, Line: 13, Column: 7},
		{Address: 0x401024, File: file, Line: 17, Column: 1, IsStmt: true},
		{Address: 0x401027, File: file, Line: 16, Column: 10, IsStmt: true},
		{Address: 0x40102c, EndSequence: true},
		{Address: 0x401000, File: file, Line: 2, IsStmt: true},
		{Address: 0x401000, File: file, Line: 6, Column: 17, IsStmt: true, PrologueEnd: true},
		{Address: 0x401002, File: file, Line: 6, Column: 3},
		{Address: 0x401019, File: file, Line: 9, Column: 3, IsStmt: true},
		{Address: 0x40101a, File: file, Line: 0, Column: 3},
		{Address: 0x40101c, File: file, Line: 9, Column: 3},
		{Address: 0x40101d, EndSequence: true},
	}
	files := [][]*LineFile{{file}}

	testLineTable(t, want, files, elfData(t, "testdata/rnglistx.elf"))
}

func TestLineSeek(t *testing.T) {
	d := elfData(t, "testdata/line-gcc.elf")

	// Get the line table for the first CU.
	cu, err := d.Reader().Next()
	if err != nil {
		t.Fatal("d.Reader().Next:", err)
	}
	lr, err := d.LineReader(cu)
	if err != nil {
		t.Fatal("d.LineReader:", err)
	}

	// Read entries forward.
	var line LineEntry
	var posTable []LineReaderPos
	var table []LineEntry
	for {
		posTable = append(posTable, lr.Tell())

		err := lr.Next(&line)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatal("lr.Next:", err)
		}
		table = append(table, line)
	}

	// Test that Reset returns to the first line.
	lr.Reset()
	if err := lr.Next(&line); err != nil {
		t.Fatal("lr.Next after Reset failed:", err)
	} else if line != table[0] {
		t.Fatal("lr.Next after Reset returned", line, "instead of", table[0])
	}

	// Check that entries match when seeking backward.
	for i := len(posTable) - 1; i >= 0; i-- {
		lr.Seek(posTable[i])
		err := lr.Next(&line)
		if i == len(posTable)-1 {
			if err != io.EOF {
				t.Fatal("expected io.EOF after seek to end, got", err)
			}
		} else if err != nil {
			t.Fatal("lr.Next after seek to", posTable[i], "failed:", err)
		} else if line != table[i] {
			t.Fatal("lr.Next after seek to", posTable[i], "returned", line, "instead of", table[i])
		}
	}

	// Check that seeking to a PC returns the right line.
	if err := lr.SeekPC(table[0].Address-1, &line); err != ErrUnknownPC {
		t.Fatalf("lr.SeekPC to %#x returned %v instead of ErrUnknownPC", table[0].Address-1, err)
	}
	for i, testLine := range table {
		if testLine.EndSequence {
			if err := lr.SeekPC(testLine.Address, &line); err != ErrUnknownPC {
				t.Fatalf("lr.SeekPC to %#x returned %v instead of ErrUnknownPC", testLine.Address, err)
			}
			continue
		}

		nextPC := table[i+1].Address
		for pc := testLine.Address; pc < nextPC; pc++ {
			if err := lr.SeekPC(pc, &line); err != nil {
				t.Fatalf("lr.SeekPC to %#x failed: %v", pc, err)
			} else if line != testLine {
				t.Fatalf("lr.SeekPC to %#x returned %v instead of %v", pc, line, testLine)
			}
		}
	}
}

func testLineTable(t *testing.T, want []LineEntry, files [][]*LineFile, d *Data) {
	// Get line table from d.
	var got []LineEntry
	dr := d.Reader()
	for {
		ent, err := dr.Next()
		if err != nil {
			t.Fatal("dr.Next:", err)
		} else if ent == nil {
			break
		}

		if ent.Tag != TagCompileUnit {
			dr.SkipChildren()
			continue
		}

		// Ignore system compilation units (this happens in
		// the Windows binary). We'll still decode the line
		// table, but won't check it.
		name := ent.Val(AttrName).(string)
		ignore := strings.HasPrefix(name, "C:/crossdev/") || strings.HasPrefix(name, "../../")

		// Decode CU's line table.
		lr, err := d.LineReader(ent)
		if err != nil {
			t.Fatal("d.LineReader:", err)
		} else if lr == nil {
			continue
		}

		for {
			var line LineEntry
			err := lr.Next(&line)
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				t.Fatal("lr.Next:", err)
			}
			// Ignore sources from the Windows build environment.
			if ignore {
				continue
			}
			got = append(got, line)
		}

		// Check file table.
		if !ignore {
			if !compareFiles(files[0], lr.Files()) {
				t.Log("File tables do not match. Got:")
				dumpFiles(t, lr.Files())
				t.Log("Want:")
				dumpFiles(t, files[0])
				t.Fail()
			}
			files = files[1:]
		}
	}

	// Compare line tables.
	if !compareLines(got, want) {
		t.Log("Line tables do not match. Got:")
		dumpLines(t, got)
		t.Log("Want:")
		dumpLines(t, want)
		t.FailNow()
	}
}

func compareFiles(a, b []*LineFile) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] == nil && b[i] == nil {
			continue
		}
		if a[i] != nil && b[i] != nil && a[i].Name == b[i].Name {
			continue
		}
		return false
	}
	return true
}

func dumpFiles(t *testing.T, files []*LineFile) {
	for i, f := range files {
		name := "<nil>"
		if f != nil {
			name = f.Name
		}
		t.Logf("  %d %s", i, name)
	}
}

func compareLines(a, b []LineEntry) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		al, bl := a[i], b[i]
		// If both are EndSequence, then the only other valid
		// field is Address. Otherwise, test equality of all
		// fields.
		if al.EndSequence && bl.EndSequence && al.Address == bl.Address {
			continue
		}
		if al.File.Name != bl.File.Name {
			return false
		}
		al.File = nil
		bl.File = nil
		if al != bl {
			return false
		}
	}
	return true
}

func dumpLines(t *testing.T, lines []LineEntry) {
	for _, l := range lines {
		t.Logf("  %+v File:%+v", l, l.File)
	}
}

type joinTest struct {
	dirname, filename string
	path              string
}

var joinTests = []joinTest{
	{"a", "b", "a/b"},
	{"a", "", "a"},
	{"", "b", "b"},
	{"/a", "b", "/a/b"},
	{"/a/", "b", "/a/b"},

	{`C:\Windows\`, `System32`, `C:\Windows\System32`},
	{`C:\Windows\`, ``, `C:\Windows\`},
	{`C:\`, `Windows`, `C:\Windows`},
	{`C:\Windows\`, `C:System32`, `C:\Windows\System32`},
	{`C:\Windows`, `a/b`, `C:\Windows\a/b`},
	{`\\host\share\`, `foo`, `\\host\share\foo`},
	{`\\host\share\`, `foo\bar`, `\\host\share\foo\bar`},
	{`//host/share/`, `foo/bar`, `//host/share/foo/bar`},

	// Note: the Go compiler currently emits DWARF line table paths
	// with '/' instead of '\' (see issues #19784, #36495). These
	// tests are to cover cases that might come up for Windows Go
	// binaries.
	{`c:/workdir/go/src/x`, `y.go`, `c:/workdir/go/src/x/y.go`},
	{`d:/some/thing/`, `b.go`, `d:/some/thing/b.go`},
	{`e:\blah\`, `foo.c`, `e:\blah\foo.c`},

	// The following are "best effort". We shouldn't see relative
	// base directories in DWARF, but these test that pathJoin
	// doesn't fail miserably if it sees one.
	{`C:`, `a`, `C:a`},
	{`C:`, `a\b`, `C:a\b`},
	{`C:.`, `a`, `C:.\a`},
	{`C:a`, `b`, `C:a\b`},
}

func TestPathJoin(t *testing.T) {
	for _, test := range joinTests {
		got := PathJoin(test.dirname, test.filename)
		if test.path != got {
			t.Errorf("pathJoin(%q, %q) = %q, want %q", test.dirname, test.filename, got, test.path)
		}
	}
}

func TestPathLineReaderMalformed(t *testing.T) {
	// This test case drawn from issue #52354. What's happening
	// here is that the stmtList attribute in the compilation
	// unit is malformed (negative).
	var aranges, frame, pubnames, ranges, str []byte
	abbrev := []byte{0x10, 0x20, 0x20, 0x20, 0x21, 0x20, 0x10, 0x21, 0x61,
		0x0, 0x0, 0xff, 0x20, 0xff, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20,
		0x20, 0x20, 0x20, 0x20, 0x20, 0x20}
	info := []byte{0x0, 0x0, 0x0, 0x9, 0x0, 0x2, 0x0, 0x0, 0x0, 0x0,
		0x20, 0x10, 0x10}
	line := []byte{0x20}
	Data0, err := New(abbrev, aranges, frame, info, line, pubnames, ranges, str)
	if err != nil {
		t.Fatalf("error unexpected: %v", err)
	}
	Reader0 := Data0.Reader()
	Entry0, err := Reader0.Next()
	if err != nil {
		t.Fatalf("error unexpected: %v", err)
	}
	LineReader0, err := Data0.LineReader(Entry0)
	if err == nil {
		t.Fatalf("expected error")
	}
	if LineReader0 != nil {
		t.Fatalf("expected nil line reader")
	}
}
