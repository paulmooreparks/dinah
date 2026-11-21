# yavc.app content release

`yavc/` holds the portable Editor, Docs, Diagrams and Slides applets from yavc.app, as one release set, unchanged. The plan for adopting them is [docs/design/content-applets.md](../../../docs/design/content-applets.md).

| Item | Value |
| --- | --- |
| Producer | https://github.com/paulmooreparks/yavc.app |
| Release | v0.2.111, commit `38e94e32f202069ecdf947ac782a3bc3f2a21e25`, downloaded from the release's assets (`yavc-content-v0.2.111.zip`, SHA-256 `7dd9b3b43d4fae8c107db9c72b78bb6701b7f3f8868f658420c47d3e55872fcc`), checked with yavc.app's `scripts/release_record.py` against the release record pinned on the bus (SHA-256 `8603da8b645f3667c48bc7157f0272de99ff8cfce3dd9e94afa2a732b046f72f`) and against GitHub's build attestation |
| Manifest digest | `b47b7814b569f8096fdae702c48ca51f398232d8fce647bab963599e9a64a05d` |
| Applets | Editor 46.37.51, Docs 1.37.51, Diagrams 0.46.11, Slides 0.65.5 |
| PUDL | 0.108.0, the same release Dinah ships in `pudl/` |

The release's own `manifest.json` is kept beside it. Dinah ships the release's `content/`, `support/` and `LICENSE`. It leaves out the release's `pudl/`, because Dinah's copy is the same release byte for byte, and its demonstration hosts (`lab/`, `workspace-lab/`) with their `go.mod` and `go.sum`. `yavc_release_test.go` checks the aggregate digest, and the length and SHA-256 of every file, against Dinah's tree and Dinah's PUDL.

To take another release, download its three assets (`yavc-content-VERSION.zip`, `.manifest.json` and `.sha256`) from the release at https://github.com/paulmooreparks/yavc.app/releases, check the ZIP's SHA-256 against the published one, extract it into an empty folder, compare the archive's `manifest.json` with the published manifest and verify the digest and every file, then replace `yavc/` whole with the same parts and change the digest in `yavc_release_test.go` and the table above. yavc.app's `docs/content-release-downloads.md` describes the assets. Nothing in `yavc/` is edited by hand; a change goes to yavc.app.
