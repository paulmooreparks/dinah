The document exporters use the published browser builds of [docx 9.15.2](https://github.com/dolanmiu/docx), [pdfmake 0.2.23](https://github.com/bpampuch/pdfmake) and [@pdf-lib/fontkit 1.1.1](https://github.com/Hopding/fontkit). Their JavaScript code uses the MIT license. License texts for bundled dependencies are retained under the vendor directories.

Pdfmake's virtual font file contains Roboto 3.014. Its copyright is held by the Roboto Project Authors, and the fonts use the SIL Open Font License 1.1, retained as pdfmake-0.2.23/OFL.txt. The regular font is also stored separately for the exporter's documented glyph-coverage check. Fontkit's inherited MIT text is copied from the upstream foliojs/fontkit project because the published fork omits a standalone license file.

The package locks record the build inputs. The browser loads these local assets only when an export needs them. No export loads code or fonts from a CDN, and document source remains in the browser.
