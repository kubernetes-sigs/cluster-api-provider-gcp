# Prerequisites

### Base requirements

1. Install [go][go]
   - Get the latest patch version for go v1.18.
2. Install [jq][jq]
   - `brew install jq` on macOS.
   - `sudo apt install jq` on Windows + WSL2.
   - `sudo apt install jq` on Ubuntu Linux.
3. Install [gettext][gettext] package
   - `brew install gettext && brew link --force gettext` on macOS.
   - `sudo apt install gettext` on Windows + WSL2.
   - `sudo apt install gettext` on Ubuntu Linux.
4. Install [KinD][kind]
   - `GO111MODULE="on" go get sigs.k8s.io/kind@v0.14.0`.
5. Install [Kustomize][kustomize]
   - `brew install kustomize` on macOS.
   - [install instructions](https://kubectl.docs.kubernetes.io/installation/kustomize/) on Windows + WSL2, Linux and macOS.
6. Install Python 3.x, if neither is already installed.
7. Install make.
   - `brew install make` on MacOS.
   - `sudo apt install make` on Windows + WSL2.
   - `sudo apt install make` on Linux.
8. Install [timeout][timeout]
   - `brew install coreutils` on macOS.

When developing on Windows, it is suggested to set up the project on Windows + WSL2 and the file should be checked out on as wsl file system for better results.

### Setting up the environment

Your environment must have the GCP credentials, check [Authentication Getting Started](https://cloud.google.com/docs/authentication/getting-started)

### Tilt Requirements

Install [Tilt][tilt]:

- `brew install tilt-dev/tap/tilt` on macOS or Linux
- `scoop bucket add tilt-dev https://github.com/tilt-dev/scoop-bucket` & `scoop install tilt` on Windows

After the installation is done, verify that you have installed it correctly with: `tilt version`

Install [Helm](https://helm.sh/docs/intro/install/):

- `brew install helm` on MacOS
- `choco install kubernetes-helm` on Windows
- [Install instructions](https://helm.sh/docs/intro/install/#from-source-linux-macos) for Linux

As the project lacks a lot of feature for windows, it would be suggested to follow the above steps on Windows + WSL2
rather than Windows.

[go]: https://golang.org/doc/install
[tilt]: https://docs.tilt.dev/install.html
[jq]: https://stedolan.github.io/jq/download/
[gettext]: https://www.gnu.org/software/gettext/
[kind]: https://sigs.k8s.io/kind
[kustomize]: https://github.com/kubernetes-sigs/kustomize
[timeout]: http://man7.org/linux/man-pages/man1/timeout.1.html
