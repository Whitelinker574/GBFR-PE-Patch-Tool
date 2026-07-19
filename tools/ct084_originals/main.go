package main

import (
	"bytes"
	"crypto/sha256"
	"debug/pe"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	lockedCT084SourceVersion = "0.8.4"
	lockedCT084SourceSHA256  = "B75DF049E27D1423FC5ECDD47CC85DBAC241BEE582A49CEBA30CF020E150B659"
	lockedGame202SHA256      = "63340832BCF731FBC97796F686B05C988418E83D451D4A49B2244A85D00E297F"
	lockedGame202Size        = int64(123522016)
	lockedFeatureCount       = 58
	lockedSiteCount          = 81
)

type inputCatalog struct {
	SourceVersion string         `json:"sourceVersion"`
	SourceSHA256  string         `json:"sourceSha256"`
	Features      []inputFeature `json:"features"`
}

type inputFeature struct {
	ID    string      `json:"id"`
	CTID  int         `json:"ctId"`
	Sites []inputSite `json:"sites"`
}

type inputSite struct {
	Symbol      string `json:"symbol"`
	AOB         string `json:"aob"`
	Offset      int    `json:"offset"`
	EnableBytes []byte `json:"enableBytes"`
}

type executableIdentity struct {
	SHA256 string
	Size   int64
}

type executableSection struct {
	Name string
	RVA  uint32
	Data []byte
}

type originalManifest struct {
	SchemaVersion    int                `json:"schemaVersion"`
	SourceVersion    string             `json:"sourceVersion"`
	SourceSHA256     string             `json:"sourceSha256"`
	ExecutableSHA256 string             `json:"executableSha256"`
	ExecutableSize   int64              `json:"executableSize"`
	Sites            []originalSiteLock `json:"sites"`
}

type originalSiteLock struct {
	FeatureID             string    `json:"featureId"`
	CTID                  int       `json:"ctId"`
	SiteIndex             int       `json:"siteIndex"`
	Symbol                string    `json:"symbol"`
	AOB                   string    `json:"aob"`
	Offset                int       `json:"offset"`
	PatchRVA              uint32    `json:"patchRva"`
	ExpectedOriginalBytes byteArray `json:"expectedOriginalBytes"`
}

type byteArray []byte

func (values byteArray) MarshalJSON() ([]byte, error) {
	numbers := make([]uint16, len(values))
	for index, value := range values {
		numbers[index] = uint16(value)
	}
	return json.Marshal(numbers)
}

type bytePattern struct {
	values []byte
	masks  []byte
}

type patternMatch struct {
	sectionIndex int
	offset       int
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	inputCatalogPath := flag.String("input-catalog", "", "CT 0.8.4 catalog JSON generated from the locked CT source")
	inputExecutablePath := flag.String("input-exe", "", "unmodified Granblue Fantasy: Relink 2.0.2 executable")
	outputPath := flag.String("output", "", "output original-byte evidence manifest")
	flag.Parse()
	if strings.TrimSpace(*inputCatalogPath) == "" || strings.TrimSpace(*inputExecutablePath) == "" || strings.TrimSpace(*outputPath) == "" {
		return errors.New("-input-catalog, -input-exe, and -output are required")
	}

	catalog, err := readInputCatalog(*inputCatalogPath)
	if err != nil {
		return err
	}
	if catalog.SourceVersion != lockedCT084SourceVersion || !strings.EqualFold(catalog.SourceSHA256, lockedCT084SourceSHA256) {
		return fmt.Errorf("catalog is not derived from the locked CT %s source", lockedCT084SourceVersion)
	}
	if len(catalog.Features) != lockedFeatureCount {
		return fmt.Errorf("catalog feature count=%d, want %d", len(catalog.Features), lockedFeatureCount)
	}
	siteCount := 0
	for _, feature := range catalog.Features {
		siteCount += len(feature.Sites)
	}
	if siteCount != lockedSiteCount {
		return fmt.Errorf("catalog site count=%d, want %d", siteCount, lockedSiteCount)
	}

	identity, err := identifyFile(*inputExecutablePath)
	if err != nil {
		return fmt.Errorf("identify executable: %w", err)
	}
	if identity.Size != lockedGame202Size || !strings.EqualFold(identity.SHA256, lockedGame202SHA256) {
		return fmt.Errorf("executable identity is not the locked game 2.0.2 image (size=%d SHA256=%s)", identity.Size, identity.SHA256)
	}
	sections, err := readExecutableSections(*inputExecutablePath)
	if err != nil {
		return err
	}
	manifest, err := lockOriginals(catalog, sections, identity)
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode original-byte evidence: %w", err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(*outputPath, raw, 0o644); err != nil {
		return fmt.Errorf("write original-byte evidence: %w", err)
	}
	return nil
}

func readInputCatalog(path string) (inputCatalog, error) {
	file, err := os.Open(path)
	if err != nil {
		return inputCatalog{}, fmt.Errorf("open catalog: %w", err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	var catalog inputCatalog
	if err := decoder.Decode(&catalog); err != nil {
		return inputCatalog{}, fmt.Errorf("decode catalog: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return inputCatalog{}, errors.New("decode catalog: unexpected trailing JSON value")
		}
		return inputCatalog{}, fmt.Errorf("decode catalog trailing data: %w", err)
	}
	return catalog, nil
}

func identifyFile(path string) (executableIdentity, error) {
	file, err := os.Open(path)
	if err != nil {
		return executableIdentity{}, err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return executableIdentity{}, err
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return executableIdentity{}, err
	}
	return executableIdentity{SHA256: fmt.Sprintf("%X", hasher.Sum(nil)), Size: stat.Size()}, nil
}

func readExecutableSections(path string) ([]executableSection, error) {
	image, err := pe.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open PE image: %w", err)
	}
	defer image.Close()
	sections := make([]executableSection, 0, len(image.Sections))
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
		sections = append(sections, executableSection{Name: section.Name, RVA: section.VirtualAddress, Data: data[:int(limit)]})
	}
	if len(sections) == 0 {
		return nil, errors.New("PE image has no executable sections")
	}
	return sections, nil
}

func lockOriginals(catalog inputCatalog, sections []executableSection, identity executableIdentity) (originalManifest, error) {
	manifest := originalManifest{
		SchemaVersion:    1,
		SourceVersion:    catalog.SourceVersion,
		SourceSHA256:     strings.ToUpper(catalog.SourceSHA256),
		ExecutableSHA256: strings.ToUpper(identity.SHA256),
		ExecutableSize:   identity.Size,
		Sites:            make([]originalSiteLock, 0),
	}
	matchCache := make(map[string][]patternMatch)
	for _, feature := range catalog.Features {
		if feature.ID == "" || feature.CTID == 0 {
			return originalManifest{}, errors.New("catalog contains a feature with empty identity")
		}
		for siteIndex, site := range feature.Sites {
			label := fmt.Sprintf("%s site[%d] %s", feature.ID, siteIndex, site.Symbol)
			pattern, err := parsePattern(site.AOB)
			if err != nil {
				return originalManifest{}, fmt.Errorf("%s: %w", label, err)
			}
			if site.Offset < 0 || len(site.EnableBytes) == 0 || site.Offset > len(pattern.values) || len(site.EnableBytes) > len(pattern.values)-site.Offset {
				return originalManifest{}, fmt.Errorf("%s: invalid patch range", label)
			}
			matches, cached := matchCache[site.AOB]
			if !cached {
				matches = findPatternMatches(sections, pattern)
				matchCache[site.AOB] = matches
			}
			if len(matches) != 1 {
				return originalManifest{}, fmt.Errorf("%s: signature has %d executable matches, want exactly 1", label, len(matches))
			}
			match := matches[0]
			section := sections[match.sectionIndex]
			start := match.offset + site.Offset
			end := start + len(site.EnableBytes)
			if start < 0 || end < start || end > len(section.Data) {
				return originalManifest{}, fmt.Errorf("%s: patch slice is outside executable section", label)
			}
			original := append([]byte(nil), section.Data[start:end]...)
			if bytes.Equal(original, site.EnableBytes) {
				return originalManifest{}, fmt.Errorf("%s: locked image bytes already equal enable bytes", label)
			}
			patchRVA64 := uint64(section.RVA) + uint64(start)
			if patchRVA64 > uint64(^uint32(0)) {
				return originalManifest{}, fmt.Errorf("%s: patch RVA overflows", label)
			}
			manifest.Sites = append(manifest.Sites, originalSiteLock{
				FeatureID:             feature.ID,
				CTID:                  feature.CTID,
				SiteIndex:             siteIndex,
				Symbol:                site.Symbol,
				AOB:                   site.AOB,
				Offset:                site.Offset,
				PatchRVA:              uint32(patchRVA64),
				ExpectedOriginalBytes: byteArray(original),
			})
		}
	}
	return manifest, nil
}

func parsePattern(raw string) (bytePattern, error) {
	compact := strings.ToUpper(strings.Join(strings.Fields(raw), ""))
	if compact == "" || len(compact)%2 != 0 {
		return bytePattern{}, errors.New("invalid empty or odd-length AOB")
	}
	pattern := bytePattern{values: make([]byte, len(compact)/2), masks: make([]byte, len(compact)/2)}
	for index, char := range []byte(compact) {
		if char == '?' || char == 'X' {
			continue
		}
		var nibble byte
		switch {
		case char >= '0' && char <= '9':
			nibble = char - '0'
		case char >= 'A' && char <= 'F':
			nibble = char - 'A' + 10
		default:
			return bytePattern{}, fmt.Errorf("invalid AOB character %q", char)
		}
		byteIndex := index / 2
		shift := uint(4)
		if index%2 == 1 {
			shift = 0
		}
		pattern.values[byteIndex] |= nibble << shift
		pattern.masks[byteIndex] |= 0x0f << shift
	}
	return pattern, nil
}

func findPatternMatches(sections []executableSection, pattern bytePattern) []patternMatch {
	anchorStart, anchorLength := 0, 0
	for start := 0; start < len(pattern.masks); {
		if pattern.masks[start] != 0xff {
			start++
			continue
		}
		end := start + 1
		for end < len(pattern.masks) && pattern.masks[end] == 0xff {
			end++
		}
		if end-start > anchorLength {
			anchorStart, anchorLength = start, end-start
		}
		start = end
	}
	var matches []patternMatch
	for sectionIndex, section := range sections {
		if len(section.Data) < len(pattern.values) {
			continue
		}
		if anchorLength == 0 {
			for offset := 0; offset <= len(section.Data)-len(pattern.values); offset++ {
				if matchPattern(section.Data[offset:], pattern) {
					matches = append(matches, patternMatch{sectionIndex: sectionIndex, offset: offset})
				}
			}
			continue
		}
		anchor := pattern.values[anchorStart : anchorStart+anchorLength]
		searchFrom := 0
		for searchFrom <= len(section.Data)-anchorLength {
			relative := bytes.Index(section.Data[searchFrom:], anchor)
			if relative < 0 {
				break
			}
			anchorOffset := searchFrom + relative
			candidateOffset := anchorOffset - anchorStart
			if candidateOffset >= 0 && candidateOffset+len(pattern.values) <= len(section.Data) && matchPattern(section.Data[candidateOffset:], pattern) {
				matches = append(matches, patternMatch{sectionIndex: sectionIndex, offset: candidateOffset})
			}
			searchFrom = anchorOffset + 1
		}
	}
	return matches
}

func matchPattern(data []byte, pattern bytePattern) bool {
	if len(data) < len(pattern.values) || len(pattern.values) != len(pattern.masks) {
		return false
	}
	for index := range pattern.values {
		if data[index]&pattern.masks[index] != pattern.values[index] {
			return false
		}
	}
	return true
}
