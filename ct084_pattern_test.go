package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestParseCT084Pattern(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		want      ct084Pattern
		wantError bool
	}{
		{
			name: "exact bytes are case insensitive and ignore Unicode whitespace",
			raw:  "Aa\u2003bB\n0c",
			want: ct084Pattern{
				Values: []byte{0xAA, 0xBB, 0x0C},
				Mask:   []byte{0xFF, 0xFF, 0xFF},
			},
		},
		{
			name: "whole byte wildcards accept x and question mark",
			raw:  "xx ?? X? ?x",
			want: ct084Pattern{
				Values: []byte{0x00, 0x00, 0x00, 0x00},
				Mask:   []byte{0x00, 0x00, 0x00, 0x00},
			},
		},
		{
			name: "high and low nibble wildcards",
			raw:  "A? ?b x8 4X",
			want: ct084Pattern{
				Values: []byte{0xA0, 0x0B, 0x08, 0x40},
				Mask:   []byte{0xF0, 0x0F, 0x0F, 0xF0},
			},
		},
		{
			name: "CT084 compact wildcard example",
			raw:  "88xxx8 4? ?F",
			want: ct084Pattern{
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
			got, err := parseCT084Pattern(tt.raw)
			if tt.wantError {
				if err == nil {
					t.Fatalf("parseCT084Pattern(%q) error = nil, want error", tt.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseCT084Pattern(%q) error = %v", tt.raw, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseCT084Pattern(%q) = %#v, want %#v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestMatchCT084Pattern(t *testing.T) {
	pattern := ct084Pattern{
		Values: []byte{0xA0, 0x0B, 0x00, 0x08},
		Mask:   []byte{0xF0, 0x0F, 0x00, 0x0F},
	}
	if !matchCT084Pattern([]byte{0xAF, 0xCB, 0x77, 0xF8}, pattern) {
		t.Fatal("matchCT084Pattern rejected bytes matching all exact nibbles")
	}
	if matchCT084Pattern([]byte{0x9F, 0xCB, 0x77, 0xF8}, pattern) {
		t.Fatal("matchCT084Pattern accepted a mismatched exact nibble")
	}
	if matchCT084Pattern([]byte{0xAF, 0xCB}, pattern) {
		t.Fatal("matchCT084Pattern accepted a buffer with the wrong length")
	}
}

func TestMatchCT084PatternRejectsInvalidPatterns(t *testing.T) {
	tests := []ct084Pattern{
		{},
		{Values: []byte{0xAA}, Mask: nil},
		{Values: []byte{0xAA, 0xBB}, Mask: []byte{0xFF}},
		{Values: []byte{0xAA}, Mask: []byte{0xCC}},
	}
	for _, pattern := range tests {
		if matchCT084Pattern(pattern.Values, pattern) {
			t.Fatalf("matchCT084Pattern accepted invalid pattern %#v", pattern)
		}
	}
}

func TestCT084PatternRejectsNonzeroWildcardBits(t *testing.T) {
	pattern := ct084Pattern{Values: []byte{0x0F}, Mask: []byte{0xF0}}
	if matchCT084Pattern([]byte{0x00}, pattern) {
		t.Fatal("matchCT084Pattern accepted a noncanonical wildcard value")
	}
	if got := findCT084PatternMatches([]byte{0x00}, 0x1000, pattern); got != nil {
		t.Fatalf("findCT084PatternMatches() = %#v, want nil for a noncanonical wildcard value", got)
	}

	readerCalled := false
	reader := func(uintptr, int) ([]byte, error) {
		readerCalled = true
		return []byte{0x00}, nil
	}
	if _, err := scanCT084PatternChunksUnique(0x1000, 1, 1, pattern, "noncanonical", reader); err == nil {
		t.Fatal("scanCT084PatternChunksUnique() error = nil, want validation error")
	}
	if readerCalled {
		t.Fatal("scanCT084PatternChunksUnique() called reader before rejecting a noncanonical wildcard value")
	}
}

func TestFindCT084PatternMatches(t *testing.T) {
	pattern := ct084Pattern{Values: []byte{0xA0, 0x0B}, Mask: []byte{0xF0, 0x0F}}
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
			got := findCT084PatternMatches(tt.buf, 0x1000, pattern)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("findCT084PatternMatches() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestScanCT084PatternChunksUniqueAcross64KiBBoundary(t *testing.T) {
	const (
		base      = uintptr(0x400000)
		chunkSize = uintptr(64 * 1024)
	)
	data := make([]byte, int(chunkSize)+2)
	copy(data[int(chunkSize)-1:], []byte{0xAA, 0xBB, 0xCC})
	pattern := ct084Pattern{Values: []byte{0xAA, 0xBB, 0xCC}, Mask: []byte{0xFF, 0xFF, 0xFF}}

	var sizes []int
	reader := func(addr uintptr, size int) ([]byte, error) {
		sizes = append(sizes, size)
		offset := int(addr - base)
		return append([]byte(nil), data[offset:offset+size]...), nil
	}
	got, err := scanCT084PatternChunksUnique(base, uintptr(len(data)), chunkSize, pattern, "boundary", reader)
	if err != nil {
		t.Fatalf("scanCT084PatternChunksUnique() error = %v", err)
	}
	if want := base + chunkSize - 1; got != want {
		t.Fatalf("scanCT084PatternChunksUnique() = %#x, want %#x", got, want)
	}
	if !reflect.DeepEqual(sizes, []int{int(chunkSize), 2}) {
		t.Fatalf("reader sizes = %v, want [%d 2]", sizes, chunkSize)
	}
}

func TestScanCT084PatternChunksUniqueWhenPatternExceedsChunk(t *testing.T) {
	const base = uintptr(0x5000)
	data := []byte{0x00, 0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0x00}
	pattern := ct084Pattern{
		Values: []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE},
		Mask:   []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
	}
	got, err := scanCT084PatternChunksUnique(base, uintptr(len(data)), 2, pattern, "long", ct084SliceReader(base, data, nil))
	if err != nil {
		t.Fatalf("scanCT084PatternChunksUnique() error = %v", err)
	}
	if want := base + 1; got != want {
		t.Fatalf("scanCT084PatternChunksUnique() = %#x, want %#x", got, want)
	}
}

func TestScanCT084PatternChunksUniqueClearsCarryAfterReadFailureAndContinues(t *testing.T) {
	const base = uintptr(0x6000)
	data := []byte{0x00, 0xAA, 0xFF, 0xFF, 0xBB, 0xCC, 0x00, 0xAA, 0xBB, 0xCC}
	pattern := ct084Pattern{Values: []byte{0xAA, 0xBB, 0xCC}, Mask: []byte{0xFF, 0xFF, 0xFF}}
	failures := map[uintptr]error{base + 2: errors.New("unreadable")}

	got, err := scanCT084PatternChunksUnique(base, uintptr(len(data)), 2, pattern, "hole", ct084SliceReader(base, data, failures))
	if err != nil {
		t.Fatalf("scanCT084PatternChunksUnique() error = %v", err)
	}
	if want := base + 7; got != want {
		t.Fatalf("scanCT084PatternChunksUnique() = %#x, want %#x", got, want)
	}
}

func TestScanCT084PatternChunksUniqueHandlesLengthOneInTailChunk(t *testing.T) {
	const base = uintptr(0x7000)
	data := []byte{0x00, 0x00, 0x00, 0x00, 0xAB}
	pattern := ct084Pattern{Values: []byte{0xAB}, Mask: []byte{0xFF}}

	got, err := scanCT084PatternChunksUnique(base, uintptr(len(data)), 4, pattern, "tail", ct084SliceReader(base, data, nil))
	if err != nil {
		t.Fatalf("scanCT084PatternChunksUnique() error = %v", err)
	}
	if want := base + 4; got != want {
		t.Fatalf("scanCT084PatternChunksUnique() = %#x, want %#x", got, want)
	}
}

func TestScanCT084PatternChunksUniqueReportsZeroAndMultipleMatches(t *testing.T) {
	pattern := ct084Pattern{Values: []byte{0xAB}, Mask: []byte{0xFF}}
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
			const label = "CT084 test signature"
			_, err := scanCT084PatternChunksUnique(0x8000, uintptr(len(tt.data)), 2, pattern, label, ct084SliceReader(0x8000, tt.data, nil))
			if err == nil {
				t.Fatal("scanCT084PatternChunksUnique() error = nil, want error")
			}
			if !strings.Contains(err.Error(), label) || !strings.Contains(strings.ToLower(err.Error()), tt.kind) {
				t.Fatalf("error = %q, want label %q and %q semantics", err, label, tt.kind)
			}
		})
	}
}

func TestScanCT084PatternChunksUniqueRejectsInvalidConfiguration(t *testing.T) {
	reader := func(uintptr, int) ([]byte, error) {
		t.Fatal("reader called for invalid scan configuration")
		return nil, nil
	}
	tests := []struct {
		name      string
		chunkSize uintptr
		pattern   ct084Pattern
	}{
		{name: "zero chunk", chunkSize: 0, pattern: ct084Pattern{Values: []byte{1}, Mask: []byte{0xFF}}},
		{name: "empty mask", chunkSize: 1, pattern: ct084Pattern{}},
		{name: "unequal lengths", chunkSize: 1, pattern: ct084Pattern{Values: []byte{1, 2}, Mask: []byte{0xFF}}},
		{name: "invalid mask", chunkSize: 1, pattern: ct084Pattern{Values: []byte{1}, Mask: []byte{0x33}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := scanCT084PatternChunksUnique(0x9000, 4, tt.chunkSize, tt.pattern, "invalid", reader); err == nil {
				t.Fatal("scanCT084PatternChunksUnique() error = nil, want error")
			}
		})
	}
}

func TestAppScanCT084PatternUniqueReadsRemoteModuleWith64KiBChunks(t *testing.T) {
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
	pattern := ct084Pattern{Values: values, Mask: []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}}
	base := uintptr(unsafe.Pointer(&image[0]))
	app := &App{hProcess: windows.CurrentProcess(), moduleBase: base}

	got, err := app.scanCT084PatternUnique(pattern, "adapter")
	runtime.KeepAlive(image)
	if err != nil {
		t.Fatalf("scanCT084PatternUnique() error = %v", err)
	}
	if want := base + patternStart; got != want {
		t.Fatalf("scanCT084PatternUnique() = %#x, want %#x", got, want)
	}
}

func ct084SliceReader(base uintptr, data []byte, failures map[uintptr]error) ct084ChunkReader {
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
