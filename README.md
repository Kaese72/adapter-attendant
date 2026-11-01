# adapter-attendant

Keeps track of the device adapters on a Humi appliance: which image each one
runs, and its configuration. It never deploys anything itself, and never
talks to the Kubernetes API at all. ArgoCD deploys each adapter once this
service lists it, the same way appliance-registry has ArgoCD deploy a
cloud-connect-server per appliance in the cloud.

## Deployment

```
huemie-ui ──POST /adapters/{id}/sync──► this service ──► DB: syncedImage, syncedEnvironment, syncGeneration++
ArgoCD applicationset-controller ──getparams──► this service (internal port)
   └─► Application adapter-<id> ──► adapters ns: Secret + Deployment + Service adapter-<id>
```

* **Deploy (sync) is the switch.** Editing an adapter or its arguments
  changes only the database. Deploying records a snapshot of what to run:
  the image, and the adapter's whole environment (the arguments as
  `ADAPTER_*`, plus `HUEMIE_ENROLL_STORE` and a freshly minted
  `HUEMIE_ENROLL_TOKEN`). The plugin endpoint lists every adapter with a
  snapshot, so edits only take effect on the next deploy.
* **The environment goes through ArgoCD, credentials included.** The plugin
  answers `id`, `image`, `syncGeneration` and `secretData` (the environment,
  base64-encoded), and the ApplicationSet renders the adapter's Secret from
  it. That puts credentials in plain text on each generated Application,
  which is acceptable while only cluster admins can read Applications; the
  appliance doesn't expose ArgoCD at all.
* **Every deploy restarts the adapter.** Adapters read their environment
  only at startup. `syncGeneration` is bumped on every deploy and put on the
  pod template.
* **No failure ever answers with an empty parameter list.** ArgoCD deletes
  generated Applications whose parameters disappear. A database error, or an
  unreadable snapshot, is a 500, on which ArgoCD leaves every Application
  alone.
* **Deleting an adapter** drops it from the plugin's answer, so ArgoCD removes
  its Deployment, Service and Secret.
* **Adapters deployed before ArgoCD did it** (directly through the
  Kubernetes API, configured from a ConfigMap) keep running untouched until
  they are next deployed, after which ArgoCD takes over the existing
  Deployment and Service in place. Their old `adapter-<id>` ConfigMap holds
  credentials in plain text and isn't removed by anything: delete it by hand.

See huemie-gitops-base's `appliance/argocd/applicationset-adapters.yaml` and
`appliance/base/adapter-instance/`.

## Configuration

| Environment variable | Default | |
|---|---|---|
| `AUTH_RSA_PUBLIC_KEY_PATH` | | authentication's public key, for use tokens |
| `AUTH_PLUGIN_TOKENS` | | comma-separated; the plugin token ArgoCD presents |
| `KUBERNETES_ADAPTER_NAMESPACE` | | where ArgoCD deploys the adapters; only used to build their addresses |
| `ADAPTERS_DEVICE_STORE_URL` | | handed to adapters as `HUEMIE_ENROLL_STORE` |
| `ADAPTERS_DEVICE_STORE_JWT_SECRET` | | signs adapters' enroll tokens |
| `DATABASE_HOST` / `DATABASE_USER` / `DATABASE_PASSWORD` | | |
| `PUBLIC_PORT` / `INTERNAL_PORT` | `8080` / `8081` | |

Internal port (8081), reachable only by device-store and ArgoCD's
applicationset-controller:

| | |
|---|---|
| `GET /adapter-attendant-internal/v1/adapters/{id}/address` | the adapter's in-cluster address, for device-store |
| `POST /adapter-attendant-internal/v1/argocd-plugin/api/v1/getparams.execute` | ArgoCD Plugin generator, bearer plugin token |
