# yavc.app content release

`yavc/` holds the portable Editor, Docs, Diagrams and Slides applets from yavc.app, as one release set, unchanged. The plan for adopting them is [docs/design/content-applets.md](../../../docs/design/content-applets.md).

| Item | Value |
| --- | --- |
| Producer | https://github.com/paulmooreparks/yavc.app |
| Release | v0.2.113, commit `78ebad4cc6083799efa25877c23f43dfc4d64cee`, downloaded from the release's assets (`yavc-content-v0.2.113.zip`, SHA-256 `24b8f14bbf243e911c04d06d41e8475bbb827ba21578914ddd5e66953ae562d5`), checked with yavc.app's `scripts/release_record.py` against the release record pinned on the bus (SHA-256 `34bc2c7b75120208e950c6032c9ff0aab1572816db8bb03cc7eb8f4c3f03df73`) and against GitHub's build attestation |
| Manifest digest | `58cc6b5ac4cda66ac8a02d53d7838cf781b49a89b56dd7a873d7f90d0fb5133b` |
| Applets | Editor 46.37.53, Docs 1.37.53, Diagrams 0.46.12, Slides 0.65.7 |
| PUDL | 0.108.0, the same release Dinah ships in `pudl/` |

The release's own `manifest.json` is kept beside it. Dinah ships the release's `content/`, `support/` and `LICENSE`. It leaves out the release's `pudl/`, because Dinah's copy is the same release byte for byte, and its demonstration hosts (`lab/`, `workspace-lab/`) with their `go.mod` and `go.sum`. `yavc_release_test.go` checks the aggregate digest, and the length and SHA-256 of every file, against Dinah's tree and Dinah's PUDL.

To take another release, download its three assets (`yavc-content-VERSION.zip`, `.manifest.json` and `.sha256`) from the release at https://github.com/paulmooreparks/yavc.app/releases, check the ZIP's SHA-256 against the published one, extract it into an empty folder, compare the archive's `manifest.json` with the published manifest and verify the digest and every file, then replace `yavc/` whole with the same parts and change the digest in `yavc_release_test.go` and the table above. yavc.app's `docs/content-release-downloads.md` describes the assets. Nothing in `yavc/` is edited by hand; a change goes to yavc.app.
