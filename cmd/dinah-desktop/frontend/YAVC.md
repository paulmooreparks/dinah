# yavc.app content release

`yavc/` holds the portable Editor, Docs, Diagrams and Slides applets from yavc.app, as one release set, unchanged. The plan for adopting them is [docs/design/content-applets.md](../../../docs/design/content-applets.md).

| Item | Value |
| --- | --- |
| Producer | https://github.com/paulmooreparks/yavc.app |
| Release | v0.2.115, commit `faa1a8eb9aafa99caa62196458227abd6994f33f`, downloaded from the release's assets (`yavc-content-v0.2.115.zip`, SHA-256 `229fe07ca5118550987819436c617819f6ef0a7693507de811b3facf524c328c`), checked with yavc.app's `scripts/release_record.py` against the release record pinned on the bus (SHA-256 `9fc603d4287196922453cf081a0b3f502842499c5bdf8c8e2624341aa4c68c7e`) and against GitHub's build attestation |
| Manifest digest | `94e436f98e738bc12bf2e95afac2e830cbf065732a03eea93cacf1d9c1c02535` |
| Applets | Editor 46.37.55, Docs 1.37.55, Diagrams 0.46.14, Slides 0.65.9 |
| PUDL | 0.108.0, the same release Dinah ships in `pudl/` |

The release's own `manifest.json` is kept beside it. Dinah ships the release's `content/`, `support/` and `LICENSE`. It leaves out the release's `pudl/`, because Dinah's copy is the same release byte for byte, and its demonstration hosts (`lab/`, `workspace-lab/`) with their `go.mod` and `go.sum`. `yavc_release_test.go` checks the aggregate digest, and the length and SHA-256 of every file, against Dinah's tree and Dinah's PUDL.

To take another release, download its three assets (`yavc-content-VERSION.zip`, `.manifest.json` and `.sha256`) from the release at https://github.com/paulmooreparks/yavc.app/releases, check the ZIP's SHA-256 against the published one, extract it into an empty folder, compare the archive's `manifest.json` with the published manifest and verify the digest and every file, then replace `yavc/` whole with the same parts and change the digest in `yavc_release_test.go` and the table above. yavc.app's `docs/content-release-downloads.md` describes the assets. Nothing in `yavc/` is edited by hand; a change goes to yavc.app.
