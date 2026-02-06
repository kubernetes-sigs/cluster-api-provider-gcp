# Release Process

## Change milestone

 - Create a new GitHub milestone for the next release
 - Change milestone applier so new changes can be applied to the appropriate release
      - Open a PR in https://github.com/kubernetes/test-infra to change this [line](https://github.com/kubernetes/test-infra/blob/25db54eb9d52e08c16b3601726d8f154f8741025/config/prow/plugins.yaml#L344)
        - Example PR: https://github.com/kubernetes/test-infra/pull/16827

## Ensure that CI is stable
 
 1. Before releasing always ensure CI is stable. Check [Prow CAPG dashboard](https://prow.k8s.io/?repo=kubernetes-sigs%2Fcluster-api-provider-gcp)

## Ensure you have a GITHUB_TOKEN and are logged into GCP

 1. If you don't have a GitHub token, create one by going to your GitHub settings in [Personal access tokens](https://github.com/settings/tokens). Make sure you give the token the `repo` scope. If you have one, make sure it has the right scope and it is not expired.
 
 1. Configure gcloud authentication:

    ```bash
    gcloud auth login <your-community-email-address>
    ```

## Create the branch, new version tag, staging image

 1. Please fork `https://github.com/kubernetes-sigs/cluster-api-provider-gcp` and clone your own repository with e.g. `git clone git@github.com:YourGitHubUsername/cluster-api-provider-gcp.git`. kpromo uses the fork to build images from.

 1. Add a git remote to the upstream project. git remote add upstream `git@github.com:kubernetes-sigs/cluster-api-provider-gcp.git`

 1. If this is a major or minor release, create a new release branch and push to GitHub, otherwise switch to it, e.g. `git checkout release-1.7`.

 1. If this is a major or minor release, update metadata.yaml by adding a new section with the version, and make a commit.

 1. Update the release branch on the repository, e.g. `git push origin HEAD:release-1.7`. origin refers to the remote git reference to your fork.

 1. Update the release branch on the repository, e.g. git push upstream HEAD:release-1.7. upstream refers to the upstream git reference.

 1. Make sure your repo is clean by git standards.

 1. Set the environment variable VERSION which is the current release that you are making, e.g. `export VERSION=v1.7.0`, or `export VERSION=v1.7.1`). Note: the version MUST contain a v in front. Note: you must have a gpg signing configured with git and registered with GitHub.

 1. Create a tag locally `git tag -s -a $VERSION -m $VERSION` -s flag is for GNU Privacy Guard (GPG) signing.

 1. Make sure you have push permissions to the upstream CAPG repo. Push tag you've just created `git push <upstream-repo-remote> $VERSION`. `<upstream-repo-remote>` must be the remote pointing to `github.com/kubernetes-sigs/cluster-api-provider-gcp`.
   
 1. Pushing this will create the tag and this will automatically trigger a [ProwJob](https://prow.k8s.io/?repo=kubernetes-sigs%2Fcluster-api-provider-gcp&job=post-cluster-api-provider-gcp-push-images) to publish images to the staging image repository.


## Generate release manifests and release notes

1. `make release` from repo, this will create the release artifacts in the `out/` folder. It is recommended to verify that the artifact file `infrastructure-components.yaml` points to the new image.

1. Install the `release-notes` tool according to [instructions](https://github.com/kubernetes/release/blob/master/cmd/release-notes/README.md)

1. Generate release-notes (requires exported `GITHUB_TOKEN` variable, ensure the TOKEN is not expired!):

    Run the release-notes tool with the appropriate commits. Commits range from the first commit after the previous non-beta release to the newest commit of the release branch. Set branch to the release branch you are cutting this release from. For example if this is release `v1.11.z`, branch is going to be `release-1.11`. When this finishes it will log the path to the temporary file where the notes have been written.

    ```bash
    release-notes --org kubernetes-sigs --repo cluster-api-provider-gcp \
    --start-sha 1cf1ec4a1effd9340fe7370ab45b173a4979dc8f  \
    --end-sha e843409f896981185ca31d6b4a4c939f27d975de
    --branch <NEW_RELEASE_BRANCH_OR_MAIN_BRANCH>
    ```

1. Open the output temporary file logged by the tool and manually format and categorize the release notes

## Prepare release in GitHub

Create the GitHub release in the UI
 - Go to: https://github.com/kubernetes-sigs/cluster-api-provider-gcp/releases  
 - Create a draft release with the output from above in GitHub and associate it with the tag that was created
 - Copy paste the release notes
 - Upload [artifacts](#expected-artifacts) from the `out/` folder
 - Leave everything unchecked and click "Save Draft"

## Promote image to prod repo

Images are built by the [push images job](https://prow.k8s.io/?repo=kubernetes-sigs%2Fcluster-api-provider-gcp&job=post-cluster-api-provider-gcp-push-images) after pushing a tag.

To promote images from the staging repository to the production registry (`registry.k8s.io/cluster-api-provider-gcp`):

  1. Wait until images for the tag have been built and pushed to the [staging repository](https://console.cloud.google.com/gcr/images/k8s-staging-cluster-api-gcp/global/cluster-api-gcp-controller) by
      the [push images job](https://prow.k8s.io/?repo=kubernetes-sigs%2Fcluster-api-provider-gcp&job=post-cluster-api-provider-gcp-push-images).

  1. If you don't have a GitHub token, create one by going to your GitHub settings in [Personal access tokens](https://github.com/settings/tokens). Make sure you give the token the `repo` scope.

  1. Create a PR to promote the images to the production registry:

      ```bash
      # Export the tag of the release to be cut, e.g.:
      export VERSION=v1.11.0-beta.0
      export GITHUB_TOKEN=<your GH token>
      make promote-images
      ```

      **Notes**:
       - `make promote-images` target tries to figure out your Github user handle in order to find the forked [k8s.io](https://github.com/kubernetes/k8s.io) repository.
         If you have not forked the repo, please do it before running the Makefile target.
       - if `make promote-images` fails with an error like `FATAL while checking fork of kubernetes/k8s.io` you may be able to solve it by manually setting the USER_FORK variable
         i.e. `export USER_FORK=<personal GitHub handle>`.
       - `kpromo` uses `git@github.com:...` as remote to push the branch for the PR. If you don't have `ssh` set up you can configure
         git to use `https` instead via `git config --global url."https://github.com/".insteadOf git@github.com:`.
       - This will automatically create a PR in [k8s.io](https://github.com/kubernetes/k8s.io) and assign the CAPV maintainers.
  1. Merge the PR (/lgtm + /hold cancel) and verify the images are available in the production registry:
    - Wait for the [promotion prow job](https://prow.k8s.io/?repo=kubernetes%2Fk8s.io&job=post-k8sio-image-promo) to complete successfully. Then verify that the production images are accessible:

     ```bash
     docker pull registry.k8s.io/cluster-api-gcp/cluster-api-gcp-controller:${VERSION}
     ```

<aside class="note">

<h1>Tip</h1>

You can use the following [sample PR](https://github.com/kubernetes/k8s.io/pull/1462)

</aside>

Location of image: https://console.cloud.google.com/gcr/images/k8s-staging-cluster-api-gcp/GLOBAL/cluster-api-gcp-controller?rImageListsize=30

## Release in GitHub

Go back to the GitHub release in the UI

 - Edit the draft release previously created in GitHub Releases
 - Check the "Set as the latest release" checkbox
 - Check the "Create a Discussion for this release" in Announcements checkbox
 - ONLY CHECK the "Set as a pre-release" checkbox IF it is a `vXX.XX.XX-beta.X` release
 - Now, hit "Publish release"
 - [Announce][release-announcement] the release

## Versioning

cluster-api-provider-gcp follows the [semantic versioning][semver] specification.

Example versions:
- Pre-release: `v0.1.1-alpha.1`
- Minor release: `v0.1.0`
- Patch release: `v0.1.1`
- Major release: `v1.0.0`

The major and minor versions mirror the version of Cluster-API.  Thus CAPG 1.11.x releases will use CAPI versions 1.11

The patch version is incremented for each release.  Thus the first release of CAPG using CAPI 1.11 will 1.11.0, the second will be 1.11.1, etc

## Branches

We maintain release branches to facilitate backporting fixes.  These branches are named `release-<major>.<minor>`.  Patch releases are tagged from the corresponding release branch.

Feature development happens on the `main` branch, and [nightlies](nightlies.md) are released from the `main` branch also.
While we aim to keep the `main` branch healthy, it is intended for development and testing, and should not be used in production.

As such, we will aim to keep the main branch up to date with the latest versions of all dependencies (go toolchain, CAPI, other go module dependencies).

Release branches lock to a particular minor version of CAPI, but will update other dependencies on these branches while they are supported.
Currently we maintain only one release branch (and the `main` branch).  Once a branch is no longer supported there will be no new releases
of the corresponding minor, including no further security fixes.

## Expected artifacts

1. A release yaml file `infrastructure-components.yaml` containing the resources needed to deploy to Kubernetes
1. A `cluster-templates.yaml` for each supported flavor
1. A `metadata.yaml` which maps release series to cluster-api contract version
1. Release notes

## Communication

### Patch Releases

1. Announce the release in Kubernetes Slack on the #cluster-api-gcp channel.

### Minor/Major Releases

1. Follow the communications process for [pre-releases](#pre-releases)
1. An announcement email is sent to `sig-cluster-lifecycle@kubernetes.io` with the subject `[ANNOUNCE] cluster-api-provider-gcp <version> has been released`

[release-announcement]: #communication
[semver]: https://semver.org/#semantic-versioning-200
