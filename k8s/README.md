# Kubernetes setup

This is a minimal setup for testing locally with [kind](https://kind.sigs.k8s.io/)

## Create the cluster

We use a dedicated kind cluster and set up the metrics API

```shell
kind create cluster --name genteelbeacon --image kindest/node:v1.31.4
kubectl apply -k k8s/metrics-server
```

On a cloud installation, this is enabled by default.

## Gateway Fabric

Install the NGINX Gateway Fabric

```shell
kubectl kustomize "https://github.com/nginx/nginx-gateway-fabric/config/crd/gateway-api/standard?ref=v1.6.1" | kubectl apply -f -
helm upgrade --install ngf oci://ghcr.io/nginx/charts/nginx-gateway-fabric --version "1.6.1" --create-namespace -n nginx-gateway -f k8s/helm/nginx-gateway-fabric-values.yaml
```

## Install a Genteel Beacon setup

```shell
kubectl apply -f k8s/genteelbeacon.yaml
kubectl apply -f k8s/gateway.yaml
```

## External ingress

Locally, this assumes you have [Kubernetes Cloud Provider for KIND](https://github.com/kubernetes-sigs/cloud-provider-kind?tab=readme-ov-file#install).
Alternatively, in the cloud this will also work.

```shell
kubectl wait --timeout=5m -n nginx-gateway deployment/ngf-nginx-gateway-fabric --for=condition=Available
LBIP=$(kubectl get svc/ngf-nginx-gateway-fabric -n nginx-gateway -o jsonpath="{.status.loadBalancer.ingress[0].ip}")
curl http://genteelbeacon.local/telegram --resolve genteelbeacon.local:80:${LBIP}
```

Alternatively, add DNS to you `/etc/hosts` with the output from `echo "${LBIP} genteelbeacon.local"`.

## Install open telemetry collectors

This assumes a deployment for forwarding data to a remote endpoint secured via basic_auth (such as Grafana Cloud).

Add your endpoint and credentials in a `-secret` file to replace the `PLACEHOLDER`s in the values file.

```shell
helm upgrade --install otelcol oci://ghcr.io/open-telemetry/opentelemetry-helm-charts/opentelemetry-collector --version "0.116.0" --create-namespace -n otel -f k8s/helm/otelcol-deployment-values.yaml -f k8s/helm/otelcol-deployment-values-secret.yaml
```

```shell
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.17.0/cert-manager.yaml
kubectl apply --server-side -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.80.0/example/prometheus-operator-crd/monitoring.coreos.com_podmonitors.yaml
kubectl apply --server-side -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.80.0/example/prometheus-operator-crd/monitoring.coreos.com_probes.yaml
kubectl apply --server-side -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.80.0/example/prometheus-operator-crd/monitoring.coreos.com_scrapeconfigs.yaml
kubectl apply --server-side -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.80.0/example/prometheus-operator-crd/monitoring.coreos.com_servicemonitors.yaml

kubectl apply -f https://github.com/open-telemetry/opentelemetry-operator/releases/download/v0.118.0/opentelemetry-operator.yaml


kubectl apply -f k8s/opentelemetrycollector.yaml
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
                imagePullPolicy: Never
```

or by changing the ConfigMaps. The file is gitignored for a reason!

To restart everything, use

```shell
kubectl rollout restart deployment -n genteelbeacon --selector=schildwaechter=genteelbeacon
```

## Load generation

```shell
kubectl apply -f k8s/loadgenerator.yaml
INGRESS_CLUSTER_IP=$(kubectl get svc/ngf-nginx-gateway-fabric -n nginx-gateway -o jsonpath="{.spec.clusterIP}")
kubectl patch deployment loadgenerator -n loadgenerator -p "{\"spec\":{\"template\":{\"spec\":{\"hostAliases\":[{\"ip\": \"${INGRESS_CLUSTER_IP}\", \"hostnames\":[\"genteelbeacon.local\"]}]}}}}"
kubectl scale deployment loadgenerator -n loadgenerator --replicas 10
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
docker container list
docker image list
docker container prune
docker image prune -a
```

## Gearsmith

```shell
openssl req -x509 -newkey rsa:2048 -keyout tls.key -out tls.crt -sha256 -days 365 -nodes -subj "/C=NO/O=Genteel Beacon/CN=grumpygearsmith"
kubectl create secret tls -n genteelbeacon grumpygearsmith --cert=tls.crt --key=tls.key
kubectl apply -f k8s/grumpygearsmith.yaml
```
