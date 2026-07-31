package backend

import (
	"os"
	"reflect"
	"testing"
)

func TestGBFRDataIndexRoundTrip(t *testing.T) {
	want := &gbfrDataIndex{
		Codename:          "relink",
		NumArchives:       7,
		XXHashSeed:        19,
		ArchiveFileHashes: []uint64{10, 30},
		FileToChunkIndexers: []gbfrFileToChunkIndexer{
			{ChunkEntryIndex: 2, FileSize: 100, OffsetIntoDecompressedChunk: 3},
			{ChunkEntryIndex: 5, FileSize: 200, OffsetIntoDecompressedChunk: 7},
		},
		Chunks:             []gbfrDataChunk{{FileOffset: 90, Size: 80, UncompressedSize: 120, AllocAlignment: 16, UnknownBool: true, Padding: 1, DataFileNumber: 4, Padding2: 2}},
		ExternalFileHashes: []uint64{40, 50},
		ExternalFileSizes:  []uint64{400, 500},
		CachedChunkIndices: []uint32{1, 4, 8},
	}
	data, err := buildGBFRDataIndex(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := parseGBFRDataIndex(data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestGBFRDataIndexRegistersExternalFileWithoutBreakingPairs(t *testing.T) {
	index := &gbfrDataIndex{
		Codename:            "relink",
		ArchiveFileHashes:   []uint64{10, 20, 30},
		FileToChunkIndexers: []gbfrFileToChunkIndexer{{ChunkEntryIndex: 1}, {ChunkEntryIndex: 2}, {ChunkEntryIndex: 3}},
		ExternalFileHashes:  []uint64{5, 25},
		ExternalFileSizes:   []uint64{50, 250},
	}
	registerGBFRExternalFile(index, 20, 200)
	registerGBFRExternalFile(index, 25, 251)
	if !reflect.DeepEqual(index.ArchiveFileHashes, []uint64{10, 30}) || index.FileToChunkIndexers[1].ChunkEntryIndex != 3 {
		t.Fatalf("archive vectors lost alignment: %+v %+v", index.ArchiveFileHashes, index.FileToChunkIndexers)
	}
	if !reflect.DeepEqual(index.ExternalFileHashes, []uint64{5, 20, 25}) || !reflect.DeepEqual(index.ExternalFileSizes, []uint64{50, 200, 251}) {
		t.Fatalf("external vectors lost alignment: %+v %+v", index.ExternalFileHashes, index.ExternalFileSizes)
	}
}

func TestGBFRDataIndexRoundTripsLocalGameIndex(t *testing.T) {
	path := os.Getenv("GBFR_DATA_INDEX_TEST")
	if path == "" {
		t.Skip("set GBFR_DATA_INDEX_TEST to verify a local game data.i")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	original, err := parseGBFRDataIndex(data)
	if err != nil {
		t.Fatal(err)
	}
	rebuilt, err := buildGBFRDataIndex(original)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parseGBFRDataIndex(rebuilt)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(parsed, original) {
		t.Fatal("real data.i changed semantically after round trip")
	}
	t.Logf("round-tripped data.i: archive=%d chunks=%d external=%d bytes=%d->%d", len(original.ArchiveFileHashes), len(original.Chunks), len(original.ExternalFileHashes), len(data), len(rebuilt))
}
