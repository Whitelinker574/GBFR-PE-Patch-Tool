package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestParseRuntimePatchPattern(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		want      runtimePatchPattern
		wantError bool
	}{
		{
			name: "exact bytes are case insensitive and ignore Unicode whitespace",
			raw:  "Aa\u2003bB\n0c",
			want: runtimePatchPattern{
				Values: []byte{0xAA, 0xBB, 0x0C},
				Mask:   []byte{0xFF, 0xFF, 0xFF},
			},
		},
		{
			name: "whole byte wildcards accept x and question mark",
			raw:  "xx ?? X? ?x",
			want: runtimePatchPattern{
				Values: []byte{0x00, 0x00, 0x00, 0x00},
				Mask:   []byte{0x00, 0x00, 0x00, 0x00},
			},
		},
		{
			name: "high and low nibble wildcards",
			raw:  "A? ?b x8 4X",
			want: runtimePatchPattern{
				Values: []byte{0xA0, 0x0B, 0x08, 0x40},
				Mask:   []byte{0xF0, 0x0F, 0x0F, 0xF0},
			},
		},
		{
			name: "RuntimePatch compact wildcard example",
			raw:  "88xxx8 4? ?F",
			want: runtimePatchPattern{
				Values: []byte{0x88, 0x00, 0x08, 0x40, 0x0F},
				Mask:   []byte{0xFF, 0x00, 0x0F, 0xF0, 0x0F},
			},
		},
		{name: "empty", raw: " \t\u2003", wantError: true},
		{name: "odd nibble count", raw: "AA B", wantError: true},
		{name: "invalid character", raw: "AA G0", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRuntimePatchPattern(tt.raw)
			if tt.wantError {
				if err == nil {
					t.Fatalf("parseRuntimePatchPattern(%q) error = nil, want error", tt.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseRuntimePatchPattern(%q) error = %v", tt.raw, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseRuntimePatchPattern(%q) = %#v, want %#v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestMatchRuntimePatchPattern(t *testing.T) {
	pattern := runtimePatchPattern{
		Values: []byte{0xA0, 0x0B, 0x00, 0x08},
		Mask:   []byte{0xF0, 0x0F, 0x00, 0x0F},
	}
	if !matchRuntimePatchPattern([]byte{0xAF, 0xCB, 0x77, 0xF8}, pattern) {
		t.Fatal("matchRuntimePatchPattern rejected bytes matching all exact nibbles")
	}
	if matchRuntimePatchPattern([]byte{0x9F, 0xCB, 0x77, 0xF8}, pattern) {
		t.Fatal("matchRuntimePatchPattern accepted a mismatched exact nibble")
	}
	if matchRuntimePatchPattern([]byte{0xAF, 0xCB}, pattern) {
		t.Fatal("matchRuntimePatchPattern accepted a buffer with the wrong length")
	}
}

func TestMatchRuntimePatchPatternRejectsInvalidPatterns(t *testing.T) {
	tests := []runtimePatchPattern{
		{},
		{Values: []byte{0xAA}, Mask: nil},
		{Values: []byte{0xAA, 0xBB}, Mask: []byte{0xFF}},
		{Values: []byte{0xAA}, Mask: []byte{0xCC}},
	}
	for _, pattern := range tests {
		if matchRuntimePatchPattern(pattern.Values, pattern) {
			t.Fatalf("matchRuntimePatchPattern accepted invalid pattern %#v", pattern)
		}
	}
}

func TestRuntimePatchPatternRejectsNonzeroWildcardBits(t *testing.T) {
	pattern := runtimePatchPattern{Values: []byte{0x0F}, Mask: []byte{0xF0}}
	if matchRuntimePatchPattern([]byte{0x00}, pattern) {
		t.Fatal("matchRuntimePatchPattern accepted a noncanonical wildcard value")
	}
	if got := findRuntimePatchPatternMatches([]byte{0x00}, 0x1000, pattern); got != nil {
		t.Fatalf("findRuntimePatchPatternMatches() = %#v, want nil for a noncanonical wildcard value", got)
	}

	readerCalled := false
	reader := func(uintptr, int) ([]byte, error) {
		readerCalled = true
		return []byte{0x00}, nil
	}
	if _, err := scanRuntimePatchPatternChunksUnique(0x1000, 1, 1, pattern, "noncanonical", reader); err == nil {
		t.Fatal("scanRuntimePatchPatternChunksUnique() error = nil, want validation error")
	}
	if readerCalled {
		t.Fatal("scanRuntimePatchPatternChunksUnique() called reader before rejecting a noncanonical wildcard value")
	}
}

func TestFindRuntimePatchPatternMatches(t *testing.T) {
	pattern := runtimePatchPattern{Values: []byte{0xA0, 0x0B}, Mask: []byte{0xF0, 0x0F}}
	tests := []struct {
		name string
		buf  []byte
		want []uintptr
	}{
		{name: "zero", buf: []byte{0xA1, 0xC2}, want: nil},
		{name: "one", buf: []byte{0x00, 0xAF, 0xCB, 0x00}, want: []uintptr{0x1001}},
		{name: "multiple", buf: []byte{0xAF, 0x0B, 0xA1, 0xCB}, want: []uintptr{0x1000, 0x1002}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findRuntimePatchPatternMatches(tt.buf, 0x1000, pattern)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("findRuntimePatchPatternMatches() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestScanRuntimePatchPatternChunksUniqueAcross64KiBBoundary(t *testing.T) {
	const (
		base      = uintptr(0x400000)
		chunkSize = uintptr(64 * 1024)
	)
	data := make([]byte, int(chunkSize)+2)
	copy(data[int(chunkSize)-1:], []byte{0xAA, 0xBB, 0xCC})
	pattern := runtimePatchPattern{Values: []byte{0xAA, 0xBB, 0xCC}, Mask: []byte{0xFF, 0xFF, 0xFF}}

	var sizes []int
	reader := func(addr uintptr, size int) ([]byte, error) {
		sizes = append(sizes, size)
		offset := int(addr - base)
		return append([]byte(nil), data[offset:offset+size]...), nil
	}
	got, err := scanRuntimePatchPatternChunksUnique(base, uintptr(len(data)), chunkSize, pattern, "boundary", reader)
	if err != nil {
		t.Fatalf("scanRuntimePatchPatternChunksUnique() error = %v", err)
	}
	if want := base + chunkSize - 1; got != want {
		t.Fatalf("scanRuntimePatchPatternChunksUnique() = %#x, want %#x", got, want)
	}
	if !reflect.DeepEqual(sizes, []int{int(chunkSize), 2}) {
		t.Fatalf("reader sizes = %v, want [%d 2]", sizes, chunkSize)
	}
}

func TestScanRuntimePatchPatternChunksUniqueWhenPatternExceedsChunk(t *testing.T) {
	const base = uintptr(0x5000)
	data := []byte{0x00, 0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0x00}
	pattern := runtimePatchPattern{
		Values: []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE},
		Mask:   []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
	}
	got, err := scanRuntimePatchPatternChunksUnique(base, uintptr(len(data)), 2, pattern, "long", runtimePatchSliceReader(base, data, nil))
	if err != nil {
		t.Fatalf("scanRuntimePatchPatternChunksUnique() error = %v", err)
	}
	if want := base + 1; got != want {
		t.Fatalf("scanRuntimePatchPatternChunksUnique() = %#x, want %#x", got, want)
	}
}

func TestScanRuntimePatchPatternChunksUniqueClearsCarryAfterReadFailureAndContinues(t *testing.T) {
	const base = uintptr(0x6000)
	data := []byte{0x00, 0xAA, 0xFF, 0xFF, 0xBB, 0xCC, 0x00, 0xAA, 0xBB, 0xCC}
	pattern := runtimePatchPattern{Values: []byte{0xAA, 0xBB, 0xCC}, Mask: []byte{0xFF, 0xFF, 0xFF}}
	failures := map[uintptr]error{base + 2: errors.New("unreadable")}

	got, err := scanRuntimePatchPatternChunksUnique(base, uintptr(len(data)), 2, pattern, "hole", runtimePatchSliceReader(base, data, failures))
	if err != nil {
		t.Fatalf("scanRuntimePatchPatternChunksUnique() error = %v", err)
	}
	if want := base + 7; got != want {
		t.Fatalf("scanRuntimePatchPatternChunksUnique() = %#x, want %#x", got, want)
	}
}

func TestScanRuntimePatchPatternChunksUniqueHandlesLengthOneInTailChunk(t *testing.T) {
	const base = uintptr(0x7000)
	data := []byte{0x00, 0x00, 0x00, 0x00, 0xAB}
	pattern := runtimePatchPattern{Values: []byte{0xAB}, Mask: []byte{0xFF}}

	got, err := scanRuntimePatchPatternChunksUnique(base, uintptr(len(data)), 4, pattern, "tail", runtimePatchSliceReader(base, data, nil))
	if err != nil {
		t.Fatalf("scanRuntimePatchPatternChunksUnique() error = %v", err)
	}
	if want := base + 4; got != want {
		t.Fatalf("scanRuntimePatchPatternChunksUnique() = %#x, want %#x", got, want)
	}
}

func TestScanRuntimePatchPatternChunksUniqueReportsZeroAndMultipleMatches(t *testing.T) {
	pattern := runtimePatchPattern{Values: []byte{0xAB}, Mask: []byte{0xFF}}
	tests := []struct {
		name string
		data []byte
		kind string
	}{
		{name: "zero", data: []byte{0x00, 0x01}, kind: "zero"},
		{name: "multiple", data: []byte{0xAB, 0x00, 0xAB}, kind: "multiple"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const label = "RuntimePatch test signature"
			_, err := scanRuntimePatchPatternChunksUnique(0x8000, uintptr(len(tt.data)), 2, pattern, label, runtimePatchSliceReader(0x8000, tt.data, nil))
			if err == nil {
				t.Fatal("scanRuntimePatchPatternChunksUnique() error = nil, want error")
			}
			if !strings.Contains(err.Error(), label) || !strings.Contains(strings.ToLower(err.Error()), tt.kind) {
				t.Fatalf("error = %q, want label %q and %q semantics", err, label, tt.kind)
			}
		})
	}
}

func TestScanRuntimePatchPatternChunksUniqueStreamsEarlyWildcardMatches(t *testing.T) {
	const chunkSize = 64 * 1024
	chunk := make([]byte, chunkSize)
	pattern := runtimePatchPattern{Values: []byte{0x00}, Mask: []byte{0x00}}
	readCalls := 0
	reader := func(uintptr, int) ([]byte, error) {
		readCalls++
		return chunk, nil
	}

	_, err := scanRuntimePatchPatternChunksUnique(0xA000, chunkSize*2, chunkSize, pattern, "wildcard", reader)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "multiple") {
		t.Fatalf("scanRuntimePatchPatternChunksUnique() error = %v, want multiple-match error", err)
	}
	if readCalls != 1 {
		t.Fatalf("reader calls = %d, want 1 because the first chunk contains two early matches", readCalls)
	}

	allocationReader := func(uintptr, int) ([]byte, error) { return chunk, nil }
	allocs := testing.AllocsPerRun(10, func() {
		if _, err := scanRuntimePatchPatternChunksUnique(0xA000, chunkSize*2, chunkSize, pattern, "wildcard", allocationReader); err == nil {
			t.Fatal("scanRuntimePatchPatternChunksUnique() error = nil, want multiple-match error")
		}
	})
	if allocs > 8 {
		t.Fatalf("allocations per streaming uniqueness scan = %.1f, want at most 8", allocs)
	}
}

func TestRuntimePatchPatternSearchHotLoopsUseValidatedMatcher(t *testing.T) {
	bodies := runtimePatchFunctionBodies(t)
	findBody := bodies["findRuntimePatchPatternMatches"]
	if findBody == nil {
		t.Fatal("missing findRuntimePatchPatternMatches")
	}
	if got := runtimePatchCountCallsIdent(findBody, "validateRuntimePatchPattern"); got != 1 {
		t.Fatalf("findRuntimePatchPatternMatches calls validateRuntimePatchPattern %d times syntactically, want 1", got)
	}
	if got := runtimePatchCountCallsIdent(findBody, "matchRuntimePatchPattern"); got != 0 {
		t.Fatalf("findRuntimePatchPatternMatches calls validating matchRuntimePatchPattern %d times, want 0 in its hot loop", got)
	}

	scanBody := bodies["scanRuntimePatchPatternChunksUnique"]
	if scanBody == nil {
		t.Fatal("missing scanRuntimePatchPatternChunksUnique")
	}
	if got := runtimePatchCountCallsIdent(scanBody, "validateRuntimePatchPattern"); got != 1 {
		t.Fatalf("scanRuntimePatchPatternChunksUnique calls validateRuntimePatchPattern %d times syntactically, want 1", got)
	}
	if got := runtimePatchCountCallsIdent(scanBody, "findRuntimePatchPatternMatches"); got != 0 {
		t.Fatalf("scanRuntimePatchPatternChunksUnique materializes findRuntimePatchPatternMatches %d times, want streaming matching", got)
	}
	if got := runtimePatchCountCallsIdent(scanBody, "matchRuntimePatchPattern"); got != 0 {
		t.Fatalf("scanRuntimePatchPatternChunksUnique calls validating matchRuntimePatchPattern %d times, want 0 in its hot loop", got)
	}
}

func TestScanRuntimePatchPatternChunksUniqueRejectsInvalidConfiguration(t *testing.T) {
	reader := func(uintptr, int) ([]byte, error) {
		t.Fatal("reader called for invalid scan configuration")
		return nil, nil
	}
	tests := []struct {
		name      string
		chunkSize uintptr
		pattern   runtimePatchPattern
	}{
		{name: "zero chunk", chunkSize: 0, pattern: runtimePatchPattern{Values: []byte{1}, Mask: []byte{0xFF}}},
		{name: "empty mask", chunkSize: 1, pattern: runtimePatchPattern{}},
		{name: "unequal lengths", chunkSize: 1, pattern: runtimePatchPattern{Values: []byte{1, 2}, Mask: []byte{0xFF}}},
		{name: "invalid mask", chunkSize: 1, pattern: runtimePatchPattern{Values: []byte{1}, Mask: []byte{0x33}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := scanRuntimePatchPatternChunksUnique(0x9000, 4, tt.chunkSize, tt.pattern, "invalid", reader); err == nil {
				t.Fatal("scanRuntimePatchPatternChunksUnique() error = nil, want error")
			}
		})
	}
}

func TestAppScanRuntimePatchPatternUniqueReadsRemoteModuleWith64KiBChunks(t *testing.T) {
	const (
		chunkSize    = 64 * 1024
		patternStart = chunkSize - 3
	)
	image := make([]byte, chunkSize+16)
	image[0], image[1] = 'M', 'Z'
	binary.LittleEndian.PutUint32(image[0x3C:0x40], 0x80)
	copy(image[0x80:0x84], []byte{'P', 'E', 0, 0})
	binary.LittleEndian.PutUint32(image[0xD0:0xD4], uint32(len(image)))
	values := []byte{0xDE, 0xAD, 0xBE, 0xEF, 0x13, 0x37, 0xC0, 0x42}
	copy(image[patternStart:], values)
	pattern := runtimePatchPattern{Values: values, Mask: []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}}
	base := uintptr(unsafe.Pointer(&image[0]))
	app := &App{hProcess: windows.CurrentProcess(), moduleBase: base}

	got, err := app.scanRuntimePatchPatternUnique(pattern, "adapter")
	runtime.KeepAlive(image)
	if err != nil {
		t.Fatalf("scanRuntimePatchPatternUnique() error = %v", err)
	}
	if want := base + patternStart; got != want {
		t.Fatalf("scanRuntimePatchPatternUnique() = %#x, want %#x", got, want)
	}
}

func runtimePatchSliceReader(base uintptr, data []byte, failures map[uintptr]error) runtimePatchChunkReader {
	return func(addr uintptr, size int) ([]byte, error) {
		if err := failures[addr]; err != nil {
			return nil, err
		}
		if addr < base || addr-base > uintptr(len(data)) || size < 0 || uintptr(size) > uintptr(len(data))-(addr-base) {
			return nil, fmt.Errorf("read outside test data: address=%#x size=%d", addr, size)
		}
		offset := int(addr - base)
		return append([]byte(nil), data[offset:offset+size]...), nil
	}
}

func runtimePatchFunctionBodies(t *testing.T) map[string]*ast.BlockStmt {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), "runtime_patch_pattern.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	bodies := make(map[string]*ast.BlockStmt)
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if ok && function.Body != nil {
			bodies[function.Name.Name] = function.Body
		}
	}
	return bodies
}

func runtimePatchCountCallsIdent(body *ast.BlockStmt, name string) int {
	count := 0
	ast.Inspect(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		identifier, ok := call.Fun.(*ast.Ident)
		if ok && identifier.Name == name {
			count++
		}
		return true
	})
	return count
}
