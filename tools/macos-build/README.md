# Building Dinah for macOS

The window links Apple's WebKit through Apple's SDK, which Apple licenses for use on Apple hardware only, so the Mac app has to be built on a Mac. These scripts have not yet been run on one.

`install-macos.sh` is the one-step route. It needs nothing installed beforehand but macOS, and no administrator rights:

```sh
sh tools/macos-build/install-macos.sh [--no-open]
```

It asks macOS to install Xcode's command line tools if they are missing, downloads the Go release that `go.mod` asks for into `~/.dinah/toolchain` after checking go.dev's published checksum, and installs the Wails command line there at the version `go.mod` uses. It then builds and installs `Dinah.app` in `~/Applications` and the `dinah` command line in `~/.dinah/bin`, which it adds to `PATH` in `~/.zprofile`. Running it again updates both. It stops without installing while Dinah is running.

`build-macos.sh` builds the app alone, for someone who already has Go, Xcode's command line tools and the Wails command line:

```sh
sh tools/macos-build/build-macos.sh [version]
```

It runs `wails build` in `cmd/dinah-desktop`, which makes the bundle from `wails.json` and `build/appicon.png`, signs it ad hoc so that a Mac with Apple silicon will run it, and writes it to `cmd/dinah-desktop/build/bin/Dinah.app`. `PLATFORM=darwin/universal` builds one bundle for both Apple silicon and Intel; by default it builds for the Mac's own processor.

When `go.mod` replaces a module with a local folder, such as a sibling checkout of `dinah-extension-go`, that folder has to be beside the repository on the Mac too.
