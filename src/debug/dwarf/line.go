// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dwarf

import (
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
)

// A LineReader reads a sequence of LineEntry structures from a DWARF
// "line" section for a single compilation unit. LineEntries occur in
// order of increasing PC and each LineEntry gives metadata for the
// instructions from that LineEntry's PC to just before the next
// LineEntry's PC. The last entry will have its EndSequence field set.
type LineReader struct {
	buf buf

	// Original .debug_line section data. Used by Seek.
	section []byte

	str     []byte // .debug_str
	lineStr []byte // .debug_line_str

	// Header information
	version              uint16
	addrsize             int
	segmentSelectorSize  int
	minInstructionLength int
	maxOpsPerInstruction int
	defaultIsStmt        bool
	lineBase             int
	lineRange            int
	opcodeBase           int
	opcodeLengths        []int
	directories          []string
	fileEntries          []*LineFile

	programOffset Offset // section offset of line number program
	endOffset     Offset // section offset of byte following program

	initialFileEntries int // initial length of fileEntries

	// Current line number program state machine registers
	state     LineEntry // public state
	fileIndex int       // private state
}

// A LineEntry is a row in a DWARF line table.
type LineEntry struct {
	// Address is the program-counter value of a machine
	// instruction generated by the compiler. This LineEntry
	// applies to each instruction from Address to just before the
	// Address of the next LineEntry.
	Address uint64

	// OpIndex is the index of an operation within a VLIW
	// instruction. The index of the first operation is 0. For
	// non-VLIW architectures, it will always be 0. Address and
	// OpIndex together form an operation pointer that can
	// reference any individual operation within the instruction
	// stream.
	OpIndex int

	// File is the source file corresponding to these
	// instructions.
	File *LineFile

	// Line is the source code line number corresponding to these
	// instructions. Lines are numbered beginning at 1. It may be
	// 0 if these instructions cannot be attributed to any source
	// line.
	Line int

	// Column is the column number within the source line of these
	// instructions. Columns are numbered beginning at 1. It may
	// be 0 to indicate the "left edge" of the line.
	Column int

	// IsStmt indicates that Address is a recommended breakpoint
	// location, such as the beginning of a line, statement, or a
	// distinct subpart of a statement.
	IsStmt bool

	// BasicBlock indicates that Address is the beginning of a
	// basic block.
	BasicBlock bool

	// PrologueEnd indicates that Address is one (of possibly
	// many) PCs where execution should be suspended for a
	// breakpoint on entry to the containing function.
	//
	// Added in DWARF 3.
	PrologueEnd bool

	// EpilogueBegin indicates that Address is one (of possibly
	// many) PCs where execution should be suspended for a
	// breakpoint on exit from this function.
	//
	// Added in DWARF 3.
	EpilogueBegin bool

	// ISA is the instruction set architecture for these
	// instructions. Possible ISA values should be defined by the
	// applicable ABI specification.
	//
	// Added in DWARF 3.
	ISA int

	// Discriminator is an arbitrary integer indicating the block
	// to which these instructions belong. It serves to
	// distinguish among multiple blocks that may all have with
	// the same source file, line, and column. Where only one
	// block exists for a given source position, it should be 0.
	//
	// Added in DWARF 3.
	Discriminator int

	// EndSequence indicates that Address is the first byte after
	// the end of a sequence of target machine instructions. If it
	// is set, only this and the Address field are meaningful. A
	// line number table may contain information for multiple
	// potentially disjoint instruction sequences. The last entry
	// in a line table should always have EndSequence set.
	EndSequence bool
}

// A LineFile is a source file referenced by a DWARF line table entry.
type LineFile struct {
	Name   string
	Mtime  uint64 // Implementation defined modification time, or 0 if unknown
	Length int    // File length, or 0 if unknown
}

// LineReader returns a new reader for the line table of compilation
// unit cu, which must be an Entry with tag TagCompileUnit.
//
// If this compilation unit has no line table, it returns nil, nil.
func (d *Data) LineReader(cu *Entry) (*LineReader, error) {
	if d.line == nil {
		// No line tables available.
		return nil, nil
	}

	// Get line table information from cu.
	off, ok := cu.Val(AttrStmtList).(int64)
	if !ok {
		// cu has no line table.
		return nil, nil
	}
	if off < 0 || off > int64(len(d.line)) {
		return nil, errors.New("AttrStmtList value out of range")
	}
	// AttrCompDir is optional if all file names are absolute. Use
	// the empty string if it's not present.
	compDir, _ := cu.Val(AttrCompDir).(string)

	// Create the LineReader.
	u := &d.unit[d.offsetToUnit(cu.Offset)]
	buf := makeBuf(d, u, "line", Offset(off), d.line[off:])
	// The compilation directory is implicitly directories[0].
	r := LineReader{
		buf:     buf,
		section: d.line,
		str:     d.str,
		lineStr: d.lineStr,
	}

	// Read the header.
	if err := r.readHeader(compDir); err != nil {
		return nil, err
	}

	// Initialize line reader state.
	r.Reset()

	return &r, nil
}

// readHeader reads the line number program header from r.buf and sets
// all of the header fields in r.
func (r *LineReader) readHeader(compDir string) error {
	buf := &r.buf

	// Read basic header fields [DWARF2 6.2.4].
	hdrOffset := buf.off
	unitLength, dwarf64 := buf.unitLength()
	r.endOffset = buf.off + unitLength
	if r.endOffset > buf.off+Offset(len(buf.data)) {
		return DecodeError{"line", hdrOffset, fmt.Sprintf("line table end %d exceeds section size %d", r.endOffset, buf.off+Offset(len(buf.data)))}
	}
	r.version = buf.uint16()
	if buf.err == nil && (r.version < 2 || r.version > 5) {
		// DWARF goes to all this effort to make new opcodes
		// backward-compatible, and then adds fields right in
		// the middle of the header in new versions, so we're
		// picky about only supporting known line table
		// versions.
		return DecodeError{"line", hdrOffset, fmt.Sprintf("unknown line table version %d", r.version)}
	}
	if r.version >= 5 {
		r.addrsize = int(buf.uint8())
		r.segmentSelectorSize = int(buf.uint8())
	} else {
		r.addrsize = buf.format.addrsize()
		r.segmentSelectorSize = 0
	}
	var headerLength Offset
	if dwarf64 {
		headerLength = Offset(buf.uint64())
	} else {
		headerLength = Offset(buf.uint32())
	}
	programOffset := buf.off + headerLength
	if programOffset > r.endOffset {
		return DecodeError{"line", hdrOffset, fmt.Sprintf("malformed line table: program offset %d exceeds end offset %d", programOffset, r.endOffset)}
	}
	r.programOffset = programOffset
	r.minInstructionLength = int(buf.uint8())
	if r.version >= 4 {
		// [DWARF4 6.2.4]
		r.maxOpsPerInstruction = int(buf.uint8())
	} else {
		r.maxOpsPerInstruction = 1
	}
	r.defaultIsStmt = buf.uint8() != 0
	r.lineBase = int(int8(buf.uint8()))
	r.lineRange = int(buf.uint8())

	// Validate header.
	if buf.err != nil {
		return buf.err
	}
	if r.maxOpsPerInstruction == 0 {
		return DecodeError{"line", hdrOffset, "invalid maximum operations per instruction: 0"}
	}
	if r.lineRange == 0 {
		return DecodeError{"line", hdrOffset, "invalid line range: 0"}
	}

	// Read standard opcode length table. This table starts with opcode 1.
	r.opcodeBase = int(buf.uint8())
	r.opcodeLengths = make([]int, r.opcodeBase)
	for i := 1; i < r.opcodeBase; i++ {
		r.opcodeLengths[i] = int(buf.uint8())
	}

	// Validate opcode lengths.
	if buf.err != nil {
		return buf.err
	}
	for i, length := range r.opcodeLengths {
		if known, ok := knownOpcodeLengths[i]; ok && known != length {
			return DecodeError{"line", hdrOffset, fmt.Sprintf("opcode %d expected to have length %d, but has length %d", i, known, length)}
		}
	}

	if r.version < 5 {
		// Read include directories table.
		r.directories = []string{compDir}
		for {
			directory := buf.string()
			if buf.err != nil {
				return buf.err
			}
			if len(directory) == 0 {
				break
			}
			if !pathIsAbs(directory) {
				// Relative paths are implicitly relative to
				// the compilation directory.
				directory = pathJoin(compDir, directory)
			}
			r.directories = append(r.directories, directory)
		}

		// Read file name list. File numbering starts with 1,
		// so leave the first entry nil.
		r.fileEntries = make([]*LineFile, 1)
		for {
			if done, err := r.readFileEntry(); err != nil {
				return err
			} else if done {
				break
			}
		}
	} else {
		dirFormat := r.readLNCTFormat()
		c := buf.uint()
		r.directories = make([]string, c)
		for i := range r.directories {
			dir, _, _, err := r.readLNCT(dirFormat, dwarf64)
			if err != nil {
				return err
			}
			r.directories[i] = dir
		}
		fileFormat := r.readLNCTFormat()
		c = buf.uint()
		r.fileEntries = make([]*LineFile, c)
		for i := range r.fileEntries {
			name, mtime, size, err := r.readLNCT(fileFormat, dwarf64)
			if err != nil {
				return err
			}
			r.fileEntries[i] = &LineFile{name, mtime, int(size)}
		}
	}

	r.initialFileEntries = len(r.fileEntries)

	return buf.err
}

// lnctForm is a pair of an LNCT code and a form. This represents an
// entry in the directory name or file name description in the DWARF 5
// line number program header.
type lnctForm struct {
	lnct int
	form format
}

// readLNCTFormat reads an LNCT format description.
func (r *LineReader) readLNCTFormat() []lnctForm {
	c := r.buf.uint8()
	ret := make([]lnctForm, c)
	for i := range ret {
		ret[i].lnct = int(r.buf.uint())
		ret[i].form = format(r.buf.uint())
	}
	return ret
}

// readLNCT reads a sequence of LNCT entries and returns path information.
func (r *LineReader) readLNCT(s []lnctForm, dwarf64 bool) (path string, mtime uint64, size uint64, err error) {
	var dir string
	for _, lf := range s {
		var str string
		var val uint64
		switch lf.form {
		case formString:
			str = r.buf.string()
		case formStrp, formLineStrp:
			var off uint64
			if dwarf64 {
				off = r.buf.uint64()
			} else {
				off = uint64(r.buf.uint32())
			}
			if uint64(int(off)) != off {
				return "", 0, 0, DecodeError{"line", r.buf.off, "strp/line_strp offset out of range"}
			}
			var b1 buf
			if lf.form == formStrp {
				b1 = makeBuf(r.buf.dwarf, r.buf.format, "str", 0, r.str)
			} else {
				b1 = makeBuf(r.buf.dwarf, r.buf.format, "line_str", 0, r.lineStr)
			}
			b1.skip(int(off))
			str = b1.string()
			if b1.err != nil {
				return "", 0, 0, DecodeError{"line", r.buf.off, b1.err.Error()}
			}
		case formStrpSup:
			// Supplemental sections not yet supported.
			if dwarf64 {
				r.buf.uint64()
			} else {
				r.buf.uint32()
			}
		case formStrx:
			// .debug_line.dwo sections not yet supported.
			r.buf.uint()
		case formStrx1:
			r.buf.uint8()
		case formStrx2:
			r.buf.uint16()
		case formStrx3:
			r.buf.uint24()
		case formStrx4:
			r.buf.uint32()
		case formData1:
			val = uint64(r.buf.uint8())
		case formData2:
			val = uint64(r.buf.uint16())
		case formData4:
			val = uint64(r.buf.uint32())
		case formData8:
			val = r.buf.uint64()
		case formData16:
			r.buf.bytes(16)
		case formDwarfBlock:
			r.buf.bytes(int(r.buf.uint()))
		case formUdata:
			val = r.buf.uint()
		}

		switch lf.lnct {
		case lnctPath:
			path = str
		case lnctDirectoryIndex:
			if val >= uint64(len(r.directories)) {
				return "", 0, 0, DecodeError{"line", r.buf.off, "directory index out of range"}
			}
			dir = r.directories[val]
		case lnctTimestamp:
			mtime = val
		case lnctSize:
			size = val
		case lnctMD5:
			// Ignored.
		}
	}

	if dir != "" && path != "" {
		path = pathJoin(dir, path)
	}

	return path, mtime, size, nil
}

// readFileEntry reads a file entry from either the header or a
// DW_LNE_define_file extended opcode and adds it to r.fileEntries. A
// true return value indicates that there are no more entries to read.
func (r *LineReader) readFileEntry() (bool, error) {
	name := r.buf.string()
	if r.buf.err != nil {
		return false, r.buf.err
	}
	if len(name) == 0 {
		return true, nil
	}
	off := r.buf.off
	dirIndex := int(r.buf.uint())
	if !pathIsAbs(name) {
		if dirIndex >= len(r.directories) {
			return false, DecodeError{"line", off, "directory index too large"}
		}
		name = pathJoin(r.directories[dirIndex], name)
	}
	mtime := r.buf.uint()
	length := int(r.buf.uint())

	// If this is a dynamically added path and the cursor was
	// backed up, we may have already added this entry. Avoid
	// updating existing line table entries in this case. This
	// avoids an allocation and potential racy access to the slice
	// backing store if the user called Files.
	if len(r.fileEntries) < cap(r.fileEntries) {
		fe := r.fileEntries[:len(r.fileEntries)+1]
		if fe[len(fe)-1] != nil {
			// We already processed this addition.
			r.fileEntries = fe
			return false, nil
		}
	}
	r.fileEntries = append(r.fileEntries, &LineFile{name, mtime, length})
	return false, nil
}

// updateFile updates r.state.File after r.fileIndex has
// changed or r.fileEntries has changed.
func (r *LineReader) updateFile() {
	if r.fileIndex < len(r.fileEntries) {
		r.state.File = r.fileEntries[r.fileIndex]
	} else {
		r.state.File = nil
	}
}

// Next sets *entry to the next row in this line table and moves to
// the next row. If there are no more entries and the line table is
// properly terminated, it returns io.EOF.
//
// Rows are always in order of increasing entry.Address, but
// entry.Line may go forward or backward.
func (r *LineReader) Next(entry *LineEntry) error {
	if r.buf.err != nil {
		return r.buf.err
	}

	// Execute opcodes until we reach an opcode that emits a line
	// table entry.
	for {
		if len(r.buf.data) == 0 {
			return io.EOF
		}
		emit := r.step(entry)
		if r.buf.err != nil {
			return r.buf.err
		}
		if emit {
			return nil
		}
	}
}

// knownOpcodeLengths gives the opcode lengths (in varint arguments)
// of known standard opcodes.
var knownOpcodeLengths = map[int]int{
	lnsCopy:             0,
	lnsAdvancePC:        1,
	lnsAdvanceLine:      1,
	lnsSetFile:          1,
	lnsNegateStmt:       0,
	lnsSetBasicBlock:    0,
	lnsConstAddPC:       0,
	lnsSetPrologueEnd:   0,
	lnsSetEpilogueBegin: 0,
	lnsSetISA:           1,
	// lnsFixedAdvancePC takes a uint8 rather than a varint; it's
	// unclear what length the header is supposed to claim, so
	// ignore it.
}

// step processes the next opcode and updates r.state. If the opcode
// emits a row in the line table, this updates *entry and returns
// true.
func (r *LineReader) step(entry *LineEntry) bool {
	opcode := int(r.buf.uint8())

	if opcode >= r.opcodeBase {
		// Special opcode [DWARF2 6.2.5.1, DWARF4 6.2.5.1]
		adjustedOpcode := opcode - r.opcodeBase
		r.advancePC(adjustedOpcode / r.lineRange)
		lineDelta := r.lineBase + adjustedOpcode%r.lineRange
		r.state.Line += lineDelta
		goto emit
	}

	switch opcode {
	case 0:
		// Extended opcode [DWARF2 6.2.5.3]
		length := Offset(r.buf.uint())
		startOff := r.buf.off
		opcode := r.buf.uint8()

		switch opcode {
		case lneEndSequence:
			r.state.EndSequence = true
			*entry = r.state
			r.resetState()

		case lneSetAddress:
			switch r.addrsize {
			case 1:
				r.state.Address = uint64(r.buf.uint8())
			case 2:
				r.state.Address = uint64(r.buf.uint16())
			case 4:
				r.state.Address = uint64(r.buf.uint32())
			case 8:
				r.state.Address = r.buf.uint64()
			default:
				r.buf.error("unknown address size")
			}

		case lneDefineFile:
			if done, err := r.readFileEntry(); err != nil {
				r.buf.err = err
				return false
			} else if done {
				r.buf.err = DecodeError{"line", startOff, "malformed DW_LNE_define_file operation"}
				return false
			}
			r.updateFile()

		case lneSetDiscriminator:
			// [DWARF4 6.2.5.3]
			r.state.Discriminator = int(r.buf.uint())
		}

		r.buf.skip(int(startOff + length - r.buf.off))

		if opcode == lneEndSequence {
			return true
		}

	// Standard opcodes [DWARF2 6.2.5.2]
	case lnsCopy:
		goto emit

	case lnsAdvancePC:
		r.advancePC(int(r.buf.uint()))

	case lnsAdvanceLine:
		r.state.Line += int(r.buf.int())

	case lnsSetFile:
		r.fileIndex = int(r.buf.uint())
		r.updateFile()

	case lnsSetColumn:
		r.state.Column = int(r.buf.uint())

	case lnsNegateStmt:
		r.state.IsStmt = !r.state.IsStmt

	case lnsSetBasicBlock:
		r.state.BasicBlock = true

	case lnsConstAddPC:
		r.advancePC((255 - r.opcodeBase) / r.lineRange)

	case lnsFixedAdvancePC:
		r.state.Address += uint64(r.buf.uint16())

	// DWARF3 standard opcodes [DWARF3 6.2.5.2]
	case lnsSetPrologueEnd:
		r.state.PrologueEnd = true

	case lnsSetEpilogueBegin:
		r.state.EpilogueBegin = true

	case lnsSetISA:
		r.state.ISA = int(r.buf.uint())

	default:
		// Unhandled standard opcode. Skip the number of
		// arguments that the prologue says this opcode has.
		for i := 0; i < r.opcodeLengths[opcode]; i++ {
			r.buf.uint()
		}
	}
	return false

emit:
	*entry = r.state
	r.state.BasicBlock = false
	r.state.PrologueEnd = false
	r.state.EpilogueBegin = false
	r.state.Discriminator = 0
	return true
}

// advancePC advances "operation pointer" (the combination of Address
// and OpIndex) in r.state by opAdvance steps.
func (r *LineReader) advancePC(opAdvance int) {
	opIndex := r.state.OpIndex + opAdvance
	r.state.Address += uint64(r.minInstructionLength * (opIndex / r.maxOpsPerInstruction))
	r.state.OpIndex = opIndex % r.maxOpsPerInstruction
}

// A LineReaderPos represents a position in a line table.
type LineReaderPos struct {
	// off is the current offset in the DWARF line section.
	off Offset
	// numFileEntries is the length of fileEntries.
	numFileEntries int
	// state and fileIndex are the statement machine state at
	// offset off.
	state     LineEntry
	fileIndex int
}

// Tell returns the current position in the line table.
func (r *LineReader) Tell() LineReaderPos {
	return LineReaderPos{r.buf.off, len(r.fileEntries), r.state, r.fileIndex}
}

// Seek restores the line table reader to a position returned by Tell.
//
// The argument pos must have been returned by a call to Tell on this
// line table.
func (r *LineReader) Seek(pos LineReaderPos) {
	r.buf.off = pos.off
	r.buf.data = r.section[r.buf.off:r.endOffset]
	r.fileEntries = r.fileEntries[:pos.numFileEntries]
	r.state = pos.state
	r.fileIndex = pos.fileIndex
}

// Reset repositions the line table reader at the beginning of the
// line table.
func (r *LineReader) Reset() {
	// Reset buffer to the line number program offset.
	r.buf.off = r.programOffset
	r.buf.data = r.section[r.buf.off:r.endOffset]

	// Reset file entries list.
	r.fileEntries = r.fileEntries[:r.initialFileEntries]

	// Reset line number program state.
	r.resetState()
}

// resetState resets r.state to its default values
func (r *LineReader) resetState() {
	// Reset the state machine registers to the defaults given in
	// [DWARF4 6.2.2].
	r.state = LineEntry{
		Address:       0,
		OpIndex:       0,
		File:          nil,
		Line:          1,
		Column:        0,
		IsStmt:        r.defaultIsStmt,
		BasicBlock:    false,
		PrologueEnd:   false,
		EpilogueBegin: false,
		ISA:           0,
		Discriminator: 0,
	}
	r.fileIndex = 1
	r.updateFile()
}

// Files returns the file name table of this compilation unit as of
// the current position in the line table. The file name table may be
// referenced from attributes in this compilation unit such as
// AttrDeclFile.
//
// Entry 0 is always nil, since file index 0 represents "no file".
//
// The file name table of a compilation unit is not fixed. Files
// returns the file table as of the current position in the line
// table. This may contain more entries than the file table at an
// earlier position in the line table, though existing entries never
// change.
func (r *LineReader) Files() []*LineFile {
	return r.fileEntries
}

// ErrUnknownPC is the error returned by LineReader.ScanPC when the
// seek PC is not covered by any entry in the line table.
var ErrUnknownPC = errors.New("ErrUnknownPC")

// SeekPC sets *entry to the LineEntry that includes pc and positions
// the reader on the next entry in the line table. If necessary, this
// will seek backwards to find pc.
//
// If pc is not covered by any entry in this line table, SeekPC
// returns ErrUnknownPC. In this case, *entry and the final seek
// position are unspecified.
//
// Note that DWARF line tables only permit sequential, forward scans.
// Hence, in the worst case, this takes time linear in the size of the
// line table. If the caller wishes to do repeated fast PC lookups, it
// should build an appropriate index of the line table.
func (r *LineReader) SeekPC(pc uint64, entry *LineEntry) error {
	if err := r.Next(entry); err != nil {
		return err
	}
	if entry.Address > pc {
		// We're too far. Start at the beginning of the table.
		r.Reset()
		if err := r.Next(entry); err != nil {
			return err
		}
		if entry.Address > pc {
			// The whole table starts after pc.
			r.Reset()
			return ErrUnknownPC
		}
	}

	// Scan until we pass pc, then back up one.
	for {
		var next LineEntry
		pos := r.Tell()
		if err := r.Next(&next); err != nil {
			if errors.Is(err, io.EOF) {
				return ErrUnknownPC
			}
			return err
		}
		if next.Address > pc {
			if entry.EndSequence {
				// pc is in a hole in the table.
				return ErrUnknownPC
			}
			// entry is the desired entry. Back up the
			// cursor to "next" and return success.
			r.Seek(pos)
			return nil
		}
		*entry = next
	}
}

// pathIsAbs reports whether path is an absolute path (or "full path
// name" in DWARF parlance). This is in "whatever form makes sense for
// the host system", so this accepts both UNIX-style and DOS-style
// absolute paths. We avoid the filepath package because we want this
// to behave the same regardless of our host system and because we
// don't know what system the paths came from.
func pathIsAbs(path string) bool {
	_, path = splitDrive(path)
	return len(path) > 0 && (path[0] == '/' || path[0] == '\\')
}

// pathJoin joins dirname and filename. filename must be relative.
// DWARF paths can be UNIX-style or DOS-style, so this handles both.
func pathJoin(dirname, filename string) string {
	if len(dirname) == 0 {
		return filename
	}
	// dirname should be absolute, which means we can determine
	// whether it's a DOS path reasonably reliably by looking for
	// a drive letter or UNC path.
	drive, dirname := splitDrive(dirname)
	if drive == "" {
		// UNIX-style path.
		return path.Join(dirname, filename)
	}
	// DOS-style path.
	drive2, filename := splitDrive(filename)
	if drive2 != "" {
		if !strings.EqualFold(drive, drive2) {
			// Different drives. There's not much we can
			// do here, so just ignore the directory.
			return drive2 + filename
		}
		// Drives are the same. Ignore drive on filename.
	}
	if !(strings.HasSuffix(dirname, "/") || strings.HasSuffix(dirname, `\`)) && dirname != "" {
		sep := `\`
		if strings.HasPrefix(dirname, "/") {
			sep = `/`
		}
		dirname += sep
	}
	return drive + dirname + filename
}

// splitDrive splits the DOS drive letter or UNC share point from
// path, if any. path == drive + rest
func splitDrive(path string) (drive, rest string) {
	if len(path) >= 2 && path[1] == ':' {
		if c := path[0]; 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' {
			return path[:2], path[2:]
		}
	}
	if len(path) > 3 && (path[0] == '\\' || path[0] == '/') && (path[1] == '\\' || path[1] == '/') {
		// Normalize the path so we can search for just \ below.
		npath := strings.Replace(path, "/", `\`, -1)
		// Get the host part, which must be non-empty.
		slash1 := strings.IndexByte(npath[2:], '\\') + 2
		if slash1 > 2 {
			// Get the mount-point part, which must be non-empty.
			slash2 := strings.IndexByte(npath[slash1+1:], '\\') + slash1 + 1
			if slash2 > slash1 {
				return path[:slash2], path[slash2:]
			}
		}
	}
	return "", path
}
