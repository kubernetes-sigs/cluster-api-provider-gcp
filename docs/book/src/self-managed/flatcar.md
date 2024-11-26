# Use Flatcar images

[Flatcar](https://flatcar.org) is a Linux based OS designed to run containers.

## How do I use Flatcar ?

Flatcar uses [Ignition](https://coreos.github.io/ignition/) for initial provisioning instead of cloud-init. It is first required to enable this feature gate before initializing the management cluster:
```bash
export EXP_KUBEADM_BOOTSTRAP_FORMAT_IGNITION=true
```

Once done, proceed as documented to setup GCP variables. To set the `IMAGE_ID`, use this snippet to get the latest stable Flatcar image:
```
VERSION=$(curl -fsSL https://stable.release.flatcar-linux.net/amd64-usr/current/version.txt | grep --max-count=1 FLATCAR_VERSION | cut -d = -f 2- | tr '.' '-')
export IMAGE_ID="projects/kinvolk-public/global/images/flatcar-stable-${VERSION}"
```

## Generate the workload cluster configuration

Proceed as usual except for the flavor:
```
clusterctl generate cluster capi-gcp-quickstart --flavor flatcar > capi-gcp-quickstart.yaml
```

## Updates configuration

Flatcar auto-update and Kubernetes patch updates are disabled by default. Set `export FLATCAR_DISABLE_AUTO_UPDATE=false` to enable it. This will pull latest Flatcar update and latest Kubernetes patch release. Note that this will reboot your nodes: [`kured`](https://kured.dev/) is recommended to coordinate the nodes reboot.
