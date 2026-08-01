# Virtual-sigil 2.0.3 crash audit

Date: 2026-08-01  
Scope: read-only post-mortem of the locally captured game crash, the embedded native virtual-sigil runtime, and its owner/restore contract. The game process was not controlled or modified during this audit. The dump and runtime files remain local and are not repository artifacts.

## Proven failure

The Windows Error Reporting dump `granblue_fantasy_relink.exe.19156.dmp` is 65,872,329 bytes with SHA-256 `7E456FE09DA1E0B1ADEE56B986D4510036F876382A0E73490694BF6B4004F74A`.

- The virtual-sigil status entered `active` at 14:19:05 for the same PID/creation identity.
- The game dump was written at 14:19:20.
- The exception is `0xC0000005` at `0x7FF7B7EC005A`, attempting to write address `0x1`.
- The instruction pointer is outside every loaded module and inside a captured executable-memory range beginning at `0x7FF7B7EC0000`.
- That range begins with the exact virtual-sigil trait-fetch cave emitted by `InstallVirtualTraitFetchHook`.

The relevant captured cave bytes disassemble as:

```text
cave+00  cmp  r13d, 0x0D
cave+04  jb   cave+18
cave+06  cmp  r13d, 0x11
cave+0A  jae  cave+18
cave+0C  mov  rax, originalCallPath
cave+16  jmp  rax
cave+18  test bl, bl
cave+1A  je   cave+5A
cave+1C  mov  rax, [r15+0x5E80]
cave+23  jmp  original+0x0B
cave+28.. 00 00 ...
cave+5A  00 00                 ; faulting `add byte ptr [rax], al`
```

`kTraitFetchOriginal` contains `84 DB 74 3E ...`: the `JE +0x3E` is a short relative branch whose destination is valid only at the original game address. Copying it verbatim into the cave redirects it into the cave's zero-filled tail. This is the direct cause of the reported crash; it is not an unverified RVA guess.

The active configuration used four declared virtual slots and only one non-empty physical instance. The crash therefore does not require eight slots, duplicate SlotIDs, or a malformed entry-count boundary.

## Required native control-flow contract

The fix must relocate semantics rather than bytes:

```text
if slot is virtual:
    jump originalCallPath
else:
    test bl, bl
    if bl == 0: jump originalCallPath
    mov rax, [r15+0x5E80]
    jump originalTraitFetch + 11
```

Concretely, the cave must explicitly encode the `bl == 0` route with a freshly calculated branch or an absolute `mov/jmp`. It must never `memcpy` the original short conditional branch. Both the virtual-slot route and the relocated native `bl == 0` route must reach `traitFetchCallRva`.

The red-capable source contract is:

```powershell
go test ./internal/backend/virtual_sigil_cave_contract_test.go -run '^TestVirtualSigilTraitFetchCaveDoesNotCopyRelativeBranch$' -count=1 -v
```

Before the fix it deterministically fails with:

```text
virtual-sigil trait-fetch cave copies the original short relative JE verbatim; in the real 2.0.3 crash dump JE +0x3E landed in zero-filled cave memory and raised 0xC0000005
```

## 2.0.3 executable-site verification

Against the locked 2.0.3 executable identity (`SHA-256 1BBBEC61AAB7F75FE328CF6BFE0247EBDBCEC6C404CEC12C032B8FFA41D22102`), the following sites and complete original byte windows pass:

| Site | 2.0.3 RVA | Result |
|---|---:|---|
| trait apply loop | `0xA1EBE4` | exact preflight match |
| trait category loop | `0xA1F7F6` | exact preflight match |
| trait fetch | `0xA1F80E` | exact 11-byte match |
| gem getter | `0xA25D70` | exact 12-byte prologue match |

The opt-in command is:

```powershell
$env:GBFR_GAME_EXE_203_TEST='path-to-locked-2.0.3-exe'
go test ./internal/backend -run '^TestVirtualSigil203NativeSitesMatchLockedExecutable$' -count=1 -v
```

It passed all four subtests. The fixed 2.0.3 RVAs are therefore not the cause of this crash.

## Implemented fix and live verification

The native cave now emits the control flow above explicitly: virtual slots and native `bl == 0` both use an absolute jump to the original call path, while the other native route loads `[r15+0x5E80]` and returns to `traitFetch + 11`. It no longer copies the original relative `JE`. The complete virtual lookup/output path is also protected by SEH so a scene-owned page disappearing during a read falls back without crashing the game.

One additional lifecycle failure was reproduced by terminating only the owning desktop application while camera, audio, and virtual-sigil companions were active. Camera and audio restored, but a read-only recovery scan found the old virtual gem-getter entry still detoured while the other three virtual sites were original. The individual runtimes are separate DLL copies and their watchdogs could suspend and patch the same process concurrently during owner-death restoration. `PatchBytes` now serializes every executable-byte publication and restoration through the process-scoped mutex `Local\\GBFRPatchCoreBytes-<PID>`, while retaining thread suspension, write readback, instruction-cache flush, and verified status reporting.

The fixed embedded DLL builds successfully. Full Go tests, Go vet, staticcheck, the 2.0.3 locked executable contracts, and the full frontend suite pass. A live enable of one virtual sigil remained stable beyond the original 15-second crash window. The currently running game process still contains the single getter detour left by the pre-fix DLL; it is held in `restore_failed` and the new application correctly refuses to layer another generation over it. A normal game process restart is required once to discard that old-process residue before the final enable/owner-death/reattach acceptance with the fixed DLL.

## Remaining lifecycle findings

Ranked after the proven P0 failure:

1. **P1 — scene-owned pointer race:** `GetGemDataDetour`, `ResolveOwnedStatus`, and `FindInventoryGem` validate pages and then dereference them without an SEH boundary. A scene transition can free or repurpose a page between validation and access. Wrap the complete virtual-slot read path in `__try/__except`, return no virtual item on a fault, and keep the original path untouched.
2. **P1 — incomplete context coverage:** the 2.0.3 original getter handles context modes `0`, `1`, `2`, `4`, and `5`; the virtual detour rejects everything above `2`. That is a likely cause of virtual slots disappearing in some menus/scenes. Modes `4/5` need an independently verified ownership rule before being enabled; merely widening the integer check is not enough.
3. **P2 — allocation cleanup:** the 96-byte trait-fetch cave is not released after verified restoration. It is unreachable after original bytes are restored and is not the crash source, but repeated enable/disable cycles leak executable pages until the game exits.
4. **No direct Hook overlap found:** virtual-sigil sites are distinct from camera, Wwise audio, damage, and QOL targets. Complete original-byte preflights make an existing third-party Hook fail closed rather than silently chain. The crash dump contains the virtual cave itself, not another runtime's detour.
5. **Owner/restore contract is structurally present:** PID + creation time + generation ownership, callback draining, status states, original-byte restoration, and owner release are implemented. The page components do not disable camera/audio/virtual-sigils on navigation; only an explicit disable, owner death, game exit, or application shutdown initiates restoration. The stale `active` status after this game crash is expected evidence of an abrupt target-process death and is rejected against a later PID/creation identity.

## Version-gate audit note

At the start of this audit the UI validator still required literal `2.0.2` values for party snapshots, spatial writes, gravity, arrow-key movement, and selected-item capture. Their backends had already gained versioned layouts, so that validator produced the visible `party game version must equal 2.0.2` failure on a valid 2.0.3 snapshot. These response `gameVersion`, `source`, and gravity-RVA contracts must be selected from the detected layout end to end; changing only the displayed label is insufficient.
