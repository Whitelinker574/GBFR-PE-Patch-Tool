# Architecture

## Runtime shape

The executable has a deliberately small root command. `main.go` embeds `frontend/dist` and passes it to `internal/backend.Run`. The backend owns Wails bindings, save transactions and all process sessions. Vue pages call generated bindings under `frontend/wailsjs`.

```text
main.go
  └─ internal/backend.Run(frontend assets)
       ├─ offline save transactions
       ├─ owned live-memory sessions
       ├─ persistent runtime-patch session
       └─ strict read-only monitoring session
```

## Repository domains

| Path | Contents | Included in the executable |
| --- | --- | --- |
| `internal/backend/` | Go backend and colocated regression tests | Production `.go` files yes; `*_test.go` files no |
| `internal/backend/data/` | Embedded game-version catalogs and evidence | Yes |
| `internal/backend/resources/` | Embedded native helper | Yes |
| `frontend/src/` | Vue pages, state helpers and frontend tests | Vue/JS production code yes; tests no |
| `frontend/public/` | Bundled official UI assets and mapping results | Yes |
| `src_dll/` | Source used to rebuild the native helper | No |
| `tools/` | Reproducible maintainer scripts and one automated regression test | No |
| `docs/` | User, architecture and evidence documentation | No |
| `build/` | Wails metadata, icons and ignored local output | Metadata/resources only |

The backend remains one package for now because its safety model shares process identity, ownership leases and rollback records across live features. Splitting those fields into independent packages without a single lifecycle owner would make cleanup races easier to introduce. The feature-prefix map in [`internal/backend/README.md`](../internal/backend/README.md) is the authoritative file index.

## Write boundaries

- Offline save operations work on a parsed snapshot, prepare all edits, write a temporary file, repair checksums, replace atomically and read the result again.
- Live operations bind a process identity, page owner token and expected target. Stale owners and changed targets are rejected.
- Runtime patches verify the game version, pattern uniqueness and exact original bytes. Restoration evidence remains owned until readback proves the original bytes are back.
- Formula monitoring uses a separate query/read-only process handle and exports redacted observations instead of memory dumps.

## Test boundaries

Tests are colocated with the code they protect because Go treats a directory as a package and several safety tests intentionally inspect package-private state. They are not shipped in the executable. Deleting or moving them to a cosmetic `tests/` directory would either remove coverage or require exporting internal memory-write primitives.
