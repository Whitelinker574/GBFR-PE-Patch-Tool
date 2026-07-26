# Performance baseline

Captured on 2026-07-27 for `experiment/community-special-features`. Production numbers come from `vite build`; browser timing is diagnostic only until the equivalent packaged Wails run is recorded.

## Reference machine

| Field | Value |
| --- | --- |
| System | MSI GP76 Leopard 11UG |
| CPU | Intel Core i7-11800H, 8 cores / 16 threads |
| Memory | 39.8 GB |
| GPU | NVIDIA GeForce RTX 3070 Laptop GPU |
| OS | Windows 11 Enterprise 10.0.26200 |
| WebView2 | 150.0.4078.99 |
| Desktop | 1707 x 960 at 150% scaling |

The low-spec gate remains 4 cores / 8 threads, 8 GB RAM, integrated graphics, 1920 x 1080 at 125% scaling. Its actual CPU/GPU model must be recorded before machine-sensitive timings become hard CI failures.

## Build comparison

| Metric | Before P0 | Current | Change |
| --- | ---: | ---: | ---: |
| Initial JS gzip | 477,720 bytes | 137,735 bytes | -71.2% |
| Initial CSS gzip | 43,600 bytes | 12,032 bytes | -72.4% |
| Initial JS chunks | 1 | 2 entry/direct-import chunks | Page code moved to async chunks |
| Function art startup decode | 46 images, about 448 MB RGBA | No global decode queue | Current target only |

The enforced budgets are 256,000 bytes initial JS gzip and 25,600 bytes initial CSS gzip. `npm run check:bundle` reads Vite's manifest and includes direct module-preload dependencies rather than checking only the entry filename.

## Asset pipeline

- 22 function identities have independent art and sticker records.
- Each record has content-hashed `thumb`, `display`, and `full` WebP variants.
- Display art is capped at 2880 px without enlargement; this preserves the existing 2560 x 1440 composition while avoiding unconditional 4K decode.
- Navigation hover, focus, or pointer-down starts page-code and display-image preparation. The active page changes only after preparation completes.
- A cold navigation keeps the current page visible until the destination module, portrait, and sticker are ready. A 15-second guard exposes an explicit retry state, and a missing display variant falls back to the approved full source rather than switching to an empty page.
- Runtime editing pages keep their connection owner and draft state in Vue's cache across navigation. Their UI-only polling pauses while hidden and resumes on activation; explicit safety restoration timers are not suspended.
- Playwright diagnostics at 960 x 640, 1366 x 768, and 2560 x 1440 showed no horizontal page overflow, missing images, console errors, or layout jumps.
- The same three cold-cache viewport checks sampled every 16 ms during the first loadout-page transition and recorded zero blank frames and zero visible frames without the function background.
- The tested loadout art/sticker fetched in about 28-30 ms from the local preview server and reused cache in about 3 ms. These figures are not Wails release thresholds.

## Runtime update path

The role-loadout detector keeps its authoritative Go ticker at 2.5 seconds. The duplicate three-second frontend status poll was replaced with a Wails status event; history reloads only when `historyCount` changes. Hiding or leaving the page does not lower the Go tick rate or stop the detector.

## Required follow-up measurements

1. Packaged Wails cold/warm page switches, at least 10 runs per viewport profile.
2. Hidden-window CPU for 60 seconds while the detector remains active at its normal tick rate.
3. Working-set trend across 50 repeated page switches.
4. Equivalent results on the recorded low-spec machine.
