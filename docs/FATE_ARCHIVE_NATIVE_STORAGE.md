# Fate archive storage correction

Game executable 2.0.5 SHA256: `7189B958FF0FE5238CEA28A2939FFDAD6E3A9ACB14DD274A9FCC8E7E275BD175`.

The prior 2555-vector hypothesis was incorrect. The `story_note_archive` fields previously interpreted as vector indices were insufficient evidence for save storage. Do not use them as write destinations.

## Native path

- Archive manager global: RVA `0x7C101A0`. Runtime archive records occupy `manager+0x120`, 100 records of eight bytes (hash, flags). The hash lookup at `manager+0x448` points to these records.
- Save registration: RVA `0xC1223` begins binding the 100 records. `0xC12C9` loads `0x1EDD00000000` (7901, archive hash), and `0xC1381` loads `0x1EDE00000000` (7902, flags); the low 32 bits are the record UnitID. Hash and flags therefore pair by UnitID, not by a guessed table row number.
- Unlock function RVA `0x413D30`: the path at `0x413DEC` reads `[record+4]`, ORs bit 0, writes only if changed, and marks the field dirty. It preserves other bits.
- Archive-list code at `0x3CEBFD5` tests bit 0 of that same record. Mark-viewed code at `0x3CEC0AE` ORs bit 1 separately.
- Fediel: `FATE_PL2900_10` -> mission `00301029` -> `ARC_OTHER_069` -> hash `1E6D7154`. The association comes from `fate_episode.StoryNoteArchiveId1`; the ambiguous `FinalFateMaybe` field is not used to infer it.

Three local saves have 100 paired 7901/7902 rows, including Fediel's hash. The application resolves hashes independently of row order. It can create an absent catalogued archive only in a paired empty-hash, zero-flags slot. All 29 character associations are covered, including the shared Captain archive and Rackam's two archives.

## Verification

The native instruction contract is checked by `TestGame205FateArchiveNativeStorageAndUnlock` with `GBFR_GAME_EXE_205_TEST` pointing to the verified executable. Copied-save tests cover all character archives, bit preservation (including already-viewed and additional flags), unchanged unrelated bytes after restoring the target bits and checksum, missing-record creation, idempotency, stale revision rejection, and forced readback rollback. Live game-menu rendering after restart was not observed during this run; tests do not claim it was.
