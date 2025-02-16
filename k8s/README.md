# Kubernetes setup

This is a minimal setup for testing locally with [kind](https://kind.sigs.k8s.io/)
Suggested to have [Kubernetes Cloud Provider for KIND](https://github.com/kubernetes-sigs/cloud-provider-kind?tab=readme-ov-file#install) (at least on Linux)

## Create the cluster

We use a dedicated kind cluster and set up the metrics API

```shell
kind create cluster --name genteelbeacon --image kindest/node:v1.31.4
kubectl apply -k k8s/metrics-server
```

On a cloud installation, this is enabled by default.

## Install a Genteel Beacon setup

```shell
kubectl apply -f genteelbeacon.yaml
```

## Install open telemetry collectors

This assumes a deployment for forwarding data to a remote endpoint secured via basic_auth (such as Grafana Cloud).

```shell
helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
```

Add your endpoint and credentials in a `-secret` file to replace the `PLACEHOLDER`s in the values file.

```shell
kubectl create ns otel
helm upgrade --install -n otel otelcol open-telemetry/opentelemetry-collector -f helm/otelcol-deployment-values.yaml -f helm/otelcol-deployment-values-secret.yaml
```

## Customize with Kustomize

Create a `kustomization.yaml` in this directory to override some of the settings, e.g.

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - genteelbeacon.yaml
patches:
  - target:
      kind: Service
      name: gildedgateway
    patch: |
      apiVersion: v1
      kind: Service
      metadata:
        name: gildedgateway
      spec:
        type: LoadBalancer
  - target:
      kind: Deployment
    patch: |
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        name: not-important
      spec:
        template:
          spec:
            containers:
              - name: genteelbeacon
                image: schildwaechter/genteelbeacon:main
```

or by changing the ConfigMaps. The file is gitignored for a reason!

To restart everything, use

```shell
kubectl rollout restart deployment -n genteelbeacon --selector=schildwaechter=genteelbeacon
```

## Local builds

Use the power of KIND (not with Podman)

```shell
export LOCALTAG="local-${RANDOM}" && echo $LOCALTAG
docker build -t schildwaechter/genteelbeacon:$LOCALTAG .
kind load --name genteelbeacon docker-image schildwaechter/genteelbeacon:$LOCALTAG
sed -i.bak "s/schildwaechter\/genteelbeacon:\(.*\)/schildwaechter\/genteelbeacon:${LOCALTAG}/" k8s/kustomization.yaml
kubectl apply -k k8s/
```

## Clean up

```shell
kind delete cluster --name genteelbeacon
docker image list
```

## Gearsmith

```shell
openssl req -x509 -newkey rsa:2048 -keyout tls.key -out tls.crt -sha256 -days 365 -nodes -subj "/C=NO/O=Genteel Beacon/CN=grumpygearsmith"
kubectl create secret tls -n genteelbeacon grumpygearsmith --cert=tls.crt --key=tls.key
kubectl apply -f k8s/grumpygearsmith.yaml
```
