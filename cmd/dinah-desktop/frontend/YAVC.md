# yavc.app content release

`yavc/` holds the portable Editor, Docs, Diagrams and Slides applets from yavc.app, as one release set, unchanged. The plan for adopting them is [docs/design/content-applets.md](../../../docs/design/content-applets.md).

| Item | Value |
| --- | --- |
| Producer | https://github.com/paulmooreparks/yavc.app |
| Release | v0.2.125, commit `9754cb632d8d245155a6b1db00cc47780bdbbe57`, downloaded from the release's assets (`yavc-content-v0.2.125.zip`, SHA-256 `95991aea702f7e3bb64b45325f91cd9ad1ac85cfdb9369b2b9c250ea6bf084a1`), checked with yavc.app's `scripts/release_record.py` against the release record pinned on the bus (SHA-256 `2a1a50f8687a7830f4e05f02d624e9ce6d99070b434580565314594128383e08`) and against GitHub's build attestation |
| Manifest digest | `f62ff278144b260323068277bc75715a868caabfdd7e3d3496c4f93f34825a3b` |
| Applets | Editor 46.37.57, Docs 1.37.57, Diagrams 0.46.15, Slides 0.65.10 |
| PUDL | 0.112.0, the same release Dinah ships in `pudl/` |

The release's own `manifest.json` is kept beside it. Dinah ships the release's `content/`, `support/` and `LICENSE`. It leaves out the release's `pudl/`, because Dinah's copy is the same release byte for byte, and its demonstration hosts (`lab/`, `workspace-lab/`) with their `go.mod` and `go.sum`. `yavc_release_test.go` checks the aggregate digest, and the length and SHA-256 of every file, against Dinah's tree and Dinah's PUDL.

To take another release, download its three assets (`yavc-content-VERSION.zip`, `.manifest.json` and `.sha256`) from the release at https://github.com/paulmooreparks/yavc.app/releases, check the ZIP's SHA-256 against the published one, extract it into an empty folder, compare the archive's `manifest.json` with the published manifest and verify the digest and every file, then replace `yavc/` whole with the same parts and change the digest in `yavc_release_test.go` and the table above. yavc.app's `docs/content-release-downloads.md` describes the assets. Nothing in `yavc/` is edited by hand; a change goes to yavc.app.
