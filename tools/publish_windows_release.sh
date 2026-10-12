#!/usr/bin/env bash
set -euo pipefail
archive="GBFR-PE-Patch-Tool-${RELEASE_VERSION}-windows-amd64"
test -s "${archive}.exe"
test -s "${archive}.zip"
sha256sum --check "SHA256SUMS-${RELEASE_VERSION}.txt"
python - <<'PY'
import hashlib, json, os, pathlib, zipfile
version = os.environ['RELEASE_VERSION']
with zipfile.ZipFile(f'GBFR-PE-Patch-Tool-{version}-windows-amd64.zip') as archive:
    provenance = json.loads(archive.read('BUILD_PROVENANCE.json'))
    assert provenance['commit'] == os.environ['RELEASE_COMMIT']
    assert provenance['version'] == version
    exe_name = f'GBFR-PE-Patch-Tool-{version}-windows-amd64.exe'
    exe_hash = hashlib.sha256(pathlib.Path(exe_name).read_bytes()).hexdigest()
    assert provenance['executable']['sha256'] == exe_hash
    assert hashlib.sha256(archive.read(exe_name)).hexdigest() == exe_hash
    assert 'THIRD_PARTY_NOTICES.md' in archive.namelist()
    assert len([name for name in archive.namelist() if name.startswith('licenses/native/') and name.endswith('.txt')]) == 7
PY
existing=$(gh api "repos/${GITHUB_REPOSITORY}/releases?per_page=100" --jq ".[] | select(.tag_name == \"${RELEASE_VERSION}\") | .draft")
if [ "$existing" = "false" ]; then
  echo 'Release already published; leaving its assets unchanged.'
  exit 0
fi
title=$(head -n 1 RELEASE_NOTES.md | tr -d '\r')
title=${title#\# }
tag_commit=$(gh api "repos/${GITHUB_REPOSITORY}/git/matching-refs/tags/${RELEASE_VERSION}" \
  --jq ".[] | select(.ref == \"refs/tags/${RELEASE_VERSION}\") | .object.sha")
if [ -z "$tag_commit" ]; then
  gh api --method POST "repos/${GITHUB_REPOSITORY}/git/refs" \
    -f "ref=refs/tags/${RELEASE_VERSION}" -f "sha=${RELEASE_COMMIT}" >/dev/null
fi
tag_commit=$(gh api "repos/${GITHUB_REPOSITORY}/git/ref/tags/${RELEASE_VERSION}" --jq '.object.sha')
test "$tag_commit" = "$RELEASE_COMMIT"
if [ "$existing" != "true" ]; then
  gh release create "$RELEASE_VERSION" --repo "$GITHUB_REPOSITORY" \
    --verify-tag --draft --title "$title" --notes-file RELEASE_NOTES.md
fi
gh release upload "$RELEASE_VERSION" --repo "$GITHUB_REPOSITORY" --clobber \
  "${archive}.exe" "${archive}.zip" "SHA256SUMS-${RELEASE_VERSION}.txt"
gh release edit "$RELEASE_VERSION" --repo "$GITHUB_REPOSITORY" \
  --draft=false --latest --notes-file RELEASE_NOTES.md
