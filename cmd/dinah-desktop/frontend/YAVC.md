# yavc.app content release

`yavc/` holds the portable Editor, Docs, Diagrams and Slides applets from yavc.app, as one release set, unchanged. The plan for adopting them is [docs/design/content-applets.md](../../../docs/design/content-applets.md).

| Item | Value |
| --- | --- |
| Producer | https://github.com/paulmooreparks/yavc.app |
| Release | v0.2.119, commit `b4fb3a6f7da6dcb6ed27e934ec3b33205e2c1ed9`, downloaded from the release's assets (`yavc-content-v0.2.119.zip`, SHA-256 `9d6c86a5072fc7149627a466833b564087d770ef7d90c5e1e86b3d950d54cebb`), checked with yavc.app's `scripts/release_record.py` against the release record pinned on the bus (SHA-256 `ce4218902546e58a3ec5e5a0851eb53a306384e25c0fd1f96aad4c5f1d4883f7`) and against GitHub's build attestation |
| Manifest digest | `a4ae59215d911dfb7fd801779950f1ac6ca619c0ac391182e72f2e50382386dd` |
| Applets | Editor 46.37.57, Docs 1.37.57, Diagrams 0.46.15, Slides 0.65.10 |
| PUDL | 0.109.0, the same release Dinah ships in `pudl/` |

The release's own `manifest.json` is kept beside it. Dinah ships the release's `content/`, `support/` and `LICENSE`. It leaves out the release's `pudl/`, because Dinah's copy is the same release byte for byte, and its demonstration hosts (`lab/`, `workspace-lab/`) with their `go.mod` and `go.sum`. `yavc_release_test.go` checks the aggregate digest, and the length and SHA-256 of every file, against Dinah's tree and Dinah's PUDL.

To take another release, download its three assets (`yavc-content-VERSION.zip`, `.manifest.json` and `.sha256`) from the release at https://github.com/paulmooreparks/yavc.app/releases, check the ZIP's SHA-256 against the published one, extract it into an empty folder, compare the archive's `manifest.json` with the published manifest and verify the digest and every file, then replace `yavc/` whole with the same parts and change the digest in `yavc_release_test.go` and the table above. yavc.app's `docs/content-release-downloads.md` describes the assets. Nothing in `yavc/` is edited by hand; a change goes to yavc.app.
