# PDF-lib 1.17.1

The browser bundle is the published `pdf-lib/dist/pdf-lib.min.js` from the exact npm dependency recorded in package-lock.json. Reproduce the installation in a temporary directory using `npm ci --ignore-scripts`, then copy that file and LICENSE.md. The licenses folder retains the notices for bundled standard-fonts, upng, pako and tslib dependencies. Fontkit and Roboto use the existing pinned export assets beside this directory. Roboto's four TTF variants are decoded from the published pdfmake virtual font file, without modifying their bytes.

The footnote PDF writer uses PDF-lib's documented font metrics, text/image/vector drawing, font embedding and public PDF object API. It does not inspect pdfmake layout objects or infer page positions from private callbacks. Runtime export needs no CDN access.
