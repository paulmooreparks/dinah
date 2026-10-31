# Skins backend review, 31 October 2026

Engine reviewed the existing backend against pudl-skins.md and the overnight
assignment. The appearance store already persists the chosen skin beside the
theme, keeps syntactically valid unavailable choices, applies Standard as the
effective fallback and preserves the stored skin when an older page omits it.
Successful saves already announce appearance:changed with the revision and
effective skin. Both desktop configuration and the web host use Dinah's home.

The web host already emits host-owned persistence, the effective skin and theme
preference on the root, and static links for admitted stylesheets before PUDL's
scripts and host palette CSS. It reads the stored choice when rendering rather
than creating a second preference store. These paths were retained.

The review found an admission-validation gap. The parser previously ignored the
catalogue version and accepted admitted entries without usable stylesheet/hash
metadata. Desktop tests checked bundled hashes, but the hosts did not validate
their embedded admitted assets themselves.

The correction requires catalogue version 1, a labelled admitted Standard entry
without a stylesheet, and local CSS filenames with SHA-256 digests for other
admitted entries. Each host validates admitted CSS bytes in its own embedded
filesystem during initialization. Unadmitted candidates remain unavailable.
A build with invalid admitted assets fails initialization rather than claiming
those assets meet admission. An unavailable saved preference still falls back
to Standard and remains unchanged on disk.

Preference readers now avoid relative-file lookup when the home is unavailable.
The web theme reader also returns the normalized mode it validated, so whitespace
cannot turn an accepted dark preference into an invalid root attribute.

ReadAppearance and SaveAppearance retain their existing public JSON contract.
No frontend assets, preference formats, provider code or running application
were changed. Front End owns runtime stylesheet-load failure handling, first
paint, settings and preview adoption. Quality must verify the rendered skin and
palette matrix; backend tests do not establish that acceptance.

Focused tests passed for catalogue rejection, asset digest mismatch/missing
files, preference preservation and fallback, desktop appearance/event behavior,
and web rendering/serving for every admitted skin. The web test verifies that
rendering leaves preference bytes unchanged. The full affected-package gate
passed internal/skins (0.507 s), internal/web (1.524 s) and cmd/dinah-desktop
(76.645 s). Live-tag vet passed for all three packages. Formatting and whitespace
checks were clean. No browser gate, rendered acceptance, install or restart was
performed by Engine.
