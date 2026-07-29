# Performance baseline

Captured on 2026-07-30 for `experiment/community-special-features`. Production numbers come from `vite build`; browser timing is diagnostic only until the equivalent packaged Wails run is recorded.

## Reference machine

| Field | Value |
| --- | --- |
| System | MSI GP76 Leopard 11UG |
| CPU | Intel Core i7-11800H, 8 cores / 16 threads |
| Memory | 39.8 GB |
| GPU | NVIDIA GeForce RTX 3070 Laptop GPU |
| OS | Windows 11 Enterprise 10.0.26200 |
| WebView2 | 150.0.4078.105 |
| Desktop | 1707 x 960 at 150% scaling |

The low-spec gate remains 4 cores / 8 threads, 8 GB RAM, integrated graphics, 1920 x 1080 at 125% scaling. Its actual CPU/GPU model must be recorded before machine-sensitive timings become hard CI failures.

## Build comparison

| Metric | Before P0 | Current | Change |
| --- | ---: | ---: | ---: |
| Initial JS gzip | 477,720 bytes | 156,670 bytes | -67.2% |
| Initial CSS gzip | 43,600 bytes | 12,224 bytes | -72.0% |
| Initial JS chunks | 1 | 2 entry/direct-import chunks | Page code moved to async chunks |
| Function art startup decode | 46 images, about 448 MB RGBA | No global decode queue across 60 current images | Current target only |

The enforced budgets are 256,000 bytes initial JS gzip, 25,600 bytes initial CSS gzip, 160,000 bytes for the largest async JS chunk, 30,000 bytes for the largest async CSS chunk, 1,500,000 bytes per raster image, 800,000 bytes per function-art asset, and 12,000,000 bytes for all function art. The final build measured 142,004 bytes for the largest async JS chunk, 12,382 bytes for the largest async CSS chunk, 1,321,102 bytes for the largest raster, 539,726 bytes for the largest function-art asset, and 11,646,630 bytes for all function art. `npm run check:bundle` reads Vite's manifest and includes direct module-preload dependencies rather than checking only the entry filename.

## Asset pipeline

- 30 function identities have independent art and sticker records.
- Each record has one content-hashed, high-quality `display` WebP. Removed `thumb` and `full` copies are not packaged because no current page consumes them.
- The generated function-art directory is 11,664,631 bytes across 60 display images plus its manifest; the 29 share portraits remain a separate lazy-loaded set.
- Display art is capped at 2520 px without enlargement and encoded as high-quality WebP; this preserves the supported desktop composition while avoiding unconditional 4K decode.
- Navigation hover, focus, or pointer-down starts page-code and display-image preparation. The active page changes only after preparation completes.
- A cold navigation keeps the current page visible until the destination module, portrait, and sticker are ready. A 15-second guard exposes an explicit retry state; a failed image leaves the old page visible instead of switching to an empty page.
- Runtime editing pages keep their connection owner and draft state in Vue's cache across navigation. Their UI-only polling pauses while hidden and resumes on activation; explicit safety restoration timers are not suspended.
- The packaged Wails build was exercised at the real 960 x 640 minimum: the home page remained internally scrollable, the flat horizontally scrollable function tabs reached the last runtime page without document-level horizontal overflow, and the save laboratory used the full work area when its portrait asset was unavailable. Maximized layout remained intact.
- A browser-shell responsive matrix traversed all 28 page destinations in Chinese and English at 960 x 640, 1024 x 768, 1280 x 720, 1366 x 768, 1600 x 900, 1920 x 1080 and 2560 x 1440 with device scale factors 1.0, 1.25 and 1.5. All 1,176 page cases had zero document/workspace horizontal overflow, zero visible-control clipping and zero page/console errors. Device-scale emulation validates CSS and raster behavior; the packaged reference-machine checks below remain the evidence for actual WebView2 at Windows 150% scaling.
- Cold-cache checks must sample the first loadout-page transition for blank frames, missing function art, console errors, and layout shifts. Local preview timings are diagnostic and are not Wails release thresholds.
- The packaged Windows amd64 build completed on the reference machine and opened its home page, local save slot, loadout catalog, and share workshop without a blank frame. A deliberately failed destination image kept the home page visible and exposed the retry action.
- The final production EXE was also walked through at 1280 x 800: all four managed-mod pages opened with bounded controls, SaveData3 returned 28 characters and 28 presets, the share workshop opened from the expanded Id preset, and the optimizer evaluated 18,875 exact states over 802 recognized inventory instances without freezing the window. No candidate was loaded and no save write was performed during this check.

## Runtime update path

The role-loadout detector keeps its authoritative Go ticker at 2.5 seconds. The duplicate three-second frontend status poll was replaced with a Wails status event; history reloads only when `historyCount` changes. Hiding or leaving the page does not lower the Go tick rate or stop the detector.

## Loadout calculation path

- `LoadoutEditor` sends at most one simulation request at a time. Changes that arrive while it is running are coalesced into one trailing request for the latest normalized draft; a stale response cannot replace a newer build.
- The complete loadout editor is an async child of the viewer, so opening the preset catalog no longer downloads or parses the editor up front.
- Exact optimizer DP runs in a dedicated Web Worker. Starting another solve terminates the previous worker, while the bounded heuristic fallback remains available for inputs outside the exact solver limits.
- Navigation image preparation starts only after a 160 ms hover intent, or immediately on focus/pointer-down. Incidental pointer travel therefore does not start a broad image-download queue.

## Background-work boundaries

The reviewed implementation uses per-domain workload control instead of a pass-through Go job manager across unrelated runtimes. Exact optimizer work is cancellable by terminating its dedicated Web Worker. DOM-dependent share-image rendering is single-flight and disables both export entry points until completion; image preparation has an explicit timeout. Relink Logs import is bounded to 1 MiB of JSON, 16 players, 200 database records and an 8 MiB compressed/decompressed blob. Save comparison is single-flight in the UI, paginates output in groups of at most 200, and now rejects either input above 64 MiB before parsing. Save writes remain outside this mechanism so UI navigation cannot interrupt an atomic backup/write/readback sequence.

## Logs archive pagination

The upstream Relink Logs schema has no `(time, id)` index, and this application opens it strictly read-only. A 10,000-row benchmark against that unmodified schema measured about 14.5 ms for the first 40-row page and 8.7 ms for a deep 40-row keyset page on the reference CPU (`10x`, Go benchmark). The query therefore retains exact `(time, id)` ordering without writing an index into the user's database; packaged Wails measurements and larger real databases remain release evidence gates.

## Sigil atlas transport

The UI uses a normalized atlas index: traits are transferred once, while each sigil refers to secondary traits by numeric index and carries only its per-sigil maximum levels. The complete GBFR 2.0.2 payload is 169,312 bytes in the Go contract test, below the 262,144-byte IPC limit. Atlas and optimizer pages share one in-flight/resolved request per language; failed requests are evicted so retry remains possible. Their component instances are cached only after first use, while the editor, share image, and battle archive remain independently mounted on demand.

A production-preview stress run used 229 sigils with 30 secondary-trait references each and switched between the atlas and optimizer 100 times. The atlas endpoint was called once, P50 was 89.25 ms, P95 was 120.54 ms, and the maximum was 129.21 ms. After explicit Chromium heap collection, used JS heap changed from 12,820,577 to 12,984,520 bytes (+163,943 bytes) instead of growing linearly. This is browser evidence; packaged WebView timing remains a separate release measurement.

The packaged share workshop also exported a 1920 x 1080 PNG with a long Chinese title and an HTTPS QR code. An independent decoder recovered the exact input URL. The automated matrix now additionally regenerates the same QR settings and recovers the exact URL from a 108 px PNG, 108 px quality-85 JPEG, 96 px quality-72 JPEG, and 96 px quality-68 WebP. This covers deterministic resize/recompression regressions; a real QQ client send/download round trip remains a separate community-device check.

## Repeated page-switch stress

On 2026-07-29, the final Vite production UI was opened in Edge 150 with a read-only Wails API stub and cycled 50 times through the preset catalog, live loadout recorder, convenience runtime, runtime monitor, and save laboratory. After a warm-up and explicit Chromium collection, retained JS heap grew by 216,636 bytes, peak growth was 803,140 bytes, DOM node and document growth were both zero, no long task exceeded 100 ms, and the hidden five-second window recorded zero task milliseconds per second. Cached loadout-panel, battle-archive, and convenience-runtime UI clocks now stop on deactivation while their Go/native sessions remain active. `tools/qa_runtime_stress.mjs` records its execution mode so this browser-shell result cannot be mistaken for a packaged WebView2 or active-detector measurement.

The debug-enabled packaged Wails application was then measured against its real WebView2 target. At 1280 x 800, the first traversal had P50 335.5 ms and P95 448.6 ms; 50 warm transitions had P50 230.1 ms and P95 277.5 ms. At the supported 960 x 640 minimum, the first traversal had P50 336.2 ms and P95 458.8 ms; 50 warm transitions had P50 231.1 ms and P95 276.3 ms. Both profiles ended with zero DOM/document growth and no task over 100 ms. A release-length 60-second minimized run with the persistent role-loadout detector explicitly active recorded 2 task ms/s, retained 546,072 bytes of JS heap after collection, and remained within every enforced runtime budget.

Two additional 50-switch OS process samples showed no retained working-set trend: the eight-process Wails/WebView2 tree ended 72,368,128 bytes and 67,264,512 bytes below its respective warm starting points. Peak transient working-set growth was 36,933,632 bytes and 52,322,304 bytes. Private allocation rose transiently while code and image caches were active, then returned below the preceding run's end before the repeat started; JS heap and DOM counts independently remained bounded. These are reference-machine release measurements, not low-spec thresholds.

## Required follow-up measurements

1. Equivalent results on a recorded 4-core / 8-thread, 8 GB, integrated-GPU low-spec machine.
2. A real QQ client send/download round trip for the generated share image; deterministic resize/recompression decoding is already automated.
