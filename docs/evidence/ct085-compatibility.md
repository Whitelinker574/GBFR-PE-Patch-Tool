# CT 0.8.5 compatibility audit

- Source: `D:\gbf\11\GFR_v0.8.5_CHS_非官方版.ct`
- SHA-256: `DBEB000F161EFB469D63DE14D3D4DEB54E185107273CAF8FDB4B8CDC75A7EEC9`
- Compared catalog: checked-in CT 0.8.4 safe rewrite (58 features / 81 patch sites / 79 AOBs)
- Result: all 58 feature definitions have identical names, groups, AOBs, offsets, enable bytes, expected original/disable bytes, and runtime-capture flags.

The production direct-patch catalog therefore remains locked to its independently verified 0.8.4 provenance instead of being relabelled as 0.8.5 without new evidence.

## Ported 0.8.5 addition

CT node 40000, “current viewed wrightstone”, captures `RDX` at:

- AOB: `48 89 D7 48 89 CE E8 ?? ?? ?? ?? 48 39 FE 74 ?? 48 8D 4E 18 8B 47 18`
- Local 2.0.2 executable: exactly one match
- PE `.text` RVA: `0x361CB4`
- Exact local bytes: `48 89 D7 48 89 CE E8 F1 05 FC FF 48 39 FE 74 4C 48 8D 4E 18 8B 47 18`
- Record layout: three `(trait hash, level)` pairs at `+0x00..+0x17`, wrightstone item hash at `+0x18`

The tool now uses this site for its existing owned wrightstone capture. Installation requires the complete 23-byte local guard, while recovery requires the versioned owned-cave marker and exact return target. The existing save-function prologue guard, owner token, stale-selection check, automatic save backup, atomic rollback, and read-back verification remain in force.

## Not claimed

The CT changelog only says “Script Fixes”. No other changed stable patch bytes were found in the 58-feature product subset, so no unverified script was copied and no version guard was weakened.
