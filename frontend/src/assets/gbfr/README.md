# GBFR UI illustration assets

Official *Granblue Fantasy: Relink* character pages were used as identity, outfit and palette references:

- `https://relink.granbluefantasy.jp/en/characters/detail?chara=lyria`
- `https://relink.granbluefantasy.jp/en/characters/`

`journal-scene-4k.webp` is the washed 16:9 notebook banner used on the home screen and as the translucent function-page background. The transparent character art under `cutouts/` is used as the page illustration and as the cropped expression badge in each character note. The square drafts under `functions/` are retained as reproducible reference inputs.

The final assets were generated through the local QNAIGC GPT Image 2 helper in `tools/qnaigc_image_edit.py`, then chroma-keyed, cropped and optimized for the UI. The generation direction uses clean large color masses, clear silhouettes, low visual noise and one cohesive parchment/sky scene rather than a collage. Each function has its own character and activity so the art also acts as a visual navigation cue.

The generated images are fan-made UI assets, not official game artwork. Granblue Fantasy and its characters are properties of Cygames, Inc. Confirm redistribution and trademark rights before including these assets in an upstream release or pull request.
