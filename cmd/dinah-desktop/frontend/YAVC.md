# yavc.app content release

`yavc/` holds the portable Editor, Docs, Diagrams and Slides applets from yavc.app, as one release set, unchanged. The plan for adopting them is [docs/design/content-applets.md](../../../docs/design/content-applets.md).

| Item | Value |
| --- | --- |
| Producer | https://github.com/paulmooreparks/yavc.app |
| Release | v0.2.121, commit `578a32e380a1a38a0c3c8f9b7fc82442b3afd0ec`, downloaded from the release's assets (`yavc-content-v0.2.121.zip`, SHA-256 `92711b948b810ecf5e7484e057a80e7f84bfe92508ae90fe1b3ac176435e7f39`), checked with yavc.app's `scripts/release_record.py` against the release record pinned on the bus (SHA-256 `1b5ada81ea2a6bf9b3eef86db980520c36689b73d4d7295fdb3f0f94bf1153af`) and against GitHub's build attestation |
| Manifest digest | `1b3df1a591a184bf94e5762832b9628ef228ff4eed665d9afa799a84a9f9bd6e` |
| Applets | Editor 46.37.57, Docs 1.37.57, Diagrams 0.46.15, Slides 0.65.10 |
| PUDL | 0.109.0, the same release Dinah ships in `pudl/` |

The release's own `manifest.json` is kept beside it. Dinah ships the release's `content/`, `support/` and `LICENSE`. It leaves out the release's `pudl/`, because Dinah's copy is the same release byte for byte, and its demonstration hosts (`lab/`, `workspace-lab/`) with their `go.mod` and `go.sum`. `yavc_release_test.go` checks the aggregate digest, and the length and SHA-256 of every file, against Dinah's tree and Dinah's PUDL.

To take another release, download its three assets (`yavc-content-VERSION.zip`, `.manifest.json` and `.sha256`) from the release at https://github.com/paulmooreparks/yavc.app/releases, check the ZIP's SHA-256 against the published one, extract it into an empty folder, compare the archive's `manifest.json` with the published manifest and verify the digest and every file, then replace `yavc/` whole with the same parts and change the digest in `yavc_release_test.go` and the table above. yavc.app's `docs/content-release-downloads.md` describes the assets. Nothing in `yavc/` is edited by hand; a change goes to yavc.app.
