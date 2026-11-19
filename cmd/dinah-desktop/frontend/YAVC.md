# yavc.app content release

`yavc/` holds the portable Editor, Docs, Diagrams and Slides applets from yavc.app, as one release set, unchanged. The plan for adopting them is [docs/design/content-applets.md](../../../docs/design/content-applets.md).

| Item | Value |
| --- | --- |
| Producer | https://github.com/paulmooreparks/yavc.app |
| Release | v0.2.107, commit `611d3f2aea34512f1599af16c94d02170ab0db2a`, downloaded from the release's assets (`yavc-content-v0.2.107.zip`, SHA-256 `6417ad86f65a2ec654a2a04f770ec7c2927f7b531d121cb7246b50cd2ffdaee5`), checked with yavc.app's `scripts/release_record.py` against the release record pinned on the bus (SHA-256 `eb5015a32b6b2ad3a67053ca6b4f5f67b6c84bb38d50380f50f23977b773f8bb`) and against GitHub's build attestation |
| Manifest digest | `ae4c74d1ac2e56806b221a58dc26f6fea50616d00c7ca23214d9059d573d6f51` |
| Applets | Editor 46.37.51, Docs 1.37.51, Diagrams 0.46.11, Slides 0.65.5 |
| PUDL | 0.108.0, the same release Dinah ships in `pudl/` |

The release's own `manifest.json` is kept beside it. Dinah ships the release's `content/`, `support/` and `LICENSE`. It leaves out the release's `pudl/`, because Dinah's copy is the same release byte for byte, and its demonstration hosts (`lab/`, `workspace-lab/`) with their `go.mod` and `go.sum`. `yavc_release_test.go` checks the aggregate digest, and the length and SHA-256 of every file, against Dinah's tree and Dinah's PUDL.

To take another release, download its three assets (`yavc-content-VERSION.zip`, `.manifest.json` and `.sha256`) from the release at https://github.com/paulmooreparks/yavc.app/releases, check the ZIP's SHA-256 against the published one, extract it into an empty folder, compare the archive's `manifest.json` with the published manifest and verify the digest and every file, then replace `yavc/` whole with the same parts and change the digest in `yavc_release_test.go` and the table above. yavc.app's `docs/content-release-downloads.md` describes the assets. Nothing in `yavc/` is edited by hand; a change goes to yavc.app.
