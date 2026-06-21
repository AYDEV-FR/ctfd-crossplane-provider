# provider-ctfd

`provider-ctfd` is a [Crossplane](https://crossplane.io/) provider that manages
[CTFd](https://ctfd.io/) resources declaratively. It is built on top of the
[crossplane/provider-template](https://github.com/crossplane/provider-template)
scaffold (Crossplane v2, **namespaced** managed resources) and uses
[`github.com/ctfer-io/go-ctfd`](https://github.com/ctfer-io/go-ctfd) as its CTFd
API client.

Maintained by the [barbhack](https://github.com/barbhack) org.

## Install

The provider is published as a Crossplane **package** (xpkg, an OCI image) to
GHCR. Install it into a cluster that runs Crossplane:

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-ctfd
spec:
  package: ghcr.io/aydev-fr/provider-ctfd:v0.1.0
```

```shell
kubectl apply -f - <<'EOF'
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata: {name: provider-ctfd}
spec: {package: ghcr.io/aydev-fr/provider-ctfd:v0.1.0}
EOF
kubectl get provider.pkg provider-ctfd -w   # wait for INSTALLED & HEALTHY
```

> The GHCR package must be **public** (Packages → provider-ctfd → Package
> settings → Change visibility) for Crossplane to pull it without credentials;
> otherwise configure a `packagePullSecrets`.

### Publishing

Pushing a `v*` tag runs the [`Publish provider`](.github/workflows/publish-provider.yml)
workflow, which builds the multi-arch xpkg and pushes it to
`ghcr.io/<owner>/provider-ctfd:<tag>`:

```shell
git tag v0.1.0 && git push origin v0.1.0
```

Locally (needs the [`crossplane` CLI](https://docs.crossplane.io/latest/cli/)
and `docker login ghcr.io`):

```shell
make xpkg.build                                  # _output/provider-ctfd.xpkg
make xpkg.push VERSION=v0.1.0                     # push to GHCR
```

The same xpkg can later be submitted to the
[Upbound Marketplace](https://marketplace.upbound.io/).

## Compatibility

Works with the **latest CTFd** (verified end-to-end against `ctfd/ctfd:latest`,
currently 3.8.6). The provider talks to CTFd through `go-ctfd` v0.18.0, whose
challenge API uses a `logic` field introduced in CTFd 3.8 — so **CTFd ≥ 3.8** is
required (3.7.x and earlier reject it with a 500). The e2e suite runs against
`ctfd/ctfd:latest`.

## Deploying CTFd (Helm chart)

A Helm chart to run the CTFd instance this provider manages lives in a separate
repo: **[AYDEV-FR/dploy-charts](https://github.com/AYDEV-FR/dploy-charts)** (the
`ctfd` chart). It installs **CTFd plugins from OCI images** via Kubernetes Image
Volumes (no custom CTFd image) and is turnkey for the OIDC IdP plugin — a good
way to link CTFd with **Dploy** ("Log in with CTFd" SSO). It can also bootstrap
CTFd and wire this provider's credentials, so `helm install` yields an instance
provider-ctfd manages with no manual step. It deploys a **single** CTFd instance
(CTFd is single-tenant — not for multi-instance).

## Managed resources

All managed resources are **namespaced** (Crossplane v2,
`scope=Namespaced`) and live in the `resources.ctfd.crossplane.io` API group:

| Kind        | Group                            | Description                                              |
|-------------|----------------------------------|---------------------------------------------------------|
| `Challenge` | `resources.ctfd.crossplane.io`   | A CTFd challenge (standard/dynamic), with inline flags and hints. |
| `Page`      | `resources.ctfd.crossplane.io`   | A content page (e.g. rules, sponsors).                  |
| `Settings`  | `resources.ctfd.crossplane.io`   | Instance-wide CTFd config (admin Config page) — singleton. |

Flags and hints are **not** separate resources: they are declared inline on a
`Challenge` (`spec.forProvider.flags` and `spec.forProvider.hints`) and the
challenge controller reconciles them as a set — adding, editing or removing an
entry creates, replaces or deletes the corresponding CTFd flag/hint. Deleting
the `Challenge` deletes its flags and hints with it.

Provider configuration is provided by two kinds in the `ctfd.crossplane.io`
group:

- `ProviderConfig` — **namespaced**, referenced by resources in the same
  namespace.
- `ClusterProviderConfig` — **cluster-scoped**, shareable across namespaces.

The external identity of every managed resource is the CTFd numeric ID, stored
in the `crossplane.io/external-name` annotation.

## Authentication

A `ProviderConfig` points at a `Secret` whose key holds a base64-encoded JSON
document:

```json
{
  "url": "https://ctf.example.com",
  "api_key": "ctfd_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
}
```

- `url` (required) — base URL of the CTFd instance.
- `api_key` (recommended) — a CTFd **admin** API token (Settings → Access Tokens).
- `session` + `nonce` — supported as a fallback when no `api_key` is available.

See [`examples/provider/config.yaml`](examples/provider/config.yaml).

## Usage

```shell
# 1. Apply the CRDs.
kubectl apply -f package/crds

# 2. Create the credentials Secret and a (Cluster)ProviderConfig.
kubectl apply -f examples/provider/config.yaml

# 3. Create CTFd resources (a Challenge carries its flags and hints inline).
kubectl apply -f examples/resources/challenge.yaml
kubectl get challenges -n default
```

### Flags and hints

Flags and hints are declared inline on the `Challenge`:

```yaml
spec:
  forProvider:
    name: Welcome
    category: misc
    description: ...
    flags:
      - content: CTF{w3lc0m3}        # type defaults to "static"
      - content: "^CTF\\{.*\\}$"
        type: regex
        data: case_insensitive       # static flags may also be case-insensitive
    hints:
      - title: Stuck?
        content: Look at the scoreboard.
        cost: 10
```

The challenge controller treats `flags` and `hints` as managed sets: entries are
matched by value, so editing an entry replaces the underlying CTFd flag/hint and
removing one deletes it. Deleting the `Challenge` removes its flags and hints too.

### Pages and settings

```shell
kubectl apply -f examples/resources/page.yaml      # content pages (rules, etc.)
kubectl apply -f examples/resources/settings.yaml  # instance-wide CTFd config
```

- **`Page`** is a regular CRUD resource: `title`, `route`, `content`,
  `format` (`markdown`/`html`), `draft`, `hidden`, `authRequired`. Its
  external-name is the CTFd page ID.
- **`Settings`** is an instance-wide **singleton** — one per CTFd instance. It
  maps the admin **Config** page to a single resource: name/description,
  appearance (`theme`, `themeHeader/Footer/Settings`), visibility, start/end/
  pause, user mode & team size, registration, challenges, HTML sanitization,
  legal/robots pages, and SMTP (`mail`, password via `passwordSecretRef`). Every
  field is optional — only the keys you set are written (partial `PATCH`), so
  unset keys keep CTFd's value. Deleting the resource stops Crossplane from
  managing the config; it does not reset CTFd.

## Developing

This project uses the Crossplane build submodule.

```shell
make submodules        # initialize the build/ submodule used for CI/CD
make generate          # regenerate deepcopy/managed methods and CRDs
make reviewable        # codegen + linters + tests
make build             # build the provider binary and package
```

Without the build submodule you can still work with the standard Go toolchain:

```shell
go generate ./apis/... # regenerate code under apis/ and CRDs under package/crds
go build ./...
go test ./...
```

### Layout

```
apis/
  ctfd.go                       # scheme registration
  v1alpha1/                     # (Cluster)ProviderConfig types
  resources/v1alpha1/           # Challenge (inline flags/hints), Page, Settings types
internal/
  clients/ctfd.go               # builds a go-ctfd client from ProviderConfig creds
  controller/
    config/                     # (Cluster)ProviderConfig reconciler
    challenge/                  # challenge controller (also reconciles its flags & hints)
    page/ theme/                # page CRUD and theme (config) controllers
cmd/provider/main.go            # provider entrypoint
examples/                       # ProviderConfig and resource manifests
package/crds/                   # generated CRDs
test/e2e/                       # kind + CTFd + Crossplane end-to-end test
```

### End-to-end tests

`make test.e2e` spins up a kind cluster, deploys a minimal CTFd, installs
Crossplane and the provider, applies the example resources and asserts the
result through the CTFd API. See [`test/e2e/README.md`](test/e2e/README.md).

Refer to Crossplane's [CONTRIBUTING.md] and the [Provider Development][provider-dev]
guide for more.

[CONTRIBUTING.md]: https://github.com/crossplane/crossplane/blob/master/CONTRIBUTING.md
[provider-dev]: https://github.com/crossplane/crossplane/blob/master/contributing/guide-provider-development.md
