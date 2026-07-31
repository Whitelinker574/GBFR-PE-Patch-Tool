# Third-party notices

This file records third-party components used by the application and its native helper. It does not grant a license for this repository's inherited or project-specific code. Native-helper license texts remain beside that source under `src_dll/thirdparty/libmem/licenses/`; package versions and license identifiers are recorded below from the locked dependency graphs.

## Project provenance

This repository was originally forked from [BitterG/GBFR-PE-Patch-Tool](https://github.com/BitterG/GBFR-PE-Patch-Tool). The upstream README records additional method provenance for early save parsing, sigil generation, and wrightstone generation. Neither repository currently declares a project-wide license covering the inherited code; public visibility alone must not be treated as permission to redistribute or relicense it.

## Go components linked into the Windows executable

| Module | Version | License |
| --- | --- | --- |
| `github.com/andybalholm/brotli` | v1.2.2 | MIT |
| `github.com/cespare/xxhash/v2` | v2.3.0 | MIT |
| `github.com/dustin/go-humanize` | v1.0.1 | MIT |
| `github.com/fxamacker/cbor/v2` | v2.9.0 | MIT |
| `github.com/google/flatbuffers` | v25.12.19 | Apache-2.0 |
| `github.com/klauspost/compress` | v1.18.4 | BSD-3-Clause |
| `github.com/leaanthony/go-ansi-parser` | v1.6.1 | MIT |
| `github.com/leaanthony/slicer` | v1.6.0 | MIT |
| `github.com/leaanthony/u` | v1.1.1 | MIT |
| `github.com/mattn/go-isatty` | v0.0.20 | MIT |
| `github.com/ncruces/go-strftime` | v0.1.9 | MIT |
| `github.com/pkg/errors` | v0.9.1 | BSD-2-Clause |
| `github.com/remyoudompheng/bigfft` | v0.0.0-20230129092748-24d4a6f8daec | BSD-3-Clause |
| `github.com/rivo/uniseg` | v0.4.7 | MIT |
| `github.com/vmihailenco/msgpack/v5` | v5.4.1 | BSD-2-Clause |
| `github.com/vmihailenco/tagparser/v2` | v2.0.0 | BSD-2-Clause |
| `github.com/wailsapp/go-webview2` | v1.0.22 | MIT |
| `github.com/wailsapp/wails/v2` | v2.13.0 | MIT |
| `github.com/x448/float16` | v0.8.4 | MIT |
| `golang.org/x/exp` | v0.0.0-20250620022241-b7579e27df2b | BSD-3-Clause |
| `golang.org/x/sys` | v0.44.0 | BSD-3-Clause |
| `modernc.org/libc` | v1.66.10 | BSD-3-Clause |
| `modernc.org/mathutil` | v1.7.1 | BSD-3-Clause |
| `modernc.org/memory` | v1.11.0 | BSD-3-Clause |
| `modernc.org/sqlite` | v1.43.0 | BSD-3-Clause |

The table is derived from `go list -deps ./...`; build-only and test-only modules are not listed as runtime components.

## Frontend production dependency graph

| Packages | Version | License |
| --- | --- | --- |
| `base32768` | 5.0.1 | MIT |
| `html-to-image` | 1.11.13 | MIT |
| `qrcode`, `dijkstrajs`, `pngjs` | 1.5.4, 1.0.3, 5.0.0 | MIT |
| `yargs`, `string-width`, `strip-ansi`, `wrap-ansi`, `emoji-regex`, `ansi-regex`, `is-fullwidth-code-point` | 15.4.1, 4.2.3, 6.0.1, 6.2.0, 8.0.0, 5.0.1, 3.0.0 | MIT |
| `decamelize`, `find-up`, `locate-path`, `path-exists`, `p-locate`, `p-limit`, `camelcase`, `require-directory` | 1.2.0, 4.1.0, 5.0.0, 4.0.0, 4.1.0, 2.3.0, 5.3.1, 2.1.1 | MIT |
| `cliui`, `get-caller-file`, `require-main-filename`, `set-blocking`, `which-module`, `y18n`, `yargs-parser` | 6.0.0, 2.0.5, 2.0.0, 2.0.0, 2.0.1, 4.0.3, 18.1.3 | ISC |
| `vue`, `@vue/compiler-core`, `@vue/compiler-dom`, `@vue/compiler-sfc`, `@vue/compiler-ssr`, `@vue/reactivity`, `@vue/runtime-core`, `@vue/runtime-dom`, `@vue/server-renderer`, `@vue/shared` | 3.5.40 | MIT |
| `@babel/helper-string-parser`, `@babel/helper-validator-identifier`, `@babel/parser`, `@babel/types` | 7.29.7 | MIT |
| `@jridgewell/sourcemap-codec` | 1.5.5 | MIT |
| `csstype` | 3.2.3 | MIT |
| `entities` | 7.0.1 | BSD-2-Clause |
| `estree-walker` | 2.0.2 | MIT |
| `magic-string` | 0.30.21 | MIT |
| `nanoid` | 3.3.16 | MIT |
| `opencc-js` | 1.4.1 | MIT AND Apache-2.0 |
| `picocolors` | 1.1.1 | ISC |
| `pinyin-pro` | 3.28.1 | MIT |
| `postcss` | 8.5.21 | MIT |
| `source-map-js` | 1.2.1 | BSD-3-Clause |

Versions and license identifiers come from `frontend/package-lock.json`. Development-only build tools are recorded by that lockfile but are not represented as application runtime dependencies here.

## Development verification dependencies

`jsqr` 1.4.0 (Apache-2.0) and `sharp` 0.35.3 (Apache-2.0) are development-only dependencies. They are used to decode resized/recompressed share QR images and to prepare test rasters; neither package is bundled into the production frontend entry.

## Native helper

`src_dll/thirdparty/libmem/` includes libmem-derived headers and associated Capstone, Keystone, LLVM, and libmem notices. Their verbatim terms remain in [`src_dll/thirdparty/libmem/licenses/`](src_dll/thirdparty/libmem/licenses/) and are copied into release archives. Camera, voice-event and virtual-sigil runtime hooks are compiled into the application-owned `patch_core.dll`; no external mod loader or managed hook runtime is distributed.

## Game-related names and assets

Granblue Fantasy: Relink, its characters, names, and game-derived UI assets belong to their respective rights holders. Their presence for local catalog matching or interface identification does not imply endorsement and is not covered by the open-source component licenses listed above.

## Research references

The Fate Episode catalog contract and GBFR text-hash cross-checks were independently verified against the MIT-licensed `relink-save-forge` research project by X-Zero-L. This application implements its own strict Go transaction for the independently verified 2.0.2 Fate and mission-state families; it does not bundle that project's save editor or Python scripts, and it preserves unverified secondary counters.

The Conflux endless-quest timer layout, its original 2.0.2 values, and the optional Logs `DamageDetails` fields (`uncapped_damage` and `damage_cap`) were independently verified against the MIT-licensed `gbfr-djeetamod` project. This application uses its own Go process ownership, validation, rollback, read-back, and read-only Logs parsing implementation and does not bundle that project's Tauri application or Rust source.

The town-camera behavior and three signature families were compared with Nenkai's MIT-licensed `GBFRUnlockedTownCameraAdjustment`. This repository carries an independently integrated native runtime with an exact 2.0.2 executable hash, unique signature checks, application-managed configuration, and unload restoration.

The legitimate Transmarvel wrightstone table layout, four item families, three drop pools and fixed 20/15/10 result shape were cross-checked against Evoyn's MIT-licensed `gbfer-wrightstone-picker`. This application implements the byte transformations in its own Go deployment transaction and uses its existing `data.i` backup, conflict detection, atomic write, read-back and restoration path; it does not bundle the reference GUI or Reloaded-II integration.

Virtual-sigil product behavior was compared with the publicly visible behavior of `GBFR-Extra-Sigil-Slots`. No explicit source license was confirmed, so its source code is not incorporated. The helper in this repository is implemented independently from the verified 2.0.2 executable layout, real save inventory records, and this application's existing status-object research.
