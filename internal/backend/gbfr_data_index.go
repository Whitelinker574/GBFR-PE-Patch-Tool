package backend

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	flatbuffers "github.com/google/flatbuffers/go"
)

type gbfrFileToChunkIndexer struct {
	ChunkEntryIndex             int32
	FileSize                    uint32
	OffsetIntoDecompressedChunk uint32
}

type gbfrDataChunk struct {
	FileOffset       uint64
	Size             uint32
	UncompressedSize uint32
	AllocAlignment   uint32
	UnknownBool      bool
	Padding          byte
	DataFileNumber   byte
	Padding2         byte
}

type gbfrDataIndex struct {
	Codename            string
	NumArchives         uint16
	XXHashSeed          uint16
	ArchiveFileHashes   []uint64
	FileToChunkIndexers []gbfrFileToChunkIndexer
	Chunks              []gbfrDataChunk
	ExternalFileHashes  []uint64
	ExternalFileSizes   []uint64
	CachedChunkIndices  []uint32
}

func parseGBFRDataIndex(data []byte) (result *gbfrDataIndex, err error) {
	if len(data) < 8 {
		return nil, errors.New("data.i is truncated")
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			result = nil
			err = fmt.Errorf("data.i FlatBuffer is invalid: %v", recovered)
		}
	}()
	root := flatbuffers.GetUOffsetT(data)
	if root < 4 || uint64(root)+4 > uint64(len(data)) {
		return nil, errors.New("data.i root offset is outside the file")
	}
	table := flatbuffers.Table{Bytes: data, Pos: root}
	result = &gbfrDataIndex{
		Codename:    string(flatbufferByteVector(table, 4)),
		NumArchives: flatbufferUint16(table, 6),
		XXHashSeed:  flatbufferUint16(table, 8),
	}
	result.ArchiveFileHashes = flatbufferUint64Vector(table, 10)
	result.FileToChunkIndexers = flatbufferFileIndexerVector(table, 12)
	result.Chunks = flatbufferDataChunkVector(table, 14)
	result.ExternalFileHashes = flatbufferUint64Vector(table, 16)
	result.ExternalFileSizes = flatbufferUint64Vector(table, 18)
	result.CachedChunkIndices = flatbufferUint32Vector(table, 20)
	if strings.TrimSpace(result.Codename) == "" {
		return nil, errors.New("data.i codename is empty")
	}
	if len(result.ArchiveFileHashes) != len(result.FileToChunkIndexers) {
		return nil, fmt.Errorf("data.i archive vectors disagree: hashes=%d indexers=%d", len(result.ArchiveFileHashes), len(result.FileToChunkIndexers))
	}
	if len(result.ExternalFileHashes) != len(result.ExternalFileSizes) {
		return nil, fmt.Errorf("data.i external vectors disagree: hashes=%d sizes=%d", len(result.ExternalFileHashes), len(result.ExternalFileSizes))
	}
	if !sort.SliceIsSorted(result.ArchiveFileHashes, func(i, j int) bool { return result.ArchiveFileHashes[i] < result.ArchiveFileHashes[j] }) {
		return nil, errors.New("data.i archive hashes are not sorted")
	}
	if !sort.SliceIsSorted(result.ExternalFileHashes, func(i, j int) bool { return result.ExternalFileHashes[i] < result.ExternalFileHashes[j] }) {
		return nil, errors.New("data.i external hashes are not sorted")
	}
	return result, nil
}

func flatbufferField(table flatbuffers.Table, slot flatbuffers.VOffsetT) flatbuffers.UOffsetT {
	offset := flatbuffers.UOffsetT(table.Offset(slot))
	if offset == 0 {
		return 0
	}
	return offset + table.Pos
}

func flatbufferByteVector(table flatbuffers.Table, slot flatbuffers.VOffsetT) []byte {
	field := flatbufferField(table, slot)
	if field == 0 {
		return nil
	}
	value := table.ByteVector(field)
	return append([]byte(nil), value...)
}

func flatbufferUint16(table flatbuffers.Table, slot flatbuffers.VOffsetT) uint16 {
	field := flatbufferField(table, slot)
	if field == 0 {
		return 0
	}
	return table.GetUint16(field)
}

func flatbufferUint64Vector(table flatbuffers.Table, slot flatbuffers.VOffsetT) []uint64 {
	offset := flatbuffers.UOffsetT(table.Offset(slot))
	if offset == 0 {
		return nil
	}
	start, length := table.Vector(offset), table.VectorLen(offset)
	result := make([]uint64, length)
	for i := range result {
		result[i] = table.GetUint64(start + flatbuffers.UOffsetT(i*8))
	}
	return result
}

func flatbufferUint32Vector(table flatbuffers.Table, slot flatbuffers.VOffsetT) []uint32 {
	offset := flatbuffers.UOffsetT(table.Offset(slot))
	if offset == 0 {
		return nil
	}
	start, length := table.Vector(offset), table.VectorLen(offset)
	result := make([]uint32, length)
	for i := range result {
		result[i] = table.GetUint32(start + flatbuffers.UOffsetT(i*4))
	}
	return result
}

func flatbufferFileIndexerVector(table flatbuffers.Table, slot flatbuffers.VOffsetT) []gbfrFileToChunkIndexer {
	offset := flatbuffers.UOffsetT(table.Offset(slot))
	if offset == 0 {
		return nil
	}
	start, length := table.Vector(offset), table.VectorLen(offset)
	result := make([]gbfrFileToChunkIndexer, length)
	for i := range result {
		position := start + flatbuffers.UOffsetT(i*12)
		result[i] = gbfrFileToChunkIndexer{
			ChunkEntryIndex:             table.GetInt32(position),
			FileSize:                    table.GetUint32(position + 4),
			OffsetIntoDecompressedChunk: table.GetUint32(position + 8),
		}
	}
	return result
}

func flatbufferDataChunkVector(table flatbuffers.Table, slot flatbuffers.VOffsetT) []gbfrDataChunk {
	offset := flatbuffers.UOffsetT(table.Offset(slot))
	if offset == 0 {
		return nil
	}
	start, length := table.Vector(offset), table.VectorLen(offset)
	result := make([]gbfrDataChunk, length)
	for i := range result {
		position := start + flatbuffers.UOffsetT(i*24)
		result[i] = gbfrDataChunk{
			FileOffset:       table.GetUint64(position),
			Size:             table.GetUint32(position + 8),
			UncompressedSize: table.GetUint32(position + 12),
			AllocAlignment:   table.GetUint32(position + 16),
			UnknownBool:      table.GetBool(position + 20),
			Padding:          table.GetByte(position + 21),
			DataFileNumber:   table.GetByte(position + 22),
			Padding2:         table.GetByte(position + 23),
		}
	}
	return result
}

func buildGBFRDataIndex(index *gbfrDataIndex) ([]byte, error) {
	if index == nil {
		return nil, errors.New("data.i model is nil")
	}
	if len(index.ArchiveFileHashes) != len(index.FileToChunkIndexers) || len(index.ExternalFileHashes) != len(index.ExternalFileSizes) {
		return nil, errors.New("data.i vector lengths disagree")
	}
	builder := flatbuffers.NewBuilder(1024)
	codename := builder.CreateString(index.Codename)
	archiveHashes := createUint64Vector(builder, index.ArchiveFileHashes)
	fileIndexers := createFileIndexerVector(builder, index.FileToChunkIndexers)
	chunks := createDataChunkVector(builder, index.Chunks)
	externalHashes := createUint64Vector(builder, index.ExternalFileHashes)
	externalSizes := createUint64Vector(builder, index.ExternalFileSizes)
	cachedIndices := createUint32Vector(builder, index.CachedChunkIndices)
	builder.StartObject(9)
	builder.PrependUOffsetTSlot(0, codename, 0)
	builder.PrependUint16Slot(1, index.NumArchives, 0)
	builder.PrependUint16Slot(2, index.XXHashSeed, 0)
	builder.PrependUOffsetTSlot(3, archiveHashes, 0)
	builder.PrependUOffsetTSlot(4, fileIndexers, 0)
	builder.PrependUOffsetTSlot(5, chunks, 0)
	builder.PrependUOffsetTSlot(6, externalHashes, 0)
	builder.PrependUOffsetTSlot(7, externalSizes, 0)
	builder.PrependUOffsetTSlot(8, cachedIndices, 0)
	root := builder.EndObject()
	builder.Finish(root)
	result := append([]byte(nil), builder.FinishedBytes()...)
	parsed, err := parseGBFRDataIndex(result)
	if err != nil {
		return nil, fmt.Errorf("generated data.i failed validation: %w", err)
	}
	if parsed.Codename != index.Codename || len(parsed.ArchiveFileHashes) != len(index.ArchiveFileHashes) || len(parsed.ExternalFileHashes) != len(index.ExternalFileHashes) {
		return nil, errors.New("generated data.i failed read-back comparison")
	}
	return result, nil
}

func createUint64Vector(builder *flatbuffers.Builder, values []uint64) flatbuffers.UOffsetT {
	builder.StartVector(8, len(values), 8)
	for i := len(values) - 1; i >= 0; i-- {
		builder.PrependUint64(values[i])
	}
	return builder.EndVector(len(values))
}

func createUint32Vector(builder *flatbuffers.Builder, values []uint32) flatbuffers.UOffsetT {
	builder.StartVector(4, len(values), 4)
	for i := len(values) - 1; i >= 0; i-- {
		builder.PrependUint32(values[i])
	}
	return builder.EndVector(len(values))
}

func createFileIndexerVector(builder *flatbuffers.Builder, values []gbfrFileToChunkIndexer) flatbuffers.UOffsetT {
	builder.StartVector(12, len(values), 4)
	for i := len(values) - 1; i >= 0; i-- {
		value := values[i]
		builder.Prep(4, 12)
		builder.PrependUint32(value.OffsetIntoDecompressedChunk)
		builder.PrependUint32(value.FileSize)
		builder.PrependInt32(value.ChunkEntryIndex)
	}
	return builder.EndVector(len(values))
}

func createDataChunkVector(builder *flatbuffers.Builder, values []gbfrDataChunk) flatbuffers.UOffsetT {
	builder.StartVector(24, len(values), 8)
	for i := len(values) - 1; i >= 0; i-- {
		value := values[i]
		builder.Prep(8, 24)
		builder.PrependByte(value.Padding2)
		builder.PrependByte(value.DataFileNumber)
		builder.PrependByte(value.Padding)
		builder.PrependBool(value.UnknownBool)
		builder.PrependUint32(value.AllocAlignment)
		builder.PrependUint32(value.UncompressedSize)
		builder.PrependUint32(value.Size)
		builder.PrependUint64(value.FileOffset)
	}
	return builder.EndVector(len(values))
}

func registerGBFRExternalFile(index *gbfrDataIndex, hash uint64, size uint64) {
	if archiveAt := sort.Search(len(index.ArchiveFileHashes), func(i int) bool { return index.ArchiveFileHashes[i] >= hash }); archiveAt < len(index.ArchiveFileHashes) && index.ArchiveFileHashes[archiveAt] == hash {
		index.ArchiveFileHashes = append(index.ArchiveFileHashes[:archiveAt], index.ArchiveFileHashes[archiveAt+1:]...)
		index.FileToChunkIndexers = append(index.FileToChunkIndexers[:archiveAt], index.FileToChunkIndexers[archiveAt+1:]...)
	}
	externalAt := sort.Search(len(index.ExternalFileHashes), func(i int) bool { return index.ExternalFileHashes[i] >= hash })
	if externalAt < len(index.ExternalFileHashes) && index.ExternalFileHashes[externalAt] == hash {
		index.ExternalFileSizes[externalAt] = size
		return
	}
	index.ExternalFileHashes = append(index.ExternalFileHashes, 0)
	copy(index.ExternalFileHashes[externalAt+1:], index.ExternalFileHashes[externalAt:])
	index.ExternalFileHashes[externalAt] = hash
	index.ExternalFileSizes = append(index.ExternalFileSizes, 0)
	copy(index.ExternalFileSizes[externalAt+1:], index.ExternalFileSizes[externalAt:])
	index.ExternalFileSizes[externalAt] = size
}
