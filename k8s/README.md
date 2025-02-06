# Kubernetes setup

This is a minimal setup for testing locally with [kind](https://kind.sigs.k8s.io/)
Suggested to have [Kubernetes Cloud Provider for KIND](https://github.com/kubernetes-sigs/cloud-provider-kind?tab=readme-ov-file#install) (at least on Linux)

## Create the cluster

```shell
kind create cluster --name genteelbeacon --image kindest/node:v1.31.4
```

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
helm upgrade --install -n otel otelds open-telemetry/opentelemetry-collector -f helm/otelcol-deamonset-values.yaml
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

To reflect changes in the ConfigMap, use

```shell
kubectl rollout restart deployment -n genteelbeacon --selector=schildwaechter=genteelbeacon
```

## Local builds

Use th power of KIND

```shell
export LOCALTAG="local-${RANDOM}" && echo $LOCALTAG
docker build -t schildwaechter/genteelbeacon:$LOCALTAG .
kind load --name genteelbeacon docker-image schildwaechter/genteelbeacon:$LOCALTAG
sed -i "s/schildwaechter\/genteelbeacon:\(.*\)/schildwaechter\/genteelbeacon:${LOCALTAG}/" k8s/kustomization.yaml
kubectl apply -k k8s/
```

And update your image tag,

## Clean up

```shell
kind delete cluster --name genteelbeacon
docker image list
```
