# Tray compaction correction

Paul clarified that tray folding means changing widgets to individual glyph buttons before taskbar windows shrink or stack. The earlier aggregate Status button was the wrong interpretation.

The corrected candidate removes Dinah's aggregate status-fold opt-in and replaces the local fold-priority PUDL patch with compact-priority measurement. It compares ordinary window tab natural widths with available space and compacts the tray first. Full-mode stack measurement already probes compact geometry. A stacked dock keeps the tray compact until ordinary tabs fit again. Each widget remains an individual action; Dinah hides its label and badge in compact mode while retaining its accessible name and glyph.

The focused PUDL Chromium test verifies compaction before stacking, preservation of individual actions and restoration at wide widths. The desktop `tray-glyphs` suite verifies the actual Dinah markup at 560 pixels, including hidden badges and absence of an aggregate Status button. The rendered screenshot is retained at `build/tray-glyphs/tray-glyphs-narrow.png`.

The source and generated PUDL JavaScript are copied to both Dinah vendor directories. No executable was built or installed. This correction takes priority over the pending list-rendering improvements.
