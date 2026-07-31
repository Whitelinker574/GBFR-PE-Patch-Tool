package backend

import "encoding/binary"

const (
	gbfrPrime32_1 uint32 = 0x9E3779B1
	gbfrPrime32_2 uint32 = 0x85EBCA77
	gbfrPrime32_3 uint32 = 0xC2B2AE3D
	gbfrPrime32_4 uint32 = 0x27D4EB2F
	gbfrPrime32_5 uint32 = 0x165667B1
)

func rotateLeft32(value uint32, bits uint) uint32 {
	return value<<bits | value>>(32-bits)
}

func gbfrHashRound(seed, input uint32) uint32 {
	return rotateLeft32(seed+input*gbfrPrime32_2, 13) * gbfrPrime32_1
}

// gbfrHash32 computes the game's text-ID hash. It is distinct from the
// XXHash64 checksums that protect save-file sections.
func gbfrHash32(text string) uint32 {
	data := []byte(text)
	length := len(data)
	offset := 0
	result := uint32(0x178A54A4)
	if length >= 16 {
		v1, v2 := uint32(0x2557311B), uint32(0x871FB76A)
		v3, v4 := uint32(0x0133ECF3), uint32(0x62FC7342)
		for {
			v1 = gbfrHashRound(v1, binary.LittleEndian.Uint32(data[offset:]))
			offset += 4
			v2 = gbfrHashRound(v2, binary.LittleEndian.Uint32(data[offset:]))
			offset += 4
			v3 = gbfrHashRound(v3, binary.LittleEndian.Uint32(data[offset:]))
			offset += 4
			v4 = gbfrHashRound(v4, binary.LittleEndian.Uint32(data[offset:]))
			offset += 4
			if length-offset <= 16 {
				break
			}
		}
		result = rotateLeft32(v1, 1) + rotateLeft32(v2, 7) + rotateLeft32(v3, 12) + rotateLeft32(v4, 18)
	}
	result += uint32(length)
	for length-offset >= 4 {
		result = rotateLeft32(result+binary.LittleEndian.Uint32(data[offset:])*gbfrPrime32_3, 17) * gbfrPrime32_4
		offset += 4
	}
	for offset < length {
		result = rotateLeft32(result+uint32(data[offset])*gbfrPrime32_5, 11) * gbfrPrime32_1
		offset++
	}
	result ^= result >> 15
	result *= gbfrPrime32_2
	result ^= result >> 13
	result *= gbfrPrime32_3
	result ^= result >> 16
	return result
}
