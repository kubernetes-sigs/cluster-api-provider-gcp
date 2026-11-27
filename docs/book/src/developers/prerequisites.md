# Development Prerequisites

<aside class="note">
As the project lacks a lot of features for Windows, it would be suggested to follow the steps below on Windows + WSL2
rather than Windows.
</aside>

## Base requirements

1. Install [go][go]
   - Get the version specified in the [go directive](https://github.com/kubernetes-sigs/cluster-api-provider-gcp/blob/main/go.mod).
2. Install [jq][jq]
   - `brew install jq` on macOS.
   - `sudo apt install jq` on Windows + WSL2.
   - `sudo apt install jq` on Ubuntu Linux.
3. Install [docker][docker]
4. Install [gcloud][gcloud]
5. Install Python 3.x, if neither is already installed.
6. Install make.
   - Install Xcode Command Line Tools on macOS
   - `sudo apt install make` on Windows + WSL2.
   - `sudo apt install make` on Linux.
7. Install [timeout][timeout]
   - `brew install coreutils` on macOS.
8. Install [Tilt][tilt]

## Setting up the environment

In order to run against a real GCP Account you must pass real GCP credentials to your setup. See our [Quick Start Guide](../quick-start.md#credentials)
for details of how to generate these credentials.

Once they have been generated keep them handy as Tilt will require the `base64` encoded form of the credentials to work correctly.

[docker]: https://docs.docker.com/engine/install/
[gcloud]: https://docs.cloud.google.com/sdk/docs/install-sdk
[go]: https://golang.org/doc/install
[jq]: https://stedolan.github.io/jq/download/
[tilt]: https://docs.tilt.dev/install.html
[timeout]: http://man7.org/linux/man-pages/man1/timeout.1.html
