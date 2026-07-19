[简体中文](README.md)

# GBFR Data Tools — Granblue Fantasy: Relink 2.0.2

A bilingual English/Chinese version of BitterG's **GBFR PE Patch Tool** for **Granblue Fantasy: Relink** and **Endless Ragnarok**.

> Open the **Language** tab to switch between English and Simplified Chinese. The preference is stored locally and restored on the next launch. Some newly added specialist loadout terminology still remains in Chinese and the app states that limitation explicitly.

## Language support

- Switch between **English** and **Simplified Chinese** from the Language tab.
- Core navigation and the mature generators are bilingual; some new loadout/mastery terminology is not translated yet.
- The selected language is saved locally and restored automatically.

## Features

### Save data tools

- **Sigil Generator** — Search for sigils, configure sigil and trait levels, and write them to an output save.
- **New Sigil Memory Editor** — Read the sigil currently selected in-game and edit its sigil, primary trait, secondary trait, and levels.
- **Wrightstone Generator** — Configure a wrightstone and its three traits, with queue support for batch generation.
- **Selected Wrightstone Memory Editor** — Read and transactionally update the selected three-trait record; successful writes automatically restore the hook.
- **Single-character Loadouts** — Edit one character build with weapon, 12 sigil slots, four skills, mastery paths, summons, and single-build JSON import/export.
- **Quest Clear Statistics** — Scan save slots and display quest clear counts and save summaries.
- **In-place Save Editing** — Optionally overwrite the selected input save directly. Back up the save first.

### EXE patches

- **Quest Clear Count** — Change the quest-clear count limit without editing the save.
- **Commendation Count** — Change the value awarded when receiving a commendation; this can affect save data.
- **Automatic Detection** — Locate the game executable from Steam registry and library paths.
- **Backup and Restore** — Create and restore a `.bak` copy of the game executable.

### Runtime tools

- **Character Usage Counts** — View and edit per-character quest usage counts.
- **Currency and Potion Editors** — Read and write supported currencies and potion counts through stable pointer paths.
- **No Material Consumption** — Temporarily prevent upgrade, enhancement, and transmutation material quantities from decreasing.
- **Guaranteed Terminus Weapon Drop** — Removes the 80% exclusion check for Terminus Weapon lots while preserving ownership and character-unlock checks.
- **Over Mastery Editor** — Scan, refresh, edit, and save Over Mastery values.
- **Monster Enhancements** — Controls for monster HP, damage, stun gauge, Overdrive state, SBA chain timing, Link Time, and related gauges. Some items are currently marked as not fixed.
- **Update Check** — Check GitHub Releases for newer versions.

### Compatibility lab

Quest-result countdown, infinite challenges, face/alternate-skin runes, unlock-all-titles, team damage metering, score multipliers, forced sub-objective rewards, and mission multipliers are retained only as legacy/experimental material. They have not completed production acceptance against game 2.0.2 and are labelled accordingly in the UI.

## Safety notes

1. Back up your save files before writing changes.
2. Back up `granblue_fantasy_relink.exe` before applying EXE patches.
3. The in-place save option directly overwrites the selected save.
4. Runtime-memory features require the game to be running and may require administrator privileges.
5. Host-side runtime changes can affect other players. Tell teammates before using them in multiplayer.

Default save location:

```text
C:\Users\YOUR_NAME\AppData\Local\GBFR\Saved\SaveGames\
```

## Building on Windows

Requirements:

- Go 1.23 or newer, amd64
- Node.js and npm
- Wails CLI v2.12.0
- Microsoft Edge WebView2 Runtime
- Visual Studio / MSBuild only when rebuilding `src_dll/patch_core`

Install Wails:

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0
```

Build the application:

```powershell
.\build-windows.bat
```

The executable is generated at:

```text
build\bin\GBFR PE Patch Tool.exe
```

When `src_dll/patch_core` is modified, build the DLL as **Release x64** first. Its post-build step publishes the result to the Git-tracked stable embed source `resources\patch_core.dll`; `build\bin` contains disposable Wails output only.

## Translation scope

The English localization currently covers:

- Main navigation and tabs
- Sigil and Wrightstone generators
- New selected-sigil memory editor
- Save and quest statistics
- Character usage statistics
- Miscellaneous runtime tools
- Monster enhancement controls
- Over Mastery interface
- Most established user-facing status and error messages
- Character, Sigil, Trait, and Wrightstone display names
- New Endless Ragnarok sigils and character-specific traits used by the memory editor

The expanded loadout, mastery, summon, and final-stat editor still contains Chinese specialist terms. Until those strings are fully catalogued, English mode is intentionally described as partial rather than complete.

The Chinese lookup tables remain in the source as reference data, but the English build returns the original English catalog names.

## Attribution

- Original project and program logic: **BitterG**
- English translation: **FionaAleksic**
