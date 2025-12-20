# sidecar-generator-operator

## Init

```sh
operator-sdk init --domain example.com --repo github.com/trevorbox/sidecar-generator-operator

operator-sdk create api --group networking --version v1alpha1 --kind SidecarGenerator --resource --controller
```

## Build Run Locally

```sh
make install run
```

## Push

```sh
make docker-build docker-push IMG="quay.io/trevorbox/sidecar-generator-operator:v0.0.1"
```
