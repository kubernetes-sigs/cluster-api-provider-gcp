# Release Process

## Change milestone

 - Create a new GitHub milestone for the next release
 - Change milestone applier so new changes can be applied to the appropriate release
      - Open a PR in https://github.com/kubernetes/test-infra to change this [line](https://github.com/kubernetes/test-infra/blob/25db54eb9d52e08c16b3601726d8f154f8741025/config/prow/plugins.yaml#L344)
        - Example PR: https://github.com/kubernetes/test-infra/pull/16827

## Prepare branch

TODO

## Prepare branch, tag and release notes

 1. Update the file `metadata.yaml` if is a major or minor release

 2. Submit a PR for the `metadata.yaml` update if needed, wait for it to be merged before continuing, and pull any changes prior to continuing.
 
 3. Create and push the release tags to the GitHub repository:

  ```bash
    # Export the tag of the release to be cut, e.g.:
    export RELEASE_TAG=v1.11.0-beta.0
    # Create tags locally
    git tag -s -a ${RELEASE_TAG} -m ${RELEASE_TAG}

    # Push tags
    # Note: `upstream` must be the remote pointing to `github.com/kubernetes-sigs/cluster-api-provider-gcp`.
    git push upstream ${RELEASE_TAG}
  ```

  Notes:

  * `-s` creates a signed tag, you must have a GPG key [added to your GitHub account](https://docs.github.com/en/authentication/managing-commit-signature-verification/generating-a-new-gpg-key)
  * This will automatically trigger a [ProwJob](https://prow.k8s.io/?repo=kubernetes-sigs%2Fcluster-api-provider-gcp&job=post-cluster-api-provider-gcp-push-images) to publish images to the staging image repository.

4. Configure gcloud authentication:

* `glcoud auth login <your-community-email-address>

5. `make release` from repo, this will create the release artifacts in the `out/` folder


6. Install the `release-notes` tool according to [instructions](https://github.com/kubernetes/release/blob/master/cmd/release-notes/README.md)

7. Generate release-notes (require's exported `GITHUB_TOKEN` variable):

  Run the release-notes tool with the appropriate commits. Commits range from the first commit after the previous release to the new release commit.

  ```bash
  release-notes --org kubernetes-sigs --repo cluster-api-provider-gcp \
  --start-sha 1cf1ec4a1effd9340fe7370ab45b173a4979dc8f  \
  --end-sha e843409f896981185ca31d6b4a4c939f27d975de
  --branch <RELEASE_BRANCH_OR_MAIN_BRANCH>
  ```

8. Manually format and categorize the release notes

## Prepare release in GitHub

Create the GitHub release in the UI

 - Create a draft release with the output from above in GitHub and associate it with the tag that was created
 - Copy paste the release notes
 - Upload [artifacts](#expected-artifacts) from the `out/` folder

## Promote image to prod repo

Images are built by the [push images job](https://prow.k8s.io/?repo=kubernetes-sigs%2Fcluster-api-provider-gcp&job=post-cluster-api-provider-gcp-push-images) after pushing a tag.

To promote images from the staging repository to the production registry (`registry.k8s.io/cluster-api-provider-gcp`):

  1. Wait until images for the tag have been built and pushed to the [staging repository](https://console.cloud.google.com/gcr/images/k8s-staging-cluster-api-gcp/global/cluster-api-gcp-controller) by
      the [push images job](https://prow.k8s.io/?repo=kubernetes-sigs%2Fcluster-api-provider-gcp&job=post-cluster-api-provider-gcp-push-images).

   2. If you don't have a GitHub token, create one by going to your GitHub settings in [Personal access tokens](https://github.com/settings/tokens). Make sure you give the token the `repo` scope.

   3. Create a PR to promote the images to the production registry:

      ```bash
      # Export the tag of the release to be cut, e.g.:
      export RELEASE_TAG=v1.11.0-beta.0
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
4. Merge the PR (/lgtm + /hold cancel) and verify the images are available in the production registry:
    - Wait for the [promotion prow job](https://prow.k8s.io/?repo=kubernetes%2Fk8s.io&job=post-k8sio-image-promo) to complete successfully. Then verify that the production images are accessible:

     ```bash
     docker pull registry.k8s.io/cluster-api-provider-gcp/cluster-api-gcp-controller:${RELEASE_TAG}
     ```

Example PR: https://github.com/kubernetes/k8s.io/pull/1462

Location of image: https://console.cloud.google.com/gcr/images/k8s-staging-cluster-api-gcp/GLOBAL/cluster-api-gcp-controller?rImageListsize=30

## Release in GitHub

Create the GitHub release in the UI

 - Create a draft release in GitHub and associate it with the tag that was created
 - Copy paste the release notes
 - Upload [artifacts](#expected-artifacts) from the `out/` folder
 - Publish release
 - [Announce][release-announcement] the release

## Versioning

cluster-api-provider-gcp follows the [semantic versioning][semver] specification.

Example versions:
- Pre-release: `v0.1.1-alpha.1`
- Minor release: `v0.1.0`
- Patch release: `v0.1.1`
- Major release: `v1.0.0`

## Expected artifacts

1. A release yaml file `infrastructure-components.yaml` containing the resources needed to deploy to Kubernetes
2. A `cluster-templates.yaml` for each supported flavor
3. A `metadata.yaml` which maps release series to cluster-api contract version
4. Release notes

## Communication

### Patch Releases

1. Announce the release in Kubernetes Slack on the #cluster-api-gcp channel.

### Minor/Major Releases

1. Follow the communications process for [pre-releases](#pre-releases)
2. An announcement email is sent to `kubernetes-sig-cluster-lifecycle@googlegroups.com` with the subject `[ANNOUNCE] cluster-api-provider-gcp <version> has been released`

[release-announcement]: #communication
[semver]: https://semver.org/#semantic-versioning-200
