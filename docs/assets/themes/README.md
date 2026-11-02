# Theme screenshots

The six PNGs are full-resolution captures of Posting's real UI at 120 × 36
terminal cells, using 18px JetBrains Mono at 2× scale (2624 × 1846 pixels).
They use demo data and the same app state for each theme.

`themes.gif` is the README slideshow: Galaxy, Aurora, Lantern, Midnight Ember,
Neon Reef, then Cyberdeck. It loops indefinitely, holding each theme for three
seconds without transitions. The PNGs retain the original colours and resolution
for reuse; the GIF uses a reduced palette to keep its download small.

To rebuild the GIF with ImageMagick 7, run from the repository root:

```sh
magick -delay 300 -dispose none \
  docs/assets/themes/galaxy.png \
  docs/assets/themes/aurora.png \
  docs/assets/themes/lantern.png \
  docs/assets/themes/midnight-ember.png \
  docs/assets/themes/neon-reef.png \
  docs/assets/themes/cyberdeck.png \
  -dither None -colors 256 -loop 0 -layers Optimize \
  docs/assets/themes/themes.gif
```
