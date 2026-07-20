# GBFR UI illustration assets

Official *Granblue Fantasy: Relink* character pages were used as identity, outfit and palette references:

- `https://relink.granbluefantasy.jp/en/characters/detail?chara=lyria`
- `https://relink.granbluefantasy.jp/en/characters/`

`journal-scene-4k.webp` is the washed 16:9 notebook banner used only on the home screen. Function pages use the character-free ornamental `parchment-ui-v2.webp`; transparent character art under `cutouts/` is composited as a fixed right-bottom background layer. The square drafts under `functions/` are retained as reproducible reference inputs.

The final assets were generated through the local QNAIGC GPT Image 2 workflow and then chroma-keyed, edge-safety validated and optimized for the UI. New function-specific work uses the Portable v3 page-brief pipeline (official identity anchor, line-art pass, final portrait pass and 2K sticker pass). The generation direction uses clean large color masses, clear silhouettes, low visual noise and one cohesive parchment/sky scene rather than a collage. Each function has its own character and activity so the art also acts as a visual navigation cue; the character-mechanics page uses Vaseraga and his Great Scythe Grynoth conflict-control scene instead of repeating Djeeta.

The generated images are fan-made UI assets, not official game artwork. Granblue Fantasy and its characters are properties of Cygames, Inc. Confirm redistribution and trademark rights before including these assets in an upstream release or pull request.
