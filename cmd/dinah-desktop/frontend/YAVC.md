# yavc.app content release

`yavc/` holds the portable Editor, Docs, Diagrams and Slides applets from yavc.app, as one release set, unchanged. The plan for adopting them is [docs/design/content-applets.md](../../../docs/design/content-applets.md).

| Item | Value |
| --- | --- |
| Producer | https://github.com/paulmooreparks/yavc.app |
| Release | v0.2.78, commit `059576adb3b22bb38cb8ff97059769849b9ba2f3`, downloaded from the release's assets (`yavc-content-v0.2.78.zip`, SHA-256 `413670bc48f6b0dc3f35d37c24bd91e8d8e34b2a336aae02e4007af7316929fd`) |
| Manifest digest | `db890c2afb91a9676e3c322079281e893d0353a56f9c03bae0dd715c8277706e` |
| Applets | Editor 46.34.0, Docs 1.34.0, Diagrams 0.43.0, Slides 0.62.0 |
| PUDL | 0.107.1, the same release Dinah ships in `pudl/` |

The release's own `manifest.json` is kept beside it. Dinah ships the release's `content/`, `support/` and `LICENSE`. It leaves out the release's `pudl/`, because Dinah's copy is the same release byte for byte, and its demonstration hosts (`lab/`, `workspace-lab/`) with their `go.mod` and `go.sum`. `yavc_release_test.go` checks the aggregate digest, and the length and SHA-256 of every file, against Dinah's tree and Dinah's PUDL.

To take another release, download its three assets (`yavc-content-VERSION.zip`, `.manifest.json` and `.sha256`) from the release at https://github.com/paulmooreparks/yavc.app/releases, check the ZIP's SHA-256 against the published one, extract it into an empty folder, compare the archive's `manifest.json` with the published manifest and verify the digest and every file, then replace `yavc/` whole with the same parts and change the digest in `yavc_release_test.go` and the table above. yavc.app's `docs/content-release-downloads.md` describes the assets. Nothing in `yavc/` is edited by hand; a change goes to yavc.app.
