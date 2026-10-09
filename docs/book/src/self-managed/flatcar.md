# Use Flatcar images

[Flatcar](https://flatcar.org) is a Linux based OS designed to run containers.

## How do I use Flatcar?

Flatcar uses [Ignition](https://coreos.github.io/ignition/) for initial provisioning instead of cloud-init. It is first required to enable this feature
gate before initializing the management cluster. Note this is a Cluster API feature-gate, not one specific to this
provider:
```bash
export EXP_KUBEADM_BOOTSTRAP_FORMAT_IGNITION=true
```

Once done, proceed as documented to setup GCP variables. To set the `IMAGE_ID`, use the snippet below:
```
export IMAGE_ID="projects/kinvolk-public/global/images/family/flatcar-lts"
```

## Generate the workload cluster configuration

Proceed as usual except for the flavor:
```
clusterctl generate cluster capi-gcp-quickstart --flavor flatcar > capi-gcp-quickstart.yaml
```

## Kubernetes Versions
The `KUBERNETES_VERSION` that is chosen must have a corresponding published extension at https://extensions.flatcar.org

## Upgrades

### Upgrading Kubernetes
As is standard with Cluster API you should always update the control plane first, followed by the workers. As such there
are several steps to follow to make sure this works correctly:

1. Edit the `KubeadmControlPlane` object, changing the `version` field and the version that's embedded in the following
three strings, all nested under `spec.kubeadmConfigSpec.ignition.containerLinuxConfig.additionalConfig`:
   1. `storage.links[0].target`
   2. `storage.files[0].path`
   3. `storage.files[0].contents.remote.url`

2. Once the control plane is updated you can then update the workers by creating a new `KubeadmConfigTemplate` where
the version embedded in the three strings below is updated. All these strings are nested under
`spec.template.spec.ignition.containerLinuxConfig.additionalConfig`:
   1. `storage.links[0].target`
   2. `storage.files[0].path`
   3. `storage.files[0].contents.remote.url`

3. Then update the version in the `MachineDeployment` CR as well as pointing the `MachineDeployment` at the new version
of the `KubeadmConfigTemplate`.

4. Let the worker nodes rollout

### Upgrading Flatcar

Flatcar auto-updates are disabled by default. To enable Flatcar OS upgrade set `FLATCAR_DISABLE_AUTO_UPDATE=false`.

Note that this will reboot your nodes: [`kured`](https://kured.dev/) is recommended to coordinate the nodes reboot.

## Size Limitations
It's worth noting that GCE limits metadata to 256 KB per value and 512 KB in total. This is not a limiting factor for the
published templates as they reference remote files but anything that is subsequently inlined will add to this total so
be careful if you are making changes in this area, or customizing the config.