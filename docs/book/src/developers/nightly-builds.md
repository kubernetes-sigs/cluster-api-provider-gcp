# Nightly Builds

Nightly builds are regular automated builds of the CAPG source code that occur every night.

These builds are generated directly from the latest commit of source code on the main branch.

Nightly builds serve several purposes:

- **Early Testing**: They provide an opportunity for developers and testers to access the most recent changes in the codebase and identify any issues or bugs that may have been introduced.
- **Feedback Loop**: They facilitate a rapid feedback loop, enabling developers to receive feedback on their changes quickly, allowing them to iterate and improve the code more efficiently.
- **Preview of New Features**: Users can get a preview of upcoming features or changes by testing nightly builds, although these builds may not always be stable enough for production use.

Overall, nightly builds play a crucial role in software development by promoting user testing, early bug detection, and rapid iteration.

CAPG Nightly build jobs run in Prow.

## Usage

To try a nightly build, you can download the latest built nightly CAPG manifests, you can find the available ones by executing the following command:
```bash
curl -sL "https://storage.googleapis.com/storage/v1/b/k8s-staging-cluster-api-gcp/o?prefix=components/nightly_main_&delimiter=/" | jq -r '.prefixes[]'
```
The output should look something like this:
```
components/nightly_main_20261118/
components/nightly_main_20261119/
components/nightly_main_20261120/
```

Then run through the following steps:

1. Run through the [Setup Tutorial](./setup-tutorial.md) to get a working Tilt setup
2. Run `export GCP_B64ENCODED_CREDENTIALS=$(jq -er .kustomize_substitutions.GCP_B64ENCODED_CREDENTIALS tilt-settings.json)`
3. Ctrl-C `make tilt-up`, this will leave the cluster running but Tilt will not be managing the cluster, allowing
  you to set the nightly build.
4. Run `make apply-nightly`
   - You can use `NIGHTLY=` and `KUBE_CONTEXT=` as overrides if required. Make sure to set NIGHTLY correctly by referring
     to the list returned from the `curl` call above.
5. Once you're done run `make tilt-up` again and it will restore your checkout.
