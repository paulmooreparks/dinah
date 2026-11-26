# yavc.app content release

`yavc/` holds the portable Editor, Docs, Diagrams and Slides applets from yavc.app, as one release set, unchanged. The plan for adopting them is [docs/design/content-applets.md](../../../docs/design/content-applets.md).

| Item | Value |
| --- | --- |
| Producer | https://github.com/paulmooreparks/yavc.app |
| Release | v0.2.122, commit `6563cf7edd07019057f80d98dcde460a765680d3`, downloaded from the release's assets (`yavc-content-v0.2.122.zip`, SHA-256 `0f48010361a69d687cc0f7be9dbc552b8e4d2c98afcb5fbb33a045741c5db254`), checked with yavc.app's `scripts/release_record.py` against the release record pinned on the bus (SHA-256 `e8915820c9e03e1ec1c9bbd2d57ac37e7eda5ee2442da4c9180a16c19015bde3`) and against GitHub's build attestation |
| Manifest digest | `eb5ed9e1db74619deba6c872c01ffe932d620f70b5f2789fec3757f9877fc452` |
| Applets | Editor 46.37.57, Docs 1.37.57, Diagrams 0.46.15, Slides 0.65.10 |
| PUDL | 0.109.0, the same release Dinah ships in `pudl/` |

The release's own `manifest.json` is kept beside it. Dinah ships the release's `content/`, `support/` and `LICENSE`. It leaves out the release's `pudl/`, because Dinah's copy is the same release byte for byte, and its demonstration hosts (`lab/`, `workspace-lab/`) with their `go.mod` and `go.sum`. `yavc_release_test.go` checks the aggregate digest, and the length and SHA-256 of every file, against Dinah's tree and Dinah's PUDL.

To take another release, download its three assets (`yavc-content-VERSION.zip`, `.manifest.json` and `.sha256`) from the release at https://github.com/paulmooreparks/yavc.app/releases, check the ZIP's SHA-256 against the published one, extract it into an empty folder, compare the archive's `manifest.json` with the published manifest and verify the digest and every file, then replace `yavc/` whole with the same parts and change the digest in `yavc_release_test.go` and the table above. yavc.app's `docs/content-release-downloads.md` describes the assets. Nothing in `yavc/` is edited by hand; a change goes to yavc.app.
