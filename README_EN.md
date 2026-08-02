# Relink Skyfarer Workshop (Granblue Fantasy: Relink)

[简体中文](README.md) · [Download the latest stable release](https://github.com/Whitelinker574/GBFR-PE-Patch-Tool/releases/latest) · [Open the community loadout catalog](https://share.whitelinker.top/?lang=en)

A Windows save, loadout, and sharing utility for *Granblue Fantasy: Relink* DLC 2.0.3, with the verified 2.0.2 offline runtime capabilities retained. Formerly known as GBFR PE Patch Tool; renamed to "Relink Skyfarer Workshop" in v2.0.12.

## Quick start

1. Download the `windows-amd64` EXE or ZIP from [GitHub Releases](https://github.com/Whitelinker574/GBFR-PE-Patch-Tool/releases/latest), extract, and run it.
2. Open **Saves & Loadouts** first and verify the detected `SaveData` slot.
3. Exit the game before an offline edit, review the draft, then confirm the write.
4. For a live feature, start the game, connect from that feature's page, and use **Stop/Restore** when finished.
5. If the application fails to start, inspect `%LOCALAPPDATA%\GBFR-PE-Patch-Tool\startup.log`.

Default save directory:

```text
C:\Users\<username>\AppData\Local\GBFR\Saved\SaveGames\
```

## Feature overview (five workspaces)

### 1. Saves & Loadouts (offline writes)

Exit the game completely before using these pages. Every write follows the same pipeline: automatic backup → edit in a temporary file → checksum repair → atomic replacement → reopen and read the changed fields back.

| Page | What it does |
| --- | --- |
| Loadout presets | View, edit, and import complete loadouts (characters, weapons and skills, 12 sigils, wrightstones, summons, mastery, Over Mastery, character growth). Smart loadout builds a twelve-sigil set from skill targets: goals are processed top to bottom, real non-duplicate inventory sigils are preferred, gaps are reported explicitly, and missing sigils are only created after confirmation |
| Sigil editor | Create, batch-manage, and delete save sigils; configure level and primary/secondary traits. Combination checks warn but never override your choice |
| Wrightstone editor | Create wrightstones and set three traits, with queue-based batch generation |
| Summons (save) | Add or modify summon type, boon, traits, level, and status; changing type migrates equipped references automatically |
| Items & weapons | Edit items, materials, growth resources, and weapon levels; supports weapon awakening/transcendence stage sync |
| Character usage | View and batch-modify usage counts for selected characters |
| Quests & titles | Edit quest clear counts, title unlocks, and viewed status; title reward claim records stay untouched |
| Save compare & copy | Side-by-side diff of two saves, categorized by character growth / loadout / items / quest state / unknown structures. Records with audited semantics can be copied in either direction; one-sided additions and unknown structures cannot |

### 2. In-game live editors (live writes)

Start the game and enter a save first, then connect from the page. Every write is bound to the current process and the selected target, with a read-back after writing. Re-enter the save, restart the game, or refresh the target list, and re-connect or re-select.

| Page | What it does |
| --- | --- |
| Live sigil editor | Edit the currently highlighted sigil (level, primary/secondary traits) |
| Live wrightstone editor | Write the current wrightstone's three traits in one transaction |
| Weapon skill editor | Back up, edit, and read back the five permanent skill slots; skill six and beyond append through the game's native skill-aggregation path |
| Live summon editor | Edit the summoned stone's main sigil, sub-parameters, and tier; persists by calling the game's save function |
| Over Mastery | Read and edit the four capability slots on the Over Mastery result screen |
| Currency, materials & drops | Edit gil, potions, material consumption, and quest drops at runtime |

### 3. Loadout capture & restore (read-only monitoring + live recording)

| Page | What it does |
| --- | --- |
| Persistent party capture | Continuously connects after you enable it, waits for three stable snapshots, then archives each quest's party loadout locally; imports Relink Logs JSON / `logs.db`; browse battle archives, party damage, skill details, and loadout snapshots. Missing fields display "Not Recorded" and are never padded with another character's data |
| Twelve-sigil live record/restore | Records the current 12 sigils and exports JSON, or restores them item-by-item onto backup sigils; sharing generates short codes, QR codes, and share images |

### 4. Offline runtime tools (single-player / host only)

Intended for offline or host environments only. Once enabled, status appears in the title bar, survives page switches, and is fully restored by F12 or by disabling on the page.

| Page | What it does |
| --- | --- |
| Display & room | Screen, room ID, party leader, position, and other runtime info |
| Virtual sigil slots | Persistent extra virtual sigils that survive page switches |
| Character voice mixer | Adjust character voice |
| Town camera workshop | Five camera presets (detail / default / comfort / combat wide / far view) |
| Spatial movement | Virtual-ground movement: WASD horizontal, PageUp/PageDown elevation; gravity suppression |
| Combat / character / quest patches | Dodge, guard, Link, summon limit, part-break, quest countdown, quest score, side objectives, summon duration, free craft/trade/upgrade, cooldown & charge tuning — 60 runtime patches; live quest-reward multiplier 1×–16× |
| Monster tuning | Monster HP, damage, stagger bar, and Overdrive state (experimental) |
| Endless mode | Endless-mode timer and rules (read-only reference) |

### 5. Game files, diagnostics & settings

| Page | What it does |
| --- | --- |
| Natural drops & forging | Two independent paths: a live "ordinary-item multiplier for all quest results" (1×–16×) and a deployment list that writes selected items into Endless Mode Forger's Bounty (1–999 base quantity). The paths do not stack; `data.i` deployment always provides backup and restoration |
| Selected-item read-only | Inspect the currently highlighted item without writing |
| Formula sampler | Read-only sampling of final HP, attack, crit, and stagger values with strict A/B/A/B experiments and sanitized evidence export; never writes the process or save |
| Compatibility | Per-feature 2.0.3 compatibility status; unverified features are marked explicitly |
| Game file maintenance | Steam path auto-detection, game EXE `.bak` backup and restore |
| Language & settings | Chinese/English UI switch, saved locally |

## Safety and data boundaries

- **Backup before write, read back after write** is the uniform rule: offline writes create a recoverable backup and re-read every changed field; live writes bind `{PID, creation time}`, verify the executable identity and original bytes before enabling, and restore on stop/disconnect/exit
- **Legality grading**: every edit is labeled Legal / Forced / Unknown / Non-writable. Combinations beyond verified game rules are rejected outright; forced writes warn first and never silently overwrite your requested values
- **No fake data**: the UI never presents global events as personal DPS or table baselines as final per-action values; values without field evidence stay marked estimated, candidate, or under verification
- **Experimental boundaries**: natural drops, virtual sigils, camera, spatial movement, and gravity suppression keep an Experimental badge; current-session damage ownership mapping and exact final per-action caps are not covered yet; noclip and camera-relative flight have no entry point
- **Offline / live / read-only** workflows are strictly separated: offline writes save files, live writes process memory, read-only changes nothing

## Online loadout sharing

- Identical immutable loadout frames receive a stable short code (16–24 digits), deduplicated server-side; QR share images can be imported from local recognition (images are never uploaded)
- The public catalog searches characters, weapons, sigils, wrightstones, summons, mastery, and skills; sorts by newest / name / likes; supports like/unlike and comments
- Share-image workshop: `1920×1080` landscape, `1440×1920` portrait, `1600×1600` square; PNG downloads keep real pixel size
- Public upload can be disabled; offline long codes and JSON files remain available as fallback

## Performance and compatibility

| Item | Details |
| --- | --- |
| System | Windows 10/11 x64 |
| Game version | 2.0.3 (offline/static verified; live works with 2.0.2/2.0.3; game 2.0.3 save/restart readback remains a field acceptance item) |
| Minimum window | 960×640, covering 100%/125%/150% scaling |
| Performance | Loadout solving runs in a cancellable Web Worker; 50 warm transitions P95 < 300 ms |
| Scenario coverage | 28 pages × 2 languages × 7 viewports × 3 scale factors = 1,176 cases |

## Troubleshooting

| Symptom | First step |
| --- | --- |
| Save not found | Check the default directory, or use Browse… to pick `SaveData*.dat` manually |
| Multiple saves, unsure which one | Do not guess by order; select each and confirm by the character/loadout/records shown |
| Live page says disconnected | Confirm the game is running and in a save, then re-connect on the current page |
| Target stale / pointer null | Re-select the target in the game, refresh in the tool, then write |
| Version or EXE unrecognized | Stop file patches; check per-feature status in Compatibility, or use Steam file verification |
| Result wrong after a write | Stop writing and fully exit the game; restore from the backup point created before the write |

## Development and verification

```powershell
cd frontend
npm ci
npm test
npm run build

cd ..
go test ./...
go vet -unsafeptr=false ./...
build-windows.bat
```

- Build environment: Windows amd64, Go 1.25+, Node.js/npm, Wails CLI v2.13, WebView2 Runtime
- Release packages: `tools/package_windows_release.ps1` produces the versioned EXE, a ZIP with license files, and a SHA-256 manifest
- Online sharing service: `services/loadout-share` (Cloudflare Worker + R2/D1)

## Acknowledgements

This project is forked from [BitterG/GBFR-PE-Patch-Tool](https://github.com/BitterG/GBFR-PE-Patch-Tool). The early implementation of save parsing and sigil/wrightstone generation follows its public work; that repository's README also credits Xzire91x and Nenkai as the upstream method sources. The current repository has been rewritten and extended on top of it. This note only preserves the source chain and does not imply endorsement, authorization, or participation by the original authors.

Other public resources are used for cross-checking only: loadout interaction was cross-referenced against a community loadout simulator; Chinese terminology against LKong621's public content; data extraction used Nenkai's public tools; summon hints were cross-checked against SinnohDawn's public notes and Relink Summon. None of these links imply cooperation, authorization, code porting, or endorsement.

## Disclaimer

This is an unofficial community tool for learning and personal local use only. It is not affiliated with, sponsored by, or authorized by Cygames, SEGA, the game's publishers, or the community authors referenced by the project. Modifying save files, game files, or runtime memory may cause data corruption, progress loss, or trigger the game's own checks. Work only with files you are entitled to use, keep recoverable backups, and accept the consequences yourself. Do not use this project for paid modification services or to affect other players in online environments.

This repository does not declare a project-level license covering all inherited code; except for third-party components with their own explicit licenses, public availability of the repository alone does not grant rights to copy, redistribute, or sell. The repository does not include, mirror, crack, or resell any third-party paid tables, member content, or restricted downloads.
