package backend

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const naturalDropHistoryDirectoryName = "data.i.gbfr-codex-natural-drop.history"

// These are independently measured original indices, not hashes supplied by a
// deployment manifest. Pair the index with the already verified executable:
// an arbitrary modded index must never be adopted as an upgrade baseline.
func naturalDropKnownOriginalIndex(identity naturalDropGameIdentityRecord, digest string) bool {
	if !naturalDropManifestIdentitySupported(identity.Version, identity.SHA256) {
		return false
	}
	var expected string
	switch identity.Version {
	case naturalDropGame203Version, naturalDropGame204Version:
		expected = "F7C4A2D3AE65B85CEC96EF3A77B8C3A370A9A02584C6A77DD7AF5C676B1511EE"
	case naturalDropGame205Version:
		expected = "4FA5FDE9A94D3F995CB3DCFADB1238DCA739CDD313C15BD59109E9DBF3385802"
	case naturalDropGame206Version:
		expected = "0A6B9989F0F0E59C4E67D5128A115DFB12560397A3421BDCCFB65D1DEBBB73F9"
	default:
		return false
	}
	return strings.EqualFold(digest, expected)
}

// A legacy manifest may still say 2.0.2, so compare actual bytes rather than
// trusting its version label. Return the old backup separately for rollback;
// it must NEVER replace the new original index during migration or removal.
func naturalDropResolveOwnedBaseline(gameDir string, manifest *naturalDropManifest, current []byte, identity naturalDropGameIdentityRecord) (base, oldBackup []byte, migrating bool, err error) {
	digest := fileSHA256(current)
	if strings.EqualFold(digest, manifest.OriginalIndexSHA) || strings.EqualFold(digest, manifest.DeployedIndexSHA) {
		base, err = naturalDropVerifyOwnedCurrent(gameDir, manifest, current)
		return base, base, false, err
	}
	if !naturalDropKnownOriginalIndex(identity, digest) {
		_, err = naturalDropVerifyOwnedCurrent(gameDir, manifest, current)
		return nil, nil, false, err
	}
	oldBackup, err = naturalDropVerifyOwnedFiles(gameDir, manifest)
	if err != nil {
		return nil, nil, false, err
	}
	return current, oldBackup, true, nil
}

// Preserve all old evidence before changing ANY active file. Each generation
// is a new directory; incomplete archives are harmless and never auto-loaded.
// archive.json is written last and records hashes for every archived file.
func naturalDropArchiveLegacyInstall(gameDir string, current, backup, manifest []byte, tables map[string][]byte) (string, error) {
	files := map[string][]byte{"data.i": current, naturalDropBackupName: backup, naturalDropManifestName: manifest}
	for relative, data := range tables {
		if err := naturalDropValidatePreparedRelative(relative); err != nil {
			return "", err
		}
		if data != nil {
			files[relative] = data
		}
	}
	history := filepath.Join(gameDir, naturalDropHistoryDirectoryName)
	if err := os.MkdirAll(history, 0o755); err != nil {
		return "", err
	}
	archive, err := os.MkdirTemp(history, "migration-")
	if err != nil {
		return "", err
	}
	hashes := make(map[string]string, len(files))
	for relative, data := range files {
		path := filepath.Join(archive, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return "", fmt.Errorf("保留旧部署文件失败（%s）: %w", archive, err)
		}
		if err := naturalDropWriteAtomic(path, data); err != nil {
			return "", fmt.Errorf("保留旧部署文件失败（%s）: %w", archive, err)
		}
		hashes[relative] = fileSHA256(data)
	}
	metadata, err := json.MarshalIndent(struct {
		SchemaVersion int               `json:"schemaVersion"`
		Owner         string            `json:"owner"`
		Files         map[string]string `json:"files"`
	}{1, naturalDropModID, hashes}, "", "  ")
	if err != nil {
		return "", err
	}
	if err := naturalDropWriteAtomic(filepath.Join(archive, "archive.json"), append(metadata, '\n')); err != nil {
		return "", err
	}
	return archive, nil
}
