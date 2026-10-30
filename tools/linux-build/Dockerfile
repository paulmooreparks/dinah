# The build environment of Dinah for Linux: Go and the libraries the Wails
# webview links against (GTK 3 and WebKitGTK 4.1), on Debian 12. The
# repository is mounted at /src when the image runs; build-linux.ps1 beside
# this file builds the image and runs it.
FROM golang:1.26-bookworm
RUN apt-get update \
 && apt-get install -y --no-install-recommends build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev \
 && rm -rf /var/lib/apt/lists/*
ENV GOTOOLCHAIN=auto CGO_ENABLED=1
WORKDIR /src
