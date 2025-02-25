package modindex

import (
	"cmd/go/internal/base"
	"encoding/binary"
	"go/token"
	"math"
	"sort"
	"strings"
)

const indexVersion = "go index v0"

// encodeModuleBytes produces the encoded representation of the module index.
// encodeModuleBytes may modify the packages slice.
func encodeModuleBytes(packages []*rawPackage) []byte {
	e := newEncoder()
	e.Bytes([]byte(indexVersion))
	e.Bytes([]byte{'\n'})
	stringTableOffsetPos := e.Pos() // fill this at the end
	e.Uint32(0)                     // string table offset
	e.Int(len(packages))
	sort.Slice(packages, func(i, j int) bool {
		return packages[i].dir < packages[j].dir
	})
	for _, p := range packages {
		e.String(p.dir)
	}
	packagesOffsetPos := e.Pos()
	for range packages {
		e.Int(0)
	}
	for i, p := range packages {
		e.IntAt(e.Pos(), packagesOffsetPos+4*i)
		encodePackage(e, p)
	}
	e.IntAt(e.Pos(), stringTableOffsetPos)
	e.Bytes(e.stringTable)
	return e.b
}

func encodePackageBytes(p *rawPackage) []byte {
	e := newEncoder()
	e.Bytes([]byte(indexVersion))
	e.Bytes([]byte{'\n'})
	stringTableOffsetPos := e.Pos() // fill this at the end
	e.Uint32(0)                     // string table offset
	encodePackage(e, p)
	e.IntAt(e.Pos(), stringTableOffsetPos)
	e.Bytes(e.stringTable)
	return e.b
}

func encodePackage(e *encoder, p *rawPackage) {
	e.String(p.error)
	e.String(p.dir)
	e.Int(len(p.sourceFiles))      // number of source files
	sourceFileOffsetPos := e.Pos() // the pos of the start of the source file offsets
	for range p.sourceFiles {
		e.Int(0)
	}
	for i, f := range p.sourceFiles {
		e.IntAt(e.Pos(), sourceFileOffsetPos+4*i)
		encodeFile(e, f)
	}
}

func encodeFile(e *encoder, f *rawFile) {
	e.String(f.error)
	e.String(f.parseError)
	e.String(f.synopsis)
	e.String(f.name)
	e.String(f.pkgName)
	e.Bool(f.ignoreFile)
	e.Bool(f.binaryOnly)
	e.String(f.cgoDirectives)
	e.String(f.goBuildConstraint)

	e.Int(len(f.plusBuildConstraints))
	for _, s := range f.plusBuildConstraints {
		e.String(s)
	}

	e.Int(len(f.imports))
	for _, m := range f.imports {
		e.String(m.path)
		e.Position(m.position)
	}

	e.Int(len(f.embeds))
	for _, embed := range f.embeds {
		e.String(embed.pattern)
		e.Position(embed.position)
	}
}

func newEncoder() *encoder {
	e := &encoder{strings: make(map[string]int)}

	// place the empty string at position 0 in the string table
	e.stringTable = append(e.stringTable, 0)
	e.strings[""] = 0

	return e
}

func (e *encoder) Position(position token.Position) {
	e.String(position.Filename)
	e.Int(position.Offset)
	e.Int(position.Line)
	e.Int(position.Column)
}

type encoder struct {
	b           []byte
	stringTable []byte
	strings     map[string]int
}

func (e *encoder) Pos() int {
	return len(e.b)
}

func (e *encoder) Bytes(b []byte) {
	e.b = append(e.b, b...)
}

func (e *encoder) String(s string) {
	if strings.IndexByte(s, 0) >= 0 {
		base.Fatalf("go: attempting to encode a string containing a null byte")
	}
	if n, ok := e.strings[s]; ok {
		e.Int(n)
		return
	}
	pos := len(e.stringTable)
	e.strings[s] = pos
	e.Int(pos)
	e.stringTable = append(e.stringTable, []byte(s)...)
	e.stringTable = append(e.stringTable, 0)
}

func (e *encoder) Bool(b bool) {
	if b {
		e.Uint32(1)
	} else {
		e.Uint32(0)
	}
}

func (e *encoder) Uint32(n uint32) {
	e.b = binary.LittleEndian.AppendUint32(e.b, n)
}

// Int encodes n. Note that all ints are written to the index as uint32s.
func (e *encoder) Int(n int) {
	if n < 0 || int64(n) > math.MaxUint32 {
		base.Fatalf("go: attempting to write an int to the index that overflows uint32")
	}
	e.Uint32(uint32(n))
}

func (e *encoder) IntAt(n int, at int) {
	if n < 0 || int64(n) > math.MaxUint32 {
		base.Fatalf("go: attempting to write an int to the index that overflows uint32")
	}
	binary.LittleEndian.PutUint32(e.b[at:], uint32(n))
}
