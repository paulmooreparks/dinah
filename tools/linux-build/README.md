# Building Dinah for Linux

The window, `dinah-desktop`, links the Linux webview (GTK 3 and WebKitGTK 4.1) through C, so it cannot be cross-compiled from Windows the way the command line can. This folder builds both programs for Linux on amd64 in a Debian 12 container, which needs Docker.

```powershell
pwsh tools/linux-build/build-linux.ps1 [-OutDir <folder>] [-Version 1.2.0] [-Test]
```

The script builds the image from the `Dockerfile` here, mounts the repository at `/src`, and writes `dinah-linux-amd64` and `dinah-desktop-linux-amd64` to `-OutDir`, which defaults to the top of the repository and has to be inside it. `-Version` stamps both programs with a release version. `-Test` runs `go vet` and `go test` on Linux in the same container before building. Go's module and build caches are kept in the Docker volumes `dinah-go-mod-cache` and `dinah-go-build-cache`, so later builds are quicker.

When `go.mod` replaces a module with a local folder, such as a sibling checkout of `dinah-extension-go`, the script mounts that folder at the same place relative to `/src`, and it stops with a message if the folder is missing. In a git worktree the container cannot reach the main repository, so the build does not record the commit; the About box then shows the version without it.

To run the window, a Linux machine needs GTK 3 and WebKitGTK 4.1 installed. On Debian 12, or Ubuntu 22.04 and later, that is `sudo apt install libgtk-3-0 libwebkit2gtk-4.1-0`. The command line is pure Go and needs nothing installed.
