# Default palette study

Paul requested alternatives to Dinah's inherited palette on 24 October 2026, including a light mode without white backgrounds and a higher-contrast dark mode. He also requested adjustable colors. Paul accepted Mineral as the new default pair and Linen as the alternative to bundle when theme selection is implemented. Mineral is applied through the desktop and bus page brand stylesheets.

Open [the interactive workshop](color-preview.html) in a browser. It loads the vendored PUDL stylesheet and fonts locally, displays paired light/dark mockups, and supports color pickers, hex entry, undo, reset and JSON import/export. The mockup keeps fonts and layout constant to isolate the palette decision. Its buttons and chat content illustrate appearance; they do not control the real application. Changing colors affects the preview immediately. Export saves both variants, and Import restores them. The study format is `dinah-color-study` version 1 and is separate from the production Dinah theme interchange contract.

Mineral is the accepted default. It uses cool stone surfaces and an ink-blue accent. Linen provides a warmer paper and olive alternative. Both use tinted input fills and content surfaces, rather than relying on the browser's default white textarea. Their dark variants use brighter secondary text and more visible control boundaries. PUDL still derives raised and sunken controls from the palette and lighting tokens.

| Role | Mineral light | Mineral dark | Linen light | Linen dark |
| --- | --- | --- | --- | --- |
| Workspace | `#E6ECEE` | `#0C141B` | `#EAE5DA` | `#191A16` |
| Content | `#EEF2F3` | `#17242D` | `#F4EFE2` | `#1D231C` |
| Alternate surface | `#DBE4E7` | `#253642` | `#E1DACB` | `#2D372B` |
| Text | `#23333B` | `#F2F6F9` | `#302D24` | `#F6F4E8` |
| Secondary text | `#50636C` | `#C3D2DF` | `#655F50` | `#CBCDBB` |
| Control boundary | `#748790` | `#8199A9` | `#8B7C65` | `#899475` |
| Accent | `#3C607B` | `#94CCEA` | `#5A6537` | `#C7D696` |

Browser checks verified editing, undo, export/reimport, non-white composer fills, a 390 px layout without horizontal page overflow, and absence of page errors. Sample ratios measured from resolved preview colors were 11.59:1 for Mineral light text on content and 14.57:1 for Mineral dark; Linen measured 11.98:1 and 14.53:1. Secondary text on alternate surfaces measured at least 4.57:1, action text at least 5.60:1, and boundaries against input fills at least 3.41:1 across the four previews. These are checks of the included samples, not a complete accessibility audit or verification of all Dinah views.

The workshop is a color-selection aid rather than the production theme editor. The default implementation covers semantic palette and syntax colors, shared controls and application status colors. Document and visual-editor surfaces now follow the selected mode; embedded images and diagrams retain their artwork colors. User light/dark/system preferences are preserved. Linen is now included in the Desktop theme selector. The application-wide font/theme design remains in [appearance.md](appearance.md).
