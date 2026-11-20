# -*- mode: Python -*-

tools_bin = "./hack/tools/bin"
kubectl_cmd = "./hack/tools/bin/kubectl"
kind_cmd = "./hack/tools/bin/kind"

#Add tools to path
os.putenv("PATH", os.getenv("PATH") + ":" + tools_bin)

update_settings(k8s_upsert_timeout_secs = 60)  # on first tilt up, often can take longer than 30 seconds

# set defaults
settings = {
    "allowed_contexts": [
        "kind-capg",
    ],
    "preload_images_for_kind": True,
    "kind_cluster_name": "capg",
    "capi_version": os.getenv("CAPI_VERSION"),
}

keys = ["GCP_B64ENCODED_CREDENTIALS"]

# global settings
settings.update(read_json(
    "tilt-settings.json",
    default = {},
))

if not settings.get("capi_version"):
    fail("Cluster API Version not set. Please use `make tilt-up` or set `capi_version` in `tilt-settings.json`")

if not os.getenv("CLUSTERCTL"):
    fail("clusterctl path is not set. Please use `make tilt-up` or set the CLUSTERCTL environment variable to point at your `clusterctl` installation")

if settings.get("trigger_mode") == "manual":
    trigger_mode(TRIGGER_MODE_MANUAL)

if "allowed_contexts" in settings:
    allow_k8s_contexts(settings.get("allowed_contexts"))

if "default_registry" in settings:
    default_registry(settings.get("default_registry"))

# deploy CAPI
def deploy_capi():
    desired_version = settings.get("capi_version")
    kind_cluster_name = settings.get("kind_cluster_name")
    version_cmd = "{0} --context kind-{1} get providers.clusterctl.cluster.x-k8s.io -n capi-system cluster-api -o jsonpath='{{.version}}' || true".format(kubectl_cmd, kind_cluster_name)
    detected_version = str(local(version_cmd, quiet = True)).strip()

    if not detected_version:
        init_command = "{0} init --kubeconfig-context kind-{2} --core cluster-api:{1} --bootstrap kubeadm:{1} --control-plane kubeadm:{1} --wait-providers".format(os.getenv("CLUSTERCTL"), desired_version, kind_cluster_name)
        local(init_command, quiet = True)
    elif desired_version == detected_version:
        pass
    else:
        upgrade_command = "{0} upgrade apply --kubeconfig-context kind-{2} --core cluster-api:{1} --bootstrap kubeadm:{1} --control-plane kubeadm:{1} --wait-providers".format(os.getenv("CLUSTERCTL"), desired_version, kind_cluster_name)
        local(upgrade_command, quiet = True)

    if settings.get("extra_args"):
        extra_args = settings.get("extra_args")
        if extra_args.get("core"):
            core_extra_args = extra_args.get("core")
            if core_extra_args:
                for namespace in ["capi-system"]:
                    patch_args_with_extra_args(namespace, "capi-controller-manager", core_extra_args)
        if extra_args.get("kubeadm-bootstrap"):
            kb_extra_args = extra_args.get("kubeadm-bootstrap")
            if kb_extra_args:
                patch_args_with_extra_args("capi-kubeadm-bootstrap-system", "capi-kubeadm-bootstrap-controller-manager", kb_extra_args)

def patch_args_with_extra_args(namespace, name, extra_args):
    args_str = str(local("{} get deployments {} -n {} -o jsonpath={{.spec.template.spec.containers[0].args}}".format(kubectl_cmd, name, namespace)))
    args_to_add = [arg for arg in extra_args if arg not in args_str]
    if args_to_add:
        args = args_str[1:-1].split()
        args.extend(args_to_add)
        patch = [{
            "op": "replace",
            "path": "/spec/template/spec/containers/0/args",
            "value": args,
        }]
        local("{} patch deployment {} -n {} --type json -p='{}'".format(kubectl_cmd, name, namespace, str(encode_json(patch)).replace("\n", "")))

# Users may define their own Tilt customizations in tilt.d. This directory is excluded from git and these files will
# not be checked in to version control.
def include_user_tilt_files():
    user_tiltfiles = listdir("tilt.d")
    for f in user_tiltfiles:
        include(f)

def append_arg_for_container_in_deployment(yaml_stream, name, namespace, contains_image_name, args):
    for item in yaml_stream:
        if item["kind"] == "Deployment" and item.get("metadata").get("name") == name and item.get("metadata").get("namespace") == namespace:
            containers = item.get("spec").get("template").get("spec").get("containers")
            for container in containers:
                if contains_image_name in container.get("image"):
                    container.get("args").extend(args)

def setup_container_for_debug(yaml_stream, name, namespace):
    for item in yaml_stream:
        if item["kind"] == "Deployment" and item.get("metadata").get("name") == name and item.get("metadata").get("namespace") == namespace:
            containers = item.get("spec").get("template").get("spec").get("containers")
            for container in containers:
                if "manager" == container.get("name"):
                    for i, arg in enumerate(container.get("args")):
                        if arg == "--leader-elect":
                            container.get("args")[i] = "--leader-elect=false"
                    container.pop("livenessProbe", None)
                    container.pop("readinessProbe", None)

def fixup_yaml_empty_arrays(yaml_str):
    yaml_str = yaml_str.replace("conditions: null", "conditions: []")
    return yaml_str.replace("storedVersions: null", "storedVersions: []")

def validate_auth():
    substitutions = settings.get("kustomize_substitutions", {})
    missing = [k for k in keys if k not in substitutions]
    if missing:
        fail("missing kustomize_substitutions keys {} in tilt-settings.json".format(missing))

tilt_helper_dockerfile_header = """
# Tilt image
FROM golang:1.26.11 as tilt-helper
# Install delve. Note this should be kept in step with the Go release minor version.
RUN go install github.com/go-delve/delve/cmd/dlv@v1.26.3
# Support live reloading with Tilt
RUN wget --output-document /restart.sh --quiet https://raw.githubusercontent.com/tilt-dev/rerun-process-wrapper/master/restart.sh  && \
    wget --output-document /start.sh --quiet https://raw.githubusercontent.com/tilt-dev/rerun-process-wrapper/master/start.sh && \
    chmod +x /start.sh && chmod +x /restart.sh && chmod +x /go/bin/dlv
"""

manager_folder = "/app"
manager_location = "{}/manager".format(manager_folder)
manager_container_security_context = read_yaml_stream("config/manager/manager.yaml")[0]["spec"]["template"]["spec"]["containers"][0]["securityContext"]
uid = manager_container_security_context["runAsUser"]
gid = manager_container_security_context["runAsGroup"]

tilt_dockerfile_header = """
FROM golang:1.26.11 as tilt
WORKDIR /
COPY --from=tilt-helper /start.sh .
COPY --from=tilt-helper /restart.sh .
COPY --from=tilt-helper /go/bin/dlv .
RUN install -d -o {1} -g {2} {0}
WORKDIR {0}
COPY manager {0}
""".format(manager_folder, uid, gid)

# Build CAPG and add feature gates
def capg():
    # Get debug settings
    debug = settings.get("debug", {}).get("gcp", {})
    debug_port = int(debug.get("port", 0))

    # Apply the kustomized yaml for this provider
    substitutions = settings.get("kustomize_substitutions", {})
    os.environ.update(substitutions)

    # yaml = str(kustomizesub("./hack/observability")) # build an observable kind deployment by default
    yaml = str(kustomizesub("./config/default"))

    # add extra_args if they are defined
    if settings.get("extra_args") or debug_port != 0:
        yaml_dict = decode_yaml_stream(yaml)
        gcp_extra_args = settings.get("extra_args", {}).get("gcp")
        if gcp_extra_args:
            append_arg_for_container_in_deployment(yaml_dict, "capg-controller-manager", "capg-system", "cluster-api-gcp-controller", gcp_extra_args)
        if debug_port != 0:
            setup_container_for_debug(yaml_dict, "capg-controller-manager", "capg-system")
        yaml = str(encode_yaml_stream(yaml_dict))
        yaml = fixup_yaml_empty_arrays(yaml)

    # Set up a local_resource build of the provider's manager binary.
    arch = str(local("go env GOARCH", quiet = True)).strip()
    cmd = 'mkdir -p .tiltbuild;CGO_ENABLED=0 GOOS=linux GOARCH={} go build -ldflags \'-extldflags "-static"\''.format(arch)
    if debug_port != 0:
        cmd += " -gcflags 'all=-N -l'"
    cmd += " -o .tiltbuild/manager"
    local_resource(
        "manager",
        cmd = cmd,
        deps = ["api", "cloud", "config", "controllers", "exp", "feature", "pkg", "go.mod", "go.sum", "main.go"],
    )

    dockerfile_contents = "\n".join([
        tilt_helper_dockerfile_header,
        tilt_dockerfile_header,
    ])

    entrypoint = ["sh", "/start.sh", manager_location]
    if debug_port != 0:
        entrypoint = ["sh", "/start.sh", "/dlv", "--accept-multiclient", "--api-version=2", "--headless=true", "exec", "--listen=:30000"]
        if debug.get("continue", True):
            entrypoint += ["--continue"]
        entrypoint += ["--", manager_location]

    # Set up an image build for the provider. The live update configuration syncs the output from the local_resource
    # build into the container.
    docker_build(
        ref = "gcr.io/k8s-staging-cluster-api-gcp/cluster-api-gcp-controller",
        context = "./.tiltbuild/",
        dockerfile_contents = dockerfile_contents,
        target = "tilt",
        entrypoint = entrypoint,
        only = "manager",
        live_update = [
            sync(".tiltbuild/manager", manager_location),
            run("sh /restart.sh"),
        ],
        ignore = ["templates"],
    )

    k8s_yaml(blob(yaml))
    if debug_port != 0:
        k8s_resource("capg-controller-manager", port_forwards = [port_forward(debug_port, 30000)])

def base64_encode(to_encode):
    encode_blob = local("echo '{}' | tr -d '\n' | base64 - | tr -d '\n'".format(to_encode), quiet = True)
    return str(encode_blob)

def base64_encode_file(path_to_encode):
    encode_blob = local("cat {} | tr -d '\n' | base64 - | tr -d '\n'".format(path_to_encode), quiet = True)
    return str(encode_blob)

def read_file_from_path(path_to_read):
    str_blob = local("cat {} | tr -d '\n'".format(path_to_read), quiet = True)
    return str(str_blob)

def base64_decode(to_decode):
    decode_blob = local("echo '{}' | base64 --decode -".format(to_decode), quiet = True)
    return str(decode_blob)

def kustomizesub(folder):
    yaml = local("hack/kustomize-sub.sh {}".format(folder), quiet = True)
    return yaml

def waitforsystem():
    local(kubectl_cmd + " wait --for=condition=ready --timeout=300s pod --all -n capi-kubeadm-bootstrap-system")
    local(kubectl_cmd + " wait --for=condition=ready --timeout=300s pod --all -n capi-kubeadm-control-plane-system")
    local(kubectl_cmd + " wait --for=condition=ready --timeout=300s pod --all -n capi-system")

##############################
# Actual work happens here
##############################

validate_auth()

include_user_tilt_files()

deploy_capi()

capg()

waitforsystem()
