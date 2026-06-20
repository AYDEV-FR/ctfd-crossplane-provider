# ctfd Helm chart

Deploys [CTFd](https://ctfd.io/) for use with
[provider-ctfd](https://github.com/AYDEV-FR/ctfd-crossplane-provider). Its
headline feature: **install CTFd plugins from OCI images** using Kubernetes
[Image Volumes](https://kubernetes.io/docs/concepts/storage/volumes/#image) — no
rebuilding the CTFd image.

> **Scope:** this chart deploys a **single CTFd instance** — it is *not* meant to
> run multiple CTFd instances. CTFd is single-tenant (one scoreboard, one theme,
> one config), so run one release per CTF.
>
> **Use it as an identity provider for Dploy.** With the bundled OIDC IdP plugin
> ([`CTFd-OIDC-Provider`](https://github.com/aydev-fr)), CTFd becomes an
> OAuth2/OIDC provider — a clean way to **link CTFd and [Dploy](https://github.com/aydev-fr)**:
> register Dploy as an OIDC application (see `oidc.apps` below) and your players
> get "Log in with CTFd" SSO into Dploy. One CTFd, one Dploy, linked by OIDC.

## Install

```shell
helm install ctfd ./dploy-chart/ctfd -n ctfd --create-namespace
# with the OIDC plugin + provider wiring:
helm install ctfd ./dploy-chart/ctfd -n ctfd --create-namespace \
  -f examples/oidc-values.yaml
```

## Plugins as OCI Image Volumes

Each entry in `plugins` is mounted **read-only** at
`/opt/CTFd/CTFd/plugins/<name>` straight from an OCI image:

```yaml
plugins:
  - name: oidc
    image: ghcr.io/aydev-fr/ctfd-oidc-plugin:1.0.0
    pullPolicy: IfNotPresent
    # subPath: ctfd_oidc   # if the package isn't at the image root
```

The image's filesystem must contain the plugin package (an `__init__.py`
exposing `load(app)`) at its root, or point `subPath` at the sub-directory that
does. Requires the `ImageVolume` feature (enabled by default since Kubernetes
1.33).

> Why this is nice: plugins are versioned, pull-once OCI artifacts. You ship a
> plugin by pushing an image, not by maintaining a forked CTFd image.

If a plugin has third-party Python deps that aren't in CTFd's base image, mount
them (e.g. a `deps/` folder vendored in the same image) and add them to
`PYTHONPATH` via `extraVolumes` / `extraVolumeMounts` + `env`. The OIDC plugin
below shows the pattern — and packages it behind a single switch.

## OIDC IdP plugin (turnkey)

The [`ghcr.io/aydev-fr/ctfd-oidc-provider`](https://github.com/aydev-fr) plugin
turns CTFd into an OAuth2/OIDC identity provider. Enable it in one block — the
chart mounts the plugin, wires its vendored dependencies (`PYTHONPATH`) and
provisions the OAuth applications you list (generated into a ConfigMap):

```yaml
oidc:
  enabled: true
  image: ghcr.io/aydev-fr/ctfd-oidc-provider:1.0.1
  apps:
    - name: Example App
      client_id: example-app
      type: confidential          # or "public" (PKCE, no secret)
      client_secret: change-me
      redirect_uris: [https://app.example.com/oauth/callback]
      scopes: [openid, profile, email]
```

No `plugins`/`extraVolumes`/`env` wiring needed. Apps are provisioned at startup
(`OIDC_PROVIDER_APPS_FILE`). See [`examples/oidc-values.yaml`](examples/oidc-values.yaml).

## Provider integration (optional, full-auto)

With `bootstrap.enabled` the chart runs the CTFd setup wizard in a Job and writes
the provider credentials `Secret` itself; with `providerConfig.enabled` it also
creates the Crossplane `(Cluster)ProviderConfig`. Result: `helm install` →
provider-ctfd can manage the instance with **no manual step**. Both need the
`ctfd-bootstrap` image built from this repo (`cluster/images/ctfd-bootstrap`).

## Key values

| Key | Default | Description |
|-----|---------|-------------|
| `image.repository` / `image.tag` | `ctfd/ctfd` / appVersion | CTFd image (use ≥ 3.8). |
| `plugins[]` | `[]` | Plugins mounted from OCI images (name, image, pullPolicy, subPath). |
| `oidc.enabled` / `oidc.apps[]` | `false` / `[]` | Turnkey OIDC IdP plugin + provisioned OAuth apps. |
| `extraVolumes[]` / `extraVolumeMounts[]` | `[]` | Standard extra volumes/mounts (config files, deps, …). |
| `env[]` | `[]` | Extra env (DB/Redis/plugin config). |
| `persistence.enabled` | `false` | PVC for CTFd's data dir. |
| `bootstrap.enabled` | `false` | Auto-run setup + write the provider creds Secret. |
| `providerConfig.enabled` | `false` | Create the Crossplane `(Cluster)ProviderConfig`. |

See [`values.yaml`](values.yaml) for the full list.
