# Third-party notices

This file records third-party components used by the application and its native helper. It does not grant a license for this repository's inherited or project-specific code. Release archives include the corresponding verbatim license files under `licenses/`.

## Project provenance

This repository was originally forked from [BitterG/GBFR-PE-Patch-Tool](https://github.com/BitterG/GBFR-PE-Patch-Tool). The upstream README records additional method provenance for early save parsing, sigil generation, and wrightstone generation. Neither repository currently declares a project-wide license covering the inherited code; public visibility alone must not be treated as permission to redistribute or relicense it.

## Go components linked into the Windows executable

| Module | Version | License |
| --- | --- | --- |
| `github.com/cespare/xxhash/v2` | v2.3.0 | MIT |
| `github.com/leaanthony/go-ansi-parser` | v1.6.1 | MIT |
| `github.com/leaanthony/slicer` | v1.6.0 | MIT |
| `github.com/leaanthony/u` | v1.1.1 | MIT |
| `github.com/pkg/errors` | v0.9.1 | BSD-2-Clause |
| `github.com/rivo/uniseg` | v0.4.7 | MIT |
| `github.com/wailsapp/go-webview2` | v1.0.22 | MIT |
| `github.com/wailsapp/wails/v2` | v2.13.0 | MIT |
| `golang.org/x/sys` | v0.44.0 | BSD-3-Clause |

The table is derived from `go list -deps ./...`; build-only and test-only modules are not listed as runtime components.

## Frontend production dependency graph

| Packages | Version | License |
| --- | --- | --- |
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

## Native helper

`src_dll/thirdparty/libmem/` includes libmem-derived headers and associated Capstone, Keystone, LLVM, and libmem notices. Their verbatim terms remain in [`src_dll/thirdparty/libmem/licenses/`](src_dll/thirdparty/libmem/licenses/) and are copied into release archives.

## Game-related names and assets

Granblue Fantasy: Relink, its characters, names, and game-derived UI assets belong to their respective rights holders. Their presence for local catalog matching or interface identification does not imply endorsement and is not covered by the open-source component licenses listed above.
