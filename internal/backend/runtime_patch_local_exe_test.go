package backend

import (
	"bytes"
	"crypto/sha256"
	"debug/pe"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

const (
	runtimePatchLocalGame202SHA256 = "63340832BCF731FBC97796F686B05C988418E83D451D4A49B2244A85D00E297F"
	runtimePatchLocalGame202Size   = int64(123522016)
	runtimePatchLocalGame203SHA256 = "1BBBEC61AAB7F75FE328CF6BFE0247EBDBCEC6C404CEC12C032B8FFA41D22102"
	runtimePatchLocalGame203Size   = int64(123506656)
)

type runtimePatchLocalExecutableSection struct {
	name string
	rva  uint32
	data []byte
}

type runtimePatchLocalPatternMatch struct {
	section string
	rva     uint32
}

func readRuntimePatchLocalExecutableSections(path string) ([]runtimePatchLocalExecutableSection, error) {
	image, err := pe.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open PE image: %w", err)
	}
	defer image.Close()

	sections := make([]runtimePatchLocalExecutableSection, 0, len(image.Sections))
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
		sections = append(sections, runtimePatchLocalExecutableSection{
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

func findRuntimePatchLocalPatternMatches(sections []runtimePatchLocalExecutableSection, pattern runtimePatchPattern) []runtimePatchLocalPatternMatch {
	if validateRuntimePatchPattern(pattern) != nil {
		return nil
	}

	// Most runtime patch signatures contain a long exact run. Using its longest run as an
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

	var matches []runtimePatchLocalPatternMatch
	for _, section := range sections {
		if len(section.data) < len(pattern.Values) {
			continue
		}
		if anchorLength == 0 {
			for _, address := range findRuntimePatchPatternMatches(section.data, uintptr(section.rva), pattern) {
				matches = append(matches, runtimePatchLocalPatternMatch{section: section.name, rva: uint32(address)})
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
				matchRuntimePatchPatternValidated(section.data[candidateOffset:], pattern) {
				matches = append(matches, runtimePatchLocalPatternMatch{
					section: section.name,
					rva:     section.rva + uint32(candidateOffset),
				})
			}
			searchFrom = anchorOffset + 1
		}
	}
	return matches
}

func verifyRuntimePatchLocalGameIdentity(path string) error {
	return verifyRuntimePatchLocalGameIdentityExact(path, runtimePatchLocalGame202Size, runtimePatchLocalGame202SHA256)
}

func verifyRuntimePatchLocalGameIdentityExact(path string, expectedSize int64, expectedSHA256 string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return err
	}
	if stat.Size() != expectedSize {
		return fmt.Errorf("game executable size=%d, want %d", stat.Size(), expectedSize)
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return err
	}
	if got := fmt.Sprintf("%X", hasher.Sum(nil)); !strings.EqualFold(got, expectedSHA256) {
		return fmt.Errorf("game executable SHA256=%s, want %s", got, expectedSHA256)
	}
	return nil
}

func TestRuntimePatchCatalogMatchesLocalGame203(t *testing.T) {
	path := os.Getenv("GBFR_GAME_EXE_203_TEST")
	if path == "" {
		t.Skip("set GBFR_GAME_EXE_203_TEST to verify the locally supplied game 2.0.3 executable")
	}
	if err := verifyRuntimePatchLocalGameIdentityExact(path, runtimePatchLocalGame203Size, runtimePatchLocalGame203SHA256); err != nil {
		t.Fatalf("verify local game 2.0.3 identity: %v", err)
	}
	sections, err := readRuntimePatchLocalExecutableSections(path)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := decodeRuntimePatchCatalog(runtimePatchCatalogJSON)
	if err != nil {
		t.Fatal(err)
	}

	unchanged := 0
	compatibleVariants := 0
	missing := make([]string, 0, 2)
	for _, feature := range catalog.Features {
		for siteIndex, catalogSite := range feature.Sites {
			site, err := runtimePatchSiteForExecutable(catalogSite, runtimePatchLocalGame203SHA256)
			if err != nil {
				t.Fatalf("%s site[%d]: resolve 2.0.3 definition: %v", feature.ID, siteIndex, err)
			}
			pattern, err := parseRuntimePatchPattern(site.AOB)
			if err != nil {
				t.Fatalf("%s site[%d]: parse AOB: %v", feature.ID, siteIndex, err)
			}
			matches := findRuntimePatchLocalPatternMatches(sections, pattern)
			if len(matches) == 0 {
				missing = append(missing, fmt.Sprintf("%d/%d", feature.CatalogID, siteIndex))
				continue
			}
			if len(matches) != 1 {
				t.Errorf("%s site[%d]: matches=%d, want 1", feature.ID, siteIndex, len(matches))
				continue
			}
			match := matches[0]
			for sectionIndex := range sections {
				section := &sections[sectionIndex]
				start := int(match.rva-section.rva) + site.Offset
				if section.name != match.section || start < 0 || start+len(site.EnableBytes) > len(section.data) {
					continue
				}
				actual := section.data[start : start+len(site.EnableBytes)]
				expected := site.ExpectedOriginalBytes
				if !bytes.Equal(actual, expected) {
					t.Errorf("%s site[%d] 2.0.3 original=% X, expected=% X", feature.ID, siteIndex, actual, expected)
				} else if bytes.Equal(actual, catalogSite.ExpectedOriginalBytes) {
					unchanged++
				} else {
					t.Logf("%s site[%d] 2.0.3 original=% X", feature.ID, siteIndex, actual)
					compatibleVariants++
				}
				break
			}
		}
	}
	if unchanged != 79 || compatibleVariants != 3 {
		t.Errorf("2.0.3 RuntimePatch coverage=%d unchanged/%d variants, want 79/3", unchanged, compatibleVariants)
	}
	if got, want := strings.Join(missing, ","), ""; got != want {
		t.Errorf("2.0.3 missing sites=%q, want %q", got, want)
	}
}

func formatRuntimePatchLocalMatchLocations(matches []runtimePatchLocalPatternMatch) string {
	if len(matches) == 0 {
		return "none in executable sections"
	}
	const locationLimit = 4
	shown := len(matches)
	if shown > locationLimit {
		shown = locationLimit
	}
	locations := make([]string, shown)
	for index := 0; index < shown; index++ {
		locations[index] = fmt.Sprintf("%s@RVA 0x%X", matches[index].section, matches[index].rva)
	}
	result := strings.Join(locations, ", ")
	if remaining := len(matches) - shown; remaining > 0 {
		result += fmt.Sprintf(" (+%d more)", remaining)
	}
	return result
}

func TestFormatRuntimePatchLocalMatchLocationsIsBoundedAndExplicit(t *testing.T) {
	if got := formatRuntimePatchLocalMatchLocations(nil); got != "none in executable sections" {
		t.Fatalf("zero-match locations=%q, want an explicit executable-section result", got)
	}
	matches := []runtimePatchLocalPatternMatch{
		{section: ".text", rva: 0x1000},
		{section: ".text", rva: 0x1010},
		{section: ".bind", rva: 0x81df020},
		{section: ".text", rva: 0x1030},
		{section: ".text", rva: 0x1040},
	}
	want := ".text@RVA 0x1000, .text@RVA 0x1010, .bind@RVA 0x81DF020, .text@RVA 0x1030 (+1 more)"
	if got := formatRuntimePatchLocalMatchLocations(matches); got != want {
		t.Fatalf("locations=%q, want %q", got, want)
	}
}

func TestRuntimePatchCatalogMatchesLocalGame202(t *testing.T) {
	path := os.Getenv("GBFR_GAME_EXE_TEST")
	if path == "" {
		t.Skip("set GBFR_GAME_EXE_TEST to verify the locally supplied game 2.0.2 executable")
	}
	if err := verifyRuntimePatchLocalGameIdentity(path); err != nil {
		t.Fatalf("verify local game 2.0.2 identity: %v", err)
	}
	sections, err := readRuntimePatchLocalExecutableSections(path)
	if err != nil {
		t.Fatal(err)
	}
	catalog := readRuntimePatchCatalogFile(t)
	evidence := readRuntimePatchOriginalEvidence(t)
	evidenceByKey := make(map[[2]int]runtimePatchOriginalEvidenceSite, len(evidence.Sites))
	for _, site := range evidence.Sites {
		evidenceByKey[[2]int{site.CatalogID, site.SiteIndex}] = site
	}

	matchCache := make(map[string][]runtimePatchLocalPatternMatch)
	siteCount := 0
	for _, feature := range catalog.Features {
		for siteIndex, site := range feature.Sites {
			siteCount++
			pattern, err := parseRuntimePatchPattern(site.AOB)
			if err != nil {
				t.Errorf("feature %s / catalog ID %d / site index %d / symbol %s: parse AOB: %v", feature.ID, feature.CatalogID, siteIndex, site.Symbol, err)
				continue
			}
			canonical := canonicalRuntimePatchAOB(pattern)
			matches, cached := matchCache[canonical]
			if !cached {
				matches = findRuntimePatchLocalPatternMatches(sections, pattern)
				matchCache[canonical] = matches
			}
			if len(matches) != 1 {
				t.Errorf("feature %s / catalog ID %d / site index %d / symbol %s: count=%d, want 1; locations=%s", feature.ID, feature.CatalogID, siteIndex, site.Symbol, len(matches), formatRuntimePatchLocalMatchLocations(matches))
				continue
			}
			match := matches[0]
			var matchedSection *runtimePatchLocalExecutableSection
			for sectionIndex := range sections {
				section := &sections[sectionIndex]
				if section.name == match.section && match.rva >= section.rva && uint64(match.rva-section.rva)+uint64(site.Offset)+uint64(len(site.EnableBytes)) <= uint64(len(section.data)) {
					matchedSection = section
					break
				}
			}
			if matchedSection == nil {
				t.Errorf("feature %s / catalog ID %d / site index %d / symbol %s: matched patch slice is outside executable section data", feature.ID, feature.CatalogID, siteIndex, site.Symbol)
				continue
			}
			start := int(match.rva-matchedSection.rva) + site.Offset
			actualOriginal := matchedSection.data[start : start+len(site.EnableBytes)]
			if !bytes.Equal(site.ExpectedOriginalBytes, actualOriginal) {
				t.Errorf("feature %s / catalog ID %d / site index %d / symbol %s: expectedOriginalBytes=% X, locked EXE has % X at RVA 0x%X", feature.ID, feature.CatalogID, siteIndex, site.Symbol, site.ExpectedOriginalBytes, actualOriginal, uint64(match.rva)+uint64(site.Offset))
			}
			locked, exists := evidenceByKey[[2]int{feature.CatalogID, siteIndex}]
			actualPatchRVA := uint64(match.rva) + uint64(site.Offset)
			if !exists || uint64(locked.PatchRVA) != actualPatchRVA {
				t.Errorf("feature %s / catalog ID %d / site index %d / symbol %s: evidence patch RVA=0x%X exists=%t, locked EXE patch RVA=0x%X", feature.ID, feature.CatalogID, siteIndex, site.Symbol, locked.PatchRVA, exists, actualPatchRVA)
			}
		}
	}
	t.Logf("verified %d runtime patch sites across %d canonical AOBs in %d executable PE sections", siteCount, len(matchCache), len(sections))
}
