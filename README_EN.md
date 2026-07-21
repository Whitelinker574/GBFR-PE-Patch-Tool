[简体中文](README.md) · [Documentation index](docs/README.md)

# GBFR PE Patch Tool · DLC 2.0.2

[Release v1.91.2](https://github.com/Whitelinker574/GBFR-PE-Patch-Tool/releases/latest)
[![CI](https://github.com/Whitelinker574/GBFR-PE-Patch-Tool/actions/workflows/ci.yml/badge.svg)](https://github.com/Whitelinker574/GBFR-PE-Patch-Tool/actions/workflows/ci.yml)

A Windows project for local save editing, controlled runtime tools, and read-only formula calibration for *Granblue Fantasy: Relink* DLC 2.0.2. It is not an official tool. Use it only with your own offline saves and local single-player environment, and keep recoverable backups.

The current stable release is **v1.91.2**. Releases include a Windows amd64 archive and SHA-256 checksums, and the in-app update checker only contacts this repository's releases.

## What is included

| Area | Current scope |
| --- | --- |
| Offline saves | Sigils, wrightstones, summons, loadouts, weapon skills, progression, quest counts, and title records. Writes use backup, checksum repair, atomic replacement, and readback. |
| Loadout workspace | Character, weapon, twelve sigils, mastery, and summons, with draft-versus-live HP/attack/critical/stun comparison. Unverified formulas remain estimates. |
| Runtime editors | Selected sigil, wrightstone, and summon records plus verified character/quest functions. Every write is bound to the process, owner token, selected target, and readback. |
| Read-only calibration | Final-panel reads, stability checks, strict A/B/A/B experiments, and redacted evidence bundles. This path does not inject, write memory, or edit saves. |
| EXE/compatibility | Audited local patches, backup/restore, and version diagnostics. Experimental items are labelled in the UI. |

## Catalog and write policy

The sigil, wrightstone, and summon catalogs are shared by offline-save, runtime-memory, and loadout routes and are sourced from audited 2.0.2 tables. Natural pools, combinations, and observed levels provide defaults and compact warnings only: every encodable selection is writable by default, with no separate force-mode switch.

That does not bypass safety checks. Target ownership, stale snapshots, storage bounds, integer encoding, checksums, transaction rollback, and field-by-field readback remain mandatory. An unopened-DLC save may receive values in existing preallocated summon records, but this does not unlock the system or guarantee that the game will consume the record.

## Before using it

1. Copy your save. Confirm the displayed backup path before an in-place write.
2. Back up the EXE before patching it; Steam file verification can restore it.
3. For runtime writes, confirm that the selected in-game character or item is the intended target. Re-read after changing character, reloading, or leaving the page.
4. Do not use runtime changes in multiplayer sessions or in ways that affect other players.

Default save path:

```text
C:\Users\YOUR_NAME\AppData\Local\GBFR\Saved\SaveGames\
```

## Build and test on Windows

Requirements: Windows amd64, Go 1.25+, Node.js/npm, Wails CLI v2.13, and the WebView2 Runtime. Visual Studio/MSBuild is needed only when rebuilding `src_dll/patch_core`.

```powershell
cd frontend
npm ci
npm run build
cd ..

go test ./...
node --test frontend/src/*.test.js
wails build -platform windows/amd64 -clean
```

The executable is written to `build\bin\GBFR PE Patch Tool.exe`.

## Documentation and evidence

- [Formula sources and evidence levels](docs/FORMULAS_2.0.2.md)
- [Read-only runtime formula sampling](docs/角色公式采样操作说明.md)
- [Save/memory catalog parity](docs/evidence/save-memory-table-parity.md)

Verified runtime reads are not presented as proof of every formula. Conditional buffs, damage reduction, damage caps, and combat settlement still require the appropriate training-area or target-dummy samples.

## Attribution and disclaimer

Required historical provenance and third-party licenses remain in the repository; current maintenance, downloads, and in-app updates point only to this repository.

This project is for learning and personal local use. You are responsible for the consequences of modifying saves, game files, or runtime memory.
