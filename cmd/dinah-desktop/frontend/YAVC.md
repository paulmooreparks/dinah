# yavc.app content release

`yavc/` holds the portable Editor, Docs, Diagrams and Slides applets from yavc.app, as one release set, unchanged. The plan for adopting them is [docs/design/content-applets.md](../../../docs/design/content-applets.md).

| Item | Value |
| --- | --- |
| Producer | https://github.com/paulmooreparks/yavc.app |
| Release | v0.2.123, commit `df39a2176796e204441398f3bd70797a76ebf35f`, downloaded from the release's assets (`yavc-content-v0.2.123.zip`, SHA-256 `9fa63ebc695d226b7690751ef1d3b11e50cfbc224f9a5a918bae395f8f778b33`), checked with yavc.app's `scripts/release_record.py` against the release record pinned on the bus (SHA-256 `3eaa7fbd02611db8e246a5d35d3c91816d306afe5ccdf0c666cb0026d0883c2a`) and against GitHub's build attestation |
| Manifest digest | `87d8129a407f94935f7d5e56f96d0d5ea95823fcf62ff5ff30fcb54cab8309c9` |
| Applets | Editor 46.37.57, Docs 1.37.57, Diagrams 0.46.15, Slides 0.65.10 |
| PUDL | 0.110.0, the same release Dinah ships in `pudl/` |

The release's own `manifest.json` is kept beside it. Dinah ships the release's `content/`, `support/` and `LICENSE`. It leaves out the release's `pudl/`, because Dinah's copy is the same release byte for byte, and its demonstration hosts (`lab/`, `workspace-lab/`) with their `go.mod` and `go.sum`. `yavc_release_test.go` checks the aggregate digest, and the length and SHA-256 of every file, against Dinah's tree and Dinah's PUDL.

To take another release, download its three assets (`yavc-content-VERSION.zip`, `.manifest.json` and `.sha256`) from the release at https://github.com/paulmooreparks/yavc.app/releases, check the ZIP's SHA-256 against the published one, extract it into an empty folder, compare the archive's `manifest.json` with the published manifest and verify the digest and every file, then replace `yavc/` whole with the same parts and change the digest in `yavc_release_test.go` and the table above. yavc.app's `docs/content-release-downloads.md` describes the assets. Nothing in `yavc/` is edited by hand; a change goes to yavc.app.
