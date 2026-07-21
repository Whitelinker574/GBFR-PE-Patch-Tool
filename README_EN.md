<p align="center">
  <img src="build/appicon.png" width="112" alt="GBFR PE Patch Tool" />
</p>

<h1 align="center">GBFR PE Patch Tool</h1>

<p align="center">Windows save editing, live changes, and read-only monitoring for <em>Granblue Fantasy: Relink</em> DLC 2.0.2</p>

<p align="center">
  <a href="https://github.com/Whitelinker574/GBFR-PE-Patch-Tool/releases/latest">Download v1.91.4</a> ·
  <a href="README.md">简体中文</a> ·
  <a href="docs/README.md">Documentation and evidence</a>
</p>

![Feature home](docs/screenshots/home.png)

This is an unofficial desktop tool whose data layouts and runtime locations target DLC 2.0.2. Its three operating modes have separate boundaries: offline save editing runs with the game closed and uses backup, checksum, and atomic replacement; live changes connect to the running game and write memory; read-only monitoring observes runtime values for diagnostics and formula calibration without writing.

The tool works with records already present in local saves or the current game process. It does not unlock DLC content that an account does not own or a save has not opened. Development and limited field checks were performed on DLC 2.0.2, but not every game state has been exercised. Back up the target first and confirm every write through the page readback.

## Quick start

1. Download the Windows amd64 archive from [Releases](https://github.com/Whitelinker574/GBFR-PE-Patch-Tool/releases/latest) and verify the supplied SHA-256.
2. Close the game before editing a save. Back up the target before changing a save or executable.
3. Choose `Save Editing (Offline)`, `Live Injection`, or `Memory Monitoring (Read Only)` from the sidebar.
4. Confirm the save slot, character, item, and destination before every write, then inspect the readback result.

Default save directory:

```text
C:\Users\YOUR_NAME\AppData\Local\GBFR\Saved\SaveGames\
```

Multiple detected saves are displayed as separate slots, and a file can also be selected manually. Offline writes use backup creation, checksum repair, a temporary file, atomic replacement, and a fresh read. Never edit the same save offline while the game is using it.

## Feature map

| Workspace | Pages | Scope |
| --- | --- | --- |
| Save Editing (Offline) | 7 | Loadout presets, sigils, items and weapons, wrightstones, summon saves, character usage, quest and title records |
| Live Injection | 10 | General live values, live sigils/wrightstones, loadout capture, summons, Over Mastery, combat/character/quest patches, monster experiments |
| Memory Monitoring (Read Only) | 2 | Party and selected-item monitoring, final character panel reads, and A/B/A/B sampling |
| Tools & Settings | 3 | Version diagnostics, language/display, and game-file maintenance |

### Saves and loadouts

![Loadout presets and three save slots](docs/screenshots/loadout-presets.png)

- Save, memory, and loadout editors share the same audited 2.0.2 sigil, wrightstone, and summon catalogs.
- The loadout workspace reads all fifteen preset slots per character and edits weapons, twelve sigils, four skills, three mastery trees, and summons.
- A single loadout can be imported or exported. Import only maps resources already owned by the selected save; it is not a bulk save transfer.
- Natural combinations, observed levels, and summon legality are compact warnings. Every encodable user selection is writable by default, while ownership, storage bounds, integer ranges, checksums, and readback remain mandatory.
- Writing a preallocated record in a pre-DLC save does not unlock the system or guarantee that the game will consume the record.

The estimator separates character progression, weapons, sigils, mastery, Over Mastery, and summon contributions. When live final HP, attack, critical rate, and stun are available, draft values are compared against the game. Results without field evidence remain labelled estimate, candidate, or open—not final formulas.

### Runtime tools and patches

![Combat patch catalog](docs/screenshots/patch-combat.png)

- Live writes are bound to the current process, an ownership token, and the selected target. Character changes, save reloads, and stale targets invalidate old snapshots.
- Combat, character, and quest patch pages share a persistent connection. Switching pages keeps active patches; explicit disconnect or application shutdown restores them together.
- Patch sites are locked to DLC 2.0.2 original bytes, signatures, and writeback. Conflicts are catalogued explicitly and version guards are not weakened to accept look-alike sites.
- The auto-perfect-guard combo fix, solo party monitor, and training-area defense samples have repeated field evidence. Runtime functions not exercised in every scenario are not advertised as universally field-tested.

Use live features only in a local single-player environment. Reconnect after restarting the game, and do not carry runtime changes into multiplayer sessions.

### Read-only monitoring and calibration

![Character formula sampler](docs/screenshots/formula-sampler.png)

The formula sampler requests query and read access only: it installs no hook, injects nothing, writes no memory, and edits no save. It continuously reads final HP, attack, critical rate, and stun, can capture stable before/transition/after states, and retains strict A1/B1/A2/B2 validation. Exported evidence removes PIDs, module bases, absolute addresses, user names, and local paths.

Defense, reduction, damage caps, and conditional skills may not change the character panel. They require training-area hit or target-dummy samples; an unchanged four-stat panel is only a negative observation.

## Safety and evidence boundaries

| Layer | Enforced behavior | Not promised |
| --- | --- | --- |
| Save writes | Backup, transaction, checksums, atomic replacement, readback | DLC unlocking or game acceptance of every unsupported combination |
| Runtime writes | Version guard, owner token, target snapshot, writeback, restoration | Multiplayer support or compatibility with unknown executables |
| Read-only monitoring | Bounded object reads, stability checks, redacted export | Full-process dumps or presenting candidates as final formulas |
| Loadout estimates | Source breakdown, evidence level, live-value delta | Hard-coded screenshot values or unverified scaling made to look exact |

See [formula sources and evidence](docs/FORMULAS_2.0.2.md), the [sampling guide](docs/角色公式采样操作说明.md), and [save/memory catalog parity](docs/evidence/save-memory-table-parity.md).

## Repository layout

```text
main.go                              Minimal Wails entry point and frontend embedding
internal/backend/README.md           Backend feature-family and filename index
internal/backend/loadout*.go         Loadout parsing, calculation, sharing, and transactions
internal/backend/save_*.go           Save discovery, backup, parsing, and mutation
internal/backend/sigil_*.go          Sigil save/live channels and shared catalog
internal/backend/wrightstone_*.go    Wrightstone save/live channels and shared catalog
internal/backend/summon_*.go         Summon catalog, save, and live channels
internal/backend/runtime_*.go        Runtime patches, monitors, guards, and readback
internal/backend/formula_*.go        Panel location, sampling, and redacted evidence
internal/backend/*_test.go           Colocated backend regression tests; not shipped
internal/backend/data/               Embedded 2.0.2 catalogs, layouts, and evidence
internal/backend/resources/          Embedded native helper
frontend/                             Vue application, generated bindings, and UI tests
src_dll/                              Reproducible patch_core native source
tools/                                Reproducible maintainer scripts only
docs/                                 User, architecture, status, and evidence documentation
.github/workflows/ci.yml              Go, frontend, and static checks
```

Only reproducible release/data maintenance tools are kept online. One-off field QA scripts, machine-specific files, credentials, handoff bundles, and local screenshots stay outside the repository. `*_test.go` and `*.test.js` files are automated verification maintained beside the features they protect; they are not user-facing scripts and are not compiled into the release. See the [architecture guide](docs/ARCHITECTURE.md) and [backend file map](internal/backend/README.md).

## Build and verify on Windows

Requirements: Windows amd64, Go 1.25+, Node.js/npm, Wails CLI v2.13, and WebView2 Runtime. Visual Studio/MSBuild is needed only to rebuild `src_dll/patch_core`.

```powershell
cd frontend
npm ci
npm run build
cd ..

go test ./...
go vet ./...
node --test frontend/src/*.test.js
wails build -platform windows/amd64 -clean
```

The executable is written to `build\bin\GBFR PE Patch Tool.exe`. Before publishing, launch the packaged executable and verify its version, main pages, release link, and SHA-256.

## Use boundary and references

This project is for learning and personal local use. It is not affiliated with, sponsored by, or authorized by Cygames, SEGA, the game's publishers, or the community authors mentioned below. Save, executable, and runtime changes can damage data, lose progress, or trigger the game's own validation. Work only with files you are entitled to use, keep recoverable backups, and accept responsibility for the result. Do not package this project as a paid modification service or use it to affect other players online.

The repository does not contain, mirror, bypass, or resell third-party paid tables, membership content, or restricted downloads. Runtime patches are released only with DLC 2.0.2 executable identity, signatures, original bytes, unique-match checks, writeback, and field evidence. Public community demonstrations are used only to compare feature concepts and testing approaches; they are neither runtime dependencies nor download sources.

For reproducibility, the following links identify public material that was used only for cross-checking; they do not imply collaboration, authorization, copied implementation, or endorsement. Early save and sigil record notes can be found in [BitterG's public project](https://github.com/BitterG) and [public page](https://b23.tv/uRLYpW8). Loadout interaction was compared with public work by [意地悪い骷髅](https://b23.tv/xhiZ7fm) and the [loadout simulator](https://lib.kannanote.top/%e7%a2%a7%e8%93%9d%e9%85%8d%e8%a3%85%e6%a8%a1%e6%8b%9f%e5%99%a8/). Chinese terminology was checked against public material by [LKong621](https://b23.tv/mnwxgDf); data extraction used public tools by [Nenkai](https://github.com/Nenkai); and summon warnings were compared with public notes from [SinnohDawn](https://b23.tv/lKSX4zy) and [Relink Summon](https://relinksummon.fate-go.top). Other public demonstrations were used only to compare feature names and test scenarios. This repository provides no related paid content, download route, or reconstructed material.
