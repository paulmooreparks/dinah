# yavc.app content release

`yavc/` holds the portable Editor, Docs, Diagrams and Slides applets from yavc.app, as one release set, unchanged. The plan for adopting them is [docs/design/content-applets.md](../../../docs/design/content-applets.md).

| Item | Value |
| --- | --- |
| Producer | https://github.com/paulmooreparks/yavc.app |
| Release | v0.2.120, commit `7be70d9d0a2cd9064b361459c422480a5c48df39`, downloaded from the release's assets (`yavc-content-v0.2.120.zip`, SHA-256 `8370cd73cc056a911881c2b3faa4e3c6267f5d4c98169755821fa846fec5ea50`), checked with yavc.app's `scripts/release_record.py` against the release record pinned on the bus (SHA-256 `7430d84b598265e554fcea2fbaad517f085148c52f3140d4afa68b8cfd28926b`) and against GitHub's build attestation |
| Known open | A WebKit check on Linux (yavc.app `test-docs-structure.cjs`) fails: text typed at the end of a document that ends in a table does not become a new paragraph. Paul accepted the release with it open; the Yavc.App Team Lead owns it. |
| Manifest digest | `4819fe84166a2d3daf09a42f7b132e5e0e53a937481074448610ad5ecd78caae` |
| Applets | Editor 46.37.57, Docs 1.37.57, Diagrams 0.46.15, Slides 0.65.10 |
| PUDL | 0.109.0, the same release Dinah ships in `pudl/` |

The release's own `manifest.json` is kept beside it. Dinah ships the release's `content/`, `support/` and `LICENSE`. It leaves out the release's `pudl/`, because Dinah's copy is the same release byte for byte, and its demonstration hosts (`lab/`, `workspace-lab/`) with their `go.mod` and `go.sum`. `yavc_release_test.go` checks the aggregate digest, and the length and SHA-256 of every file, against Dinah's tree and Dinah's PUDL.

To take another release, download its three assets (`yavc-content-VERSION.zip`, `.manifest.json` and `.sha256`) from the release at https://github.com/paulmooreparks/yavc.app/releases, check the ZIP's SHA-256 against the published one, extract it into an empty folder, compare the archive's `manifest.json` with the published manifest and verify the digest and every file, then replace `yavc/` whole with the same parts and change the digest in `yavc_release_test.go` and the table above. yavc.app's `docs/content-release-downloads.md` describes the assets. Nothing in `yavc/` is edited by hand; a change goes to yavc.app.
