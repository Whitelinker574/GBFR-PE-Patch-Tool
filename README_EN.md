<p align="center">
  <img src="build/appicon.png" width="112" alt="GBFR PE Patch Tool" />
</p>

<h1 align="center">GBFR PE Patch Tool</h1>

<p align="center">
  A Windows tool for <em>Granblue Fantasy: Relink</em> DLC 2.0.2: edit saves and loadouts, change live game data, apply single-player patches, and monitor final character stats.
</p>

<p align="center">
  <a href="https://github.com/Whitelinker574/GBFR-PE-Patch-Tool/releases/latest"><strong>Download v1.91.12</strong></a> ·
  <a href="README.md">简体中文</a> ·
  <a href="docs/README.md">Documentation index</a>
</p>

<p align="center">
  <a href="https://github.com/Whitelinker574/GBFR-PE-Patch-Tool/actions/workflows/ci.yml"><img src="https://github.com/Whitelinker574/GBFR-PE-Patch-Tool/actions/workflows/ci.yml/badge.svg" alt="CI" /></a>
</p>

> Supported game version: DLC 2.0.2. Keep backups before editing and use live features only in local single-player sessions.

## Choose the right workspace

| What you want to do | Open | Game state |
| --- | --- | --- |
| Add sigils, wrightstones, items, weapons, or summons in bulk | `Save Editing (Offline)` | **Game fully closed** |
| View or write a character's saved loadout presets | `Save Editing (Offline)` → `Loadout Presets` | **Game fully closed** |
| Edit the sigil, wrightstone, or summon currently selected in game | `Live Injection` | **Game running and save loaded** |
| Enable guard, character-mechanic, or quest-convenience patches | `Live Injection` → the matching patch page | **Single-player content loaded** |
| Inspect party state or sample final character stats | `Memory Monitoring (Read Only)` | **Game running in a stable scene** |
| Check versions, back up an EXE, or restore it | `Tools & Settings` | Follow the page instructions |

Offline, live, and read-only are separate workflows. Offline pages edit save files, live pages write to the current game process, and read-only pages do not change character, item, or save data. The formula sampler only queries and reads memory; selected-item capture in Runtime Monitor temporarily installs a read-only address hook and restores the original bytes on safe disconnect or page exit.

## Download and first run

1. Download the Windows amd64 archive from [Releases](https://github.com/Whitelinker574/GBFR-PE-Patch-Tool/releases/latest), extract it, and run `GBFR PE Patch Tool.exe`.
2. Choose a workspace from the table above. Close the game for offline editing; launch the game and load a save for live or read-only tools.
3. Before writing, confirm the save slot, character, item, and destination slot. After success, verify the page readback or the result in game.
4. Before editing a save, use `Save Protection` at the top right to create a restore point. Before changing the game EXE, create a `.bak` in Game File Maintenance. If an error appears, stop writing and use [Troubleshooting](#troubleshooting).

Default save directory:

```text
C:\Users\YOUR_NAME\AppData\Local\GBFR\Saved\SaveGames\
```

![The four workspaces on the home page](docs/screenshots/home.png)

**What to look for:** the four workspaces in the sidebar determine whether the tool uses offline writes, live writes, or read-only access. Choose the workspace from the current game state, then open the task page from the home cards.

## Complete feature list

### Save Editing (Offline) · 7 pages

Fully close the game before using these pages. The tool lists save slots 1 / 2 / 3 separately and also supports manual file selection; do not assume that the first slot is the one you want.

| Page | What it does | Basic flow |
| --- | --- | --- |
| Loadout Presets | View saved weapons, 12 sigils, 4 skills, and mastery; write a custom loadout | Choose save → character → view, or select `Edit [Character] Loadout` → destination slot |
| Sigil Save Editor | Generate (add), batch-manage, and remove sigils | Configure the sigil and traits; combination checks warn but do not change your choice |
| Items & Weapons | Edit items, materials, progression resources, and weapon levels | Use for batch changes; check the automatic backup and readback |
| Wrightstone Save Editor | Generate (add) a wrightstone with three traits | Confirm all traits, level warnings, and the pending write |
| Summon Save Editor | Add or fully edit summons in a save whose summon system was opened by the game | The tool cannot force-open that system; reopen and verify after writing |
| Character Usage | View and batch-edit usage counts for selected characters | Only checked characters are saved |
| Quest & Title Records | Edit quest completion counts and title unlock/viewed state | Title reward-claim state is left unchanged |

### Live Injection · 10 pages

Launch the game, load a save, then connect from the page. Reconnect or reselect the target after reloading a save, restarting the game, or refreshing an in-game list.

| Page | What it does | Basic flow |
| --- | --- | --- |
| In-Game Live Editor | Edit currency, consumables, material consumption, and quest drops | Connect and choose a resource or task category; reconnect after restart |
| Live Sigil Editor | Edit the sigil currently selected in game | Open the in-game sigil list and select it → return, refresh, and verify → write |
| Live Wrightstone Editor | Write all three traits of the current wrightstone in one transaction | Open the wrightstone list and select it; select it again after every write |
| Live Loadout Capture / Replay | Export 12 equipped sigils or replay them onto spare sigils | Start at the first item and move one by one; spare sigils are overwritten |
| Summon Editor | Edit a summon's sigils, secondary parameters, and level | Open the summon inventory and select the target; this page does not provide a safe summon-type write |
| Over Mastery | Read and edit the four result slots | Complete one level-3 roll, remain on the result screen, then refresh and save |
| Combat Rule Patches | Control dodge, guard, Link, summon limits, and part-break patches | Single-player only; shares one persistent connection with the next two pages |
| Character Mechanic Patches | Enable character-specific mechanics and view conflicts | Restore an active conflicting feature before switching |
| Quest & Convenience Patches | Control timers, chests, results, side rewards, and progression conveniences | Single-player only; refresh after the quest state changes |
| Monster Multipliers & Damage Log | Adjust monster multipliers and super armor, and record damage experiments | Experimental; inspect current state and change one item at a time |

The combat, character, and quest patch pages share one persistent connection. Switching pages keeps active patches; `Restore All & Disconnect` or application shutdown restores them together.

### Memory Monitoring (Read Only) · 2 pages

| Page | What it does | Basic flow |
| --- | --- | --- |
| Runtime Monitor | Inspect the player, three party members, Vyrn, and the currently selected material or key item | Connect in a stable scene; selected-item capture restores its hook on safe disconnect or page exit |
| Character Formula Sampler | Continuously read final HP, attack, critical rate, and stun; record one-variable A/B/A/B evidence | Change one item per round and wait for stable values; no process or save writes |

### Tools & Settings · 3 pages

| Page | What it does | Basic flow |
| --- | --- | --- |
| Compatibility | Show the tool version, game file, and feature support state | Do not force a patch on an unidentified executable |
| Language & Display | Change the interface language | The application refreshes; the preference is local only |
| Game File Maintenance | Identify the game EXE, create a `.bak`, and restore it | Create an original backup before applying file patches |

## Three common workflows

### Edit a save, sigil, or loadout

1. Fully close the game.
2. Open `Save Editing (Offline)` and choose the task page.
3. If several slots appear, inspect each and confirm the character or content. Use `Browse...` to select a file manually.
4. Choose the target character, item, or slot and review the change.
5. Write only after confirming the destination; check that backup and readback succeeded.
6. Launch the game and verify. If the result is wrong, stop overwriting, fully close the game, open `Save Protection` at the top right, and restore the pre-write point.

![Three save slots and saved loadouts](docs/screenshots/loadout-presets.png)

**What to look for:** `Save Slot 1 / 2 / 3` are separate saves, `Browse...` selects a file manually, and `Refresh` reloads the current file. After choosing the save and character, open the edit action on the right; the selected destination preset is overwritten.

### Use a live editor or single-player patch

1. Launch the game and load the target save; enter single-player content for patches.
2. Open `Live Injection`, choose a page, and connect to the game.
3. For a sigil, wrightstone, or summon, select the item in game before refreshing it in the tool.
4. Modify only the confirmed target. Reselect after a save reload or list refresh.
5. Switch freely among the three patch pages; the shared connection and active state persist.
6. Select `Restore All & Disconnect` when finished. A game restart also clears live state.

![Combat rule patch page](docs/screenshots/patch-combat.png)

**What to look for:** the connection card at the top is shared by all three patch pages; each feature card shows its state and readback. Select `Connect to Game`, enable one feature at a time, and finish with `Restore All & Disconnect`.

### Read final character stats and collect evidence

1. Launch the game and open a stable equipment or training scene.
2. Open `Memory Monitoring (Read Only)` → `Character Formula Sampler`, choose the current character, and connect.
3. Wait for final HP, attack, critical rate, and stun to stabilize, then record the baseline.
4. Change exactly one reversible item in game, wait for the new stable state, and stop/analyze.
5. Use A1/B1/A2/B2 for strict validation. Insufficient evidence remains labelled candidate, negative observation, or open.

![Character formula sampler](docs/screenshots/formula-sampler.png)

**What to look for:** choose the current character at the top right and select `Connect Read-Only Sampler`; after connection, verify the game version, character, and four final values. Select `Start Recording Changes` for the normal workflow; the A/B/A/B section below is for stricter repeated validation.

## Troubleshooting

| Symptom | First action |
| --- | --- |
| No save is detected | Check the default directory or use `Browse...` to select `SaveData*.dat` |
| Three saves appear and the right one is unclear | Do not guess by order; inspect the character, loadout, or record shown for each slot |
| A live page says disconnected | Make sure the game is running with a save loaded, then reconnect from the current page |
| The target expired, a pointer is null, or the record changed | Select the target again in game, return to the tool, refresh, and only then write |
| Connection state is unclear after switching patch pages | The three patch pages share one connection; check the top connection card instead of starting another session |
| Game version or EXE is not recognized | Stop applying file patches; open `Tools & Settings` → `Compatibility`, confirm DLC 2.0.2, and restore files through Steam if needed |
| The save has not opened summons or mastery | The tool does not unlock game systems; open the feature through normal game progression first |
| A combination warning appears | Warnings do not block the write, but the game may reject, clean, or hide the result; keep a backup |
| The saved result is wrong | Stop writing and fully close the game; open `Save Protection` → choose the pre-write point → `Restore This Point` |
| An EXE patch must be removed | Use `Tools & Settings` → `Game File Maintenance` to restore `.bak`, or verify files through Steam |

## Data and accuracy

- Save, live, and loadout editors share the DLC 2.0.2 sigil, wrightstone, and summon catalogs. Catalog parity does not mean that the game accepts every combination.
- A single-loadout file copies the weapon and its progression, awakening/transcendence and wrightstone, 12 independent sigils, skills, mastery selections and Master Level progress, permanent character-enhancement progress, and the global summon selection. Missing summons are created automatically. System unlock state and character Over Mastery stay unchanged in the target save; a missing matching weapon blocks a partial write.
- The loadout page separates verifiable progression, weapon, sigil, mastery, Over Mastery, and summon contributions.
- Values without sufficient field evidence remain labelled estimate, candidate, negative observation, or open—not final formulas.
- Read-only exports remove PIDs, module bases, absolute addresses, user names, and local paths. They do not export full process memory.

See [formula evidence](docs/FORMULAS_2.0.2.md), the [sampling guide](docs/角色公式采样操作说明.md), [save/live catalog parity](docs/evidence/save-memory-table-parity.md), and [implementation status](docs/IMPLEMENTATION_STATUS.md).

## Documentation and development

- [Documentation index](docs/README.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Backend file map](internal/backend/README.md)
- [Maintainer scripts](tools/README.md)
- [Third-party notices](THIRD_PARTY_NOTICES.md)

<details>
<summary><strong>Repository layout and local build</strong></summary>

```text
internal/backend/   Go backend: saves, runtime access, catalogs, and tests
frontend/           Vue UI, frontend tests, and Wails bindings
src_dll/            Reproducible patch_core native source
tools/              Data audits, catalog generation, and release scripts
docs/               Guides, architecture, screenshots, and redacted evidence
build/              Wails metadata, icons, and local build output
.github/workflows/  Continuous integration and release checks
```

Requirements: Windows amd64, Go 1.25+, Node.js/npm, Wails CLI v2.13, and WebView2 Runtime. Visual Studio/MSBuild is required only to rebuild `src_dll/patch_core`.

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

The executable is written to `build\bin\GBFR PE Patch Tool.exe`.

</details>

## Reporting an issue

Open an [Issue](https://github.com/Whitelinker574/GBFR-PE-Patch-Tool/issues) with the tool version, game version, affected page, selected save slot, reproduction steps, and actual result. Attach only redacted page state or an exported evidence bundle; do not upload a real save, PID, absolute address, user name, or screenshot containing a personal path.

## Use boundary, provenance, and third parties

This unofficial project is for learning and personal local use. It is not affiliated with, sponsored by, or authorized by Cygames, SEGA, the game's publishers, or the community authors listed below. Save, executable, and runtime changes can damage data, lose progress, or trigger the game's own validation. Work only with files you are entitled to use, keep recoverable backups, and accept responsibility for the result. Do not package this project as a paid modification service or use it to affect other players online.

This repository does not declare a project-wide open-source license covering all inherited code. Except for third-party components carrying explicit licenses, public visibility alone does not grant permission to copy, redistribute, or use the project commercially. The repository does not contain, mirror, bypass, or resell third-party paid tables, membership content, or restricted downloads.

<details>
<summary><strong>Provenance and public references</strong></summary>

This project was originally forked from [BitterG/GBFR-PE-Patch-Tool](https://github.com/BitterG/GBFR-PE-Patch-Tool). Its early save parsing, sigil generation, and wrightstone generation followed that public codebase; the upstream README records additional method provenance involving tools by Xzire91x and Nenkai. The current repository has since been rewritten and extended. This provenance statement does not imply endorsement, authorization, or participation in the current release by the original authors.

Other public material was used only for cross-checking: loadout interaction was compared with work by [意地悪い骷髅](https://b23.tv/xhiZ7fm) and the [loadout simulator](https://lib.kannanote.top/%e7%a2%a7%e8%93%9d%e9%85%8d%e8%a3%85%e6%a8%a1%e6%8b%9f%e5%99%a8/); Chinese terminology was checked against public material by [LKong621](https://b23.tv/mnwxgDf); data extraction used public tools by [Nenkai](https://github.com/Nenkai); and summon warnings were compared with notes from [SinnohDawn](https://b23.tv/lKSX4zy) and [Relink Summon](https://relinksummon.fate-go.top). These links do not imply collaboration, authorization, copied implementation, or endorsement.

Bundled open-source and native components are documented in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

</details>
