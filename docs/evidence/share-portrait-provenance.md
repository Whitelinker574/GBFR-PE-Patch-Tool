# Share portrait provenance

The loadout share workshop contains one transparent WebP background for each of the 29 visual identities in `frontend/src/characterRoster.js`. Gran and Djeeta remain separate visual identities even though the game counts them as one gameplay slot.

These files were converted locally from the project's previously reviewed transparent character sources. No community avatar, wiki crop, or another character's image is used as a fallback. The original source art, generation workflow, credentials, and intermediate files are not distributed in this repository.

Acceptance checks for the production set:

- every roster slug resolves to one existing WebP with the same slug;
- every file decodes as a non-empty image with transparency;
- file hashes are unique across all 29 identities;
- navigation loads only the selected character background;
- a missing mapping falls back to the character's verified compact game icon, never another portrait.

Game character names and derived visual assets remain the property of their respective rights holders. They are included only for local loadout identification and presentation in this non-commercial tool.
