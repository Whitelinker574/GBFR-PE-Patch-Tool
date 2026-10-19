# 2.0.6 shared HP and OD paths

Read-only audit against master `6da91f0`, 2026-10-19. No production source,
game process, or GitHub state was changed for this investigation.
Addresses are executable-relative RVAs, image base `0x140000000`.
The local executable SHA-256 is
`4B3ACE0FD03DF5D3D08C6453E12DF2279C6259ABF7D2FB3622921FCBEA775145`.

## Conclusion

Both upstream locations and complete instruction signatures match the local
2.0.6 executable uniquely. They add genuinely different register layouts;
neither can reuse the existing hook emitter unchanged. The upstream shared
OD emitter corrupts nonvolatile `XMM6`. Integrate the locations, not that
emitter. Keep each new path an auxiliary of its existing public control, with
the existing ownership, rollback, restoration, and executable-digest gates.

The upstream labels identify these as the Seofon/Tweyen shared paths. Static
instruction matching confirms the locations and operations, but does not by
itself establish which encounters execute them or enemy-only coverage.

## Primary references

- [Upstream HP-path commit 15e7df1f](https://github.com/BitterG/GBFR-PE-Patch-Tool/commit/15e7df1f):
  introduces the alternate HP entry, a separate hidden patch item, and its
  emitted scaling code. Its emitter scales signed `EBX`, writes the result
  back through `EBX`, and resumes after six original bytes.
- [Upstream OD-path commit a0846609](https://github.com/BitterG/GBFR-PE-Patch-Tool/commit/a0846609):
  adds the shared OD path and an emitter using `XMM6` without preservation;
  its existing-jump test accepts any `E9`. These behaviors should not be copied.
- [Microsoft x64 calling convention](https://learn.microsoft.com/en-us/cpp/build/x64-calling-convention?view=msvc-170):
  `XMM6..XMM15` and `RBX/RSI/RDI` are nonvolatile; `XMM0..XMM5` are volatile.
- Local executable disassembly and PE unwind-function boundaries, reproduced
  by the read-only external helper `field-tools/issue26_native_probe.py`.
- Current implementation: `src_dll/patch_core/dllmain.cpp`,
  `internal/backend/monster_od_206.go`, `monster_enhance_safety.go`, and
  `app.go`. These files already have the multi-auxiliary recovery structure.

## Shared OD: exact native contract

Containing function: `0x3AB63B0..0x3AB64BA`. The function saves `RSI`, `RDI`,
and `RBX`, reserves `0x20` stack bytes, and never saves/restores `XMM6`.
At `0x3AB63DC` it copies incoming `RDX` into `RSI`. At the hook site, `RAX`
is the resolved OD component and must remain a valid pointer.

```text
3AB645A  80 78 50 00             cmp byte ptr [rax+50h],0
3AB645E  74 13                   je 3AB6473
3AB6460  48 03 70 18             add rsi,[rax+18h]
3AB6464  48 C7 C1 FF FF FF FF    mov rcx,-1
3AB646B  48 0F 43 CE             cmovae rcx,rsi
3AB646F  48 89 48 18             mov [rax+18h],rcx
3AB6473  48 83 C4 20             add rsp,20h
3AB6477  5B                      pop rbx
3AB6478  5F                      pop rdi
3AB6479  5E                      pop rsi
3AB647A  C3                      ret
```

The 25-byte prefix plus eight-byte epilogue occurs once, at `0x3AB645A`.
The update's `CMOVAE` consumes the **unsigned carry flag from ADD**: on carry
the stored value is `UINT64_MAX`; otherwise it is the sum. Neither signed
overflow nor a guessed zero clamp is equivalent. `MOV` does not modify flags.
Do not put stack `ADD/SUB`, `TEST`, or a comparison between the original ADD
and CMOVAE. A negative-delta interpretation is not proved by these bytes.

### Smallest integration

Prefer overwriting the 19-byte update body at `0x3AB6460`, retaining the
original component-enabled `CMP/JE` and original epilogue. Expected body:

```text
48 03 70 18 48 C7 C1 FF FF FF FF 48 0F 43 CE 48 89 48 18
```

The cave preserves `XMM0`, scales `RSI` with the same explicit OD scale policy
as the current feature, restores `XMM0` and any temporary stack allocation,
then emits the original ADD/MOV/CMOVAE/STORE sequence and jumps to `0x3AB6473`.
The stack restoration must occur **before** the original ADD. `RAX` stays
unchanged. Do not mutate `XMM6`. A 128-byte cave has ample space including
the existing eight-byte ownership marker at its end.

Hooking the full 25 bytes at `0x3AB645A` is also possible, but then the cave
must reproduce the guard and branch to the correct original epilogue, not
reuse the `[RSI+50h]`/`RDI` emitter. Choosing the 19-byte body makes a stale
upstream full-prefix hook fail the preserved-guard preflight instead of being
mistakenly adopted.

## Alternate HP: exact native contract

Containing function: `0x3AB7850..0x3AB7971`. At entry it copies `R8D` into
`EDI`, copies `EDX` into `EBX`, and preserves `R14/RSI/RDI/RBX`. At the hook
site, `RAX` is the resolved health component and `EBX` is incoming uint32
damage. The original zero-extension is important.

```text
3AB7920  89 D9                   mov ecx,ebx
3AB7922  48 8B 50 10             mov rdx,[rax+10h]
3AB7926  45 31 C0                xor r8d,r8d
3AB7929  48 29 CA                sub rdx,rcx
3AB792C  4C 0F 43 C2             cmovae r8,rdx
3AB7930  B9 01 00 00 00          mov ecx,1
3AB7935  49 0F 47 C8             cmova rcx,r8
3AB7939  85 FF                   test edi,edi
3AB793B  49 0F 44 C8             cmove rcx,r8
3AB793F  48 89 48 10             mov [rax+10h],rcx
3AB7943  48 89 F1                mov rcx,rsi
3AB7946  48 83 C4 38             add rsp,38h
3AB794A  5B                      pop rbx
3AB794B  5F                      pop rdi
3AB794C  5E                      pop rsi
3AB794D  41 5E                   pop r14
3AB794F  E9 7C C9 C5 00          jmp 47142D0
```

The 35-byte HP sequence occurs once, at `0x3AB7920`. Overwrite only its first
six bytes and resume at `0x3AB7926`. That continuation's XOR/SUB replaces
incoming flags, so scaling/clamping comparisons in the cave do not need to
preserve entry flags. The later CMOVAE/CMOVA depend on SUB's carry/zero flags;
the cave should let the untouched native sequence manage them.

Native semantics are unsigned saturating subtraction, with the `EDI != 0`
mode imposing a minimum remaining HP of one. For example, HP 100/damage 100
produces zero when EDI is zero and one when EDI is nonzero. The final tail
releases a lock through `0x47142D0`; never replace it with an immediate RET.

### Safe scaling strategy

Preserve `XMM0`; execute `MOV ECX,EBX` to zero-extend damage; convert **RCX as
int64**, apply the existing HP reciprocal scale, and convert back into RCX.
Restore `XMM0`/stack, emit `MOV RDX,[RAX+10h]`, then jump to `0x3AB7926`.
Keep `RAX`, `RBX`, `EDI`, `RSI`, and `R14` unchanged. No R11 scratch is needed.

The upstream signed-EBX conversion misinterprets damage >= `0x80000000` as
negative, and its final write via `R11D -> EBX` truncates a wider scaled
result. Its EBX mutation is unnecessary, but is **not itself a proven ABI
violation here**, since the containing function eventually pops saved RBX.
Using RCX directly avoids both the local mutation and signed-int32 problem.
Even `UINT32_MAX * 9999` fits int64 for the supported scale domain.

Define numerical edges deliberately: zero damage should stay zero; if the
current positive-damage minimum-one policy is retained, apply it only to
positive original damage. Float32 precision/rounding at large inputs and
scale exactly one need explicit tests or a native passthrough special case;
do not infer exact integer identity from floating conversion.

## Ownership and version integration

1. Add shared OD as a **third auxiliary** for `od_rate`, making four total
   sites with the main entry. Existing `odInline206Sites` must gain per-site
   expected bytes because the new body uses RSI/RAX, unlike the old RDI/RSI
   bodies. Preflight its preserved six-byte guard and full epilogue too.
2. Add alternate HP as one auxiliary under the existing `monster_hp` item,
   making two total sites. Do not add another public/hidden top-level patch
   identity merely to manage it. Keep the existing reciprocal HP parameter.
3. Gate both additions to the exact verified 2.0.6 digest; older verified
   versions keep their audited site sets. Require full-signature uniqueness
   in local-EXE tests; do not assume a compatible six-byte prefix elsewhere.
4. Preflight every site before installation; snapshot original bytes before
   injection. Reuse `AuxTarget`/`ExtraAux` for normal claim, partial-failure
   rollback, owner shutdown, detach, and restart adoption. Add the HP auxiliary
   to restart adoption explicitly; OD's existing Additional loop can cover
   its new auxiliary if per-site originals are supplied correctly.
5. Stamp the existing marker at the correct cave end. An `E9` alone is not
   ownership. Preserve changed-entry refusal and recovery metadata after any
   failed restore. The DLL's already-installed-main branch must also ensure
   all associated auxiliaries, not skip them.
6. Check rel32 bounds for both entry and return jumps. Free a failed cave
   only after proving that no live entry points to it; retain the current
   safe-write/readback and other-thread suspension discipline.

## Required assertions

- Exact local-EXE signatures, unique match RVAs, instruction boundaries,
  preserved OD guard, and complete native tails; same digest as this audit.
- Emitted cave disassembly has no XMM6 write, balanced stack movement,
  correct marker/data offsets and rel32 targets, preserved RAX pointer and
  saved registers, and no flag-changing instruction between OD ADD/CMOVAE.
- Execute emitted caves in a synthetic process harness, not only string-match
  source comments. Seed XMM0/XMM6 and nonvolatile GPRs with sentinels; verify
  sentinel preservation, native return/tail behavior, and intended memory only.
- OD guard zero leaves OD untouched. Guard one covers zero/positive deltas,
  representative fractional/large scales, and unsigned-add carry boundaries.
  At scale one, compare the patched body with native behavior, including
  large values; do not accidentally impose signed saturation.
- HP covers zero damage, positive sub-one scaled damage, HP below/equal/above
  scaled damage, both EDI modes, UINT32 inputs above `INT32_MAX`, scale one,
  and values whose scaled result exceeds uint32. Example: HP 100/damage 200
  with effective HP multiplier 5 uses scale .2 and should leave HP 60.
- For all four OD sites and both HP sites: success claim, every partial-install
  shape, marker mismatch, foreign jump/change, retry after failed restore,
  restart adoption, detach, and owner-stop restore every recorded entry.
  Never report a complete enable merely because the main entry is patched.
- Rebuild/re-embed the DLL and verify its hash matches the artifact tested.
  A real shared-boss encounter must still verify visible HP/OD behavior,
  ordinary-boss regression, and disable/re-enable; static byte matches do not
  close that field acceptance gate.

## Implemented verification

The production templates in `monster_shared_206_templates.h` preserve XMM0,
avoid XMM6, and scale HP through zero-extended RCX with a 64-bit result. They
use double precision and restore the stack before replaying OD's ADD/CMOVAE.
`monster_shared_206_test.go` reads those exact production byte arrays and
executes them in private executable memory in the test process. Zero damage,
fractional scales, both native minimum-HP modes, uint32 damage above INT32_MAX,
scaled damage beyond uint32, OD enable state/carry behavior, and XMM/GPR
sentinels are checked. No game memory is involved in these arithmetic tests.

The installed 2.0.6 process also passed real HP and OD enable/readback/disable
tests, including all auxiliary original-byte restoration. These paths are
readable before a shared-boss encounter and do not require entering that fight
just to enable the existing controls. This lifecycle test is not a claim that
a shared-boss fight was completed.

The first live test reproduced an existing asynchronous race: DllMain's worker
had written the main entry but was still installing auxiliary entries when Go
attempted to claim ownership. Enable confirmation now waits for every auxiliary
entry and marker, not just the main entry. The same live test then passed.
Four-entry OD partial-install rollback cases and foreign-entry retry recovery
also pass. Original save/settings files were restored after live checks.
