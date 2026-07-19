package main

import (
	"bytes"
	"crypto/sha256"
	"debug/pe"
	"fmt"
	"io"
	"os"
	"testing"
)

const (
	ct084LocalGame202SHA256 = "63340832BCF731FBC97796F686B05C988418E83D451D4A49B2244A85D00E297F"
	ct084LocalGame202Size   = int64(123522016)
)

type ct084LocalExecutableSection struct {
	name string
	rva  uint32
	data []byte
}

type ct084LocalPatternMatch struct {
	section string
	rva     uint32
}

func readCT084LocalExecutableSections(path string) ([]ct084LocalExecutableSection, error) {
	image, err := pe.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open PE image: %w", err)
	}
	defer image.Close()

	sections := make([]ct084LocalExecutableSection, 0, len(image.Sections))
	for _, section := range image.Sections {
		if section.Characteristics&pe.IMAGE_SCN_MEM_EXECUTE == 0 {
			continue
		}
		data, err := section.Data()
		if err != nil {
			return nil, fmt.Errorf("read executable PE section %q: %w", section.Name, err)
		}
		limit := uint64(len(data))
		if virtualSize := uint64(section.VirtualSize); virtualSize < limit {
			limit = virtualSize
		}
		sections = append(sections, ct084LocalExecutableSection{
			name: section.Name,
			rva:  section.VirtualAddress,
			data: data[:int(limit)],
		})
	}
	if len(sections) == 0 {
		return nil, fmt.Errorf("PE image has no executable sections")
	}
	return sections, nil
}

func findCT084LocalPatternMatches(sections []ct084LocalExecutableSection, pattern ct084Pattern) []ct084LocalPatternMatch {
	if validateCT084Pattern(pattern) != nil {
		return nil
	}

	// Most CT signatures contain a long exact run. Using its longest run as an
	// anchor keeps this opt-in truth test fast without changing match semantics.
	anchorStart, anchorLength := 0, 0
	for start := 0; start < len(pattern.Mask); {
		if pattern.Mask[start] != 0xff {
			start++
			continue
		}
		end := start + 1
		for end < len(pattern.Mask) && pattern.Mask[end] == 0xff {
			end++
		}
		if end-start > anchorLength {
			anchorStart, anchorLength = start, end-start
		}
		start = end
	}

	var matches []ct084LocalPatternMatch
	for _, section := range sections {
		if len(section.data) < len(pattern.Values) {
			continue
		}
		if anchorLength == 0 {
			for _, address := range findCT084PatternMatches(section.data, uintptr(section.rva), pattern) {
				matches = append(matches, ct084LocalPatternMatch{section: section.name, rva: uint32(address)})
			}
			continue
		}

		anchor := pattern.Values[anchorStart : anchorStart+anchorLength]
		searchFrom := 0
		for searchFrom <= len(section.data)-anchorLength {
			relative := bytes.Index(section.data[searchFrom:], anchor)
			if relative < 0 {
				break
			}
			anchorOffset := searchFrom + relative
			candidateOffset := anchorOffset - anchorStart
			if candidateOffset >= 0 && candidateOffset+len(pattern.Values) <= len(section.data) &&
				matchCT084PatternValidated(section.data[candidateOffset:], pattern) {
				matches = append(matches, ct084LocalPatternMatch{
					section: section.name,
					rva:     section.rva + uint32(candidateOffset),
				})
			}
			searchFrom = anchorOffset + 1
		}
	}
	return matches
}

func verifyCT084LocalGameIdentity(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return err
	}
	if stat.Size() != ct084LocalGame202Size {
		return fmt.Errorf("game executable size=%d, want %d", stat.Size(), ct084LocalGame202Size)
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return err
	}
	if got := fmt.Sprintf("%X", hasher.Sum(nil)); got != ct084LocalGame202SHA256 {
		return fmt.Errorf("game executable SHA256=%s, want %s", got, ct084LocalGame202SHA256)
	}
	return nil
}

func TestCT084CatalogMatchesLocalGame202(t *testing.T) {
	path := os.Getenv("GBFR_GAME_EXE_TEST")
	if path == "" {
		t.Skip("set GBFR_GAME_EXE_TEST to verify the locally supplied game 2.0.2 executable")
	}
	if err := verifyCT084LocalGameIdentity(path); err != nil {
		t.Fatalf("verify local game 2.0.2 identity: %v", err)
	}
	sections, err := readCT084LocalExecutableSections(path)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := loadCT084Catalog()
	if err != nil {
		t.Fatal(err)
	}

	matchCache := make(map[string][]ct084LocalPatternMatch)
	siteCount := 0
	for _, feature := range catalog.Features {
		for siteIndex, site := range feature.Sites {
			siteCount++
			pattern, err := parseCT084Pattern(site.AOB)
			if err != nil {
				t.Errorf("feature %s / CT ID %d / site index %d / symbol %s: parse AOB: %v", feature.ID, feature.CTID, siteIndex, site.Symbol, err)
				continue
			}
			canonical := canonicalCT084AOB(pattern)
			matches, cached := matchCache[canonical]
			if !cached {
				matches = findCT084LocalPatternMatches(sections, pattern)
				matchCache[canonical] = matches
			}
			if len(matches) != 1 {
				t.Errorf("feature %s / CT ID %d / site index %d / symbol %s: count=%d, want 1", feature.ID, feature.CTID, siteIndex, site.Symbol, len(matches))
			}
		}
	}
	t.Logf("verified %d CT 0.8.4 sites across %d canonical AOBs in %d executable PE sections", siteCount, len(matchCache), len(sections))
}
