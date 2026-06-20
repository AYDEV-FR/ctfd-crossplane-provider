# provider-ctfd

`provider-ctfd` is a [Crossplane](https://crossplane.io/) provider that manages
[CTFd](https://ctfd.io/) resources declaratively. It is built on top of the
[crossplane/provider-template](https://github.com/crossplane/provider-template)
scaffold (Crossplane v2, **namespaced** managed resources) and uses
[`github.com/ctfer-io/go-ctfd`](https://github.com/ctfer-io/go-ctfd) as its CTFd
API client.

Maintained by the [barbhack](https://github.com/barbhack) org.

## Compatibility

Works with the **latest CTFd** (verified end-to-end against `ctfd/ctfd:latest`,
currently 3.8.6). The provider talks to CTFd through `go-ctfd` v0.18.0, whose
challenge API uses a `logic` field introduced in CTFd 3.8 — so **CTFd ≥ 3.8** is
required (3.7.x and earlier reject it with a 500). The e2e suite runs against
`ctfd/ctfd:latest`.

## Deploying CTFd (Helm chart)

This repo ships a Helm chart, [`charts/ctfd`](charts/ctfd), to run the CTFd
instance the provider manages. Its headline feature is installing **CTFd plugins
from OCI images** via Kubernetes Image Volumes — no custom CTFd image rebuild.
The OIDC IdP plugin is turnkey (plugin + vendored deps + provisioned apps from
one block):

```yaml
oidc:
  enabled: true
  apps:
    - name: Example App
      client_id: example-app
      type: public
      redirect_uris: [https://app.example.com/oauth/callback]
```

…or mount any plugin generically with `plugins[]` / `extraVolumes`.

With `bootstrap.enabled` / `providerConfig.enabled` it also runs the CTFd setup
wizard and wires the provider credentials, so `helm install` yields an instance
provider-ctfd can manage with no manual step. See
[`charts/ctfd/README.md`](charts/ctfd/README.md) and
[`examples/helm/ctfd-oidc.yaml`](examples/helm/ctfd-oidc.yaml). Release it with
`make helm.package` / `make helm.push` (or the `chart-release` workflow on a
`chart-v*` tag).

## Managed resources

All managed resources are **namespaced** (Crossplane v2,
`scope=Namespaced`) and live in the `resources.ctfd.crossplane.io` API group:

| Kind        | Group                            | Description                                              |
|-------------|----------------------------------|---------------------------------------------------------|
| `Challenge` | `resources.ctfd.crossplane.io`   | A CTFd challenge (standard/dynamic), with inline flags and hints. |
| `Page`      | `resources.ctfd.crossplane.io`   | A content page (e.g. rules, sponsors).                  |
| `Theme`     | `resources.ctfd.crossplane.io`   | Instance-wide theme settings (singleton).               |

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

### Pages and theme

```shell
kubectl apply -f examples/resources/page.yaml    # content pages (rules, etc.)
kubectl apply -f examples/resources/theme.yaml   # instance-wide theme settings
```

- **`Page`** is a regular CRUD resource: `title`, `route`, `content`,
  `format` (`markdown`/`html`), `draft`, `hidden`, `authRequired`. Its
  external-name is the CTFd page ID.
- **`Theme`** is an instance-wide **singleton** — use a single `Theme` per CTFd
  instance. It writes only the theme-related configuration keys (`ctf_theme`,
  `theme_header`, `theme_footer`, `theme_settings`) and leaves all other CTFd
  settings untouched. Because a CTFd instance always has an active theme,
  deleting the `Theme` resource stops Crossplane from managing it but does not
  revert CTFd to a previous theme.

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
  resources/v1alpha1/           # Challenge (with inline flags/hints), Page, Theme types
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
