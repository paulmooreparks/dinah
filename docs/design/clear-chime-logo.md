# Clear Chime application mark

Paul requested integration of the approved Clear Chime dinner-triangle direction on 5 November 2026, including an icon immediately before Dinah in the site menu.

The canonical artwork is cmd/dinah-desktop/frontend/dinah-mark.svg. It uses a turquoise open triangle, gold hanging loop and a silver striker with a coral grip. The shape has a dark edge for light surfaces and a bright turquoise face for dark surfaces. The delta association is secondary to the instrument silhouette.

The site menu uses PUDL's documented image-in-first-title markup. The same SVG supplies the existing About, fallback brand and native application-menu references. The web headers and favicon use an identical SVG copy. Images beside a visible product name have empty alt text to avoid repeating the accessible name.

scripts/build-icons.cjs renders the SVG with headless Edge at 16, 20, 24, 32, 40, 48, 64, 128, 256 and 512 pixels. The first nine sizes become the Windows ICO; the 512-pixel image is the application PNG. Run the generator under the shared browser-test lock, then run go generate ./cmd/dinah-desktop to rebuild the amd64 and arm64 resources with the existing pinned go-winres generator.

Rendered sizes and browser evidence are retained under build/clear-chime-logo. Windows taskbar and Start menu refresh are checked by the person after restart; the agent does not restart the live desktop or manipulate the shell icon cache.

The branding and extensions browser suites passed, including image loading, placement before the product name, site-menu opening, refresh survival and native-menu imagery. The web package tests passed. Both production binaries built and were installed with verified hashes and retained prior binaries. The built desktop executable contains all nine generated ICO image payloads. Its version is 0.0.0-clear-chime-logo-20261105. The installation receipt is build/clear-chime-logo/install-receipt.json. No appearance preferences were changed by this installation.
