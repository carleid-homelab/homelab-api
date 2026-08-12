# homelab-api

A small read-only metrics proxy for the [homelab](https://github.com/carleid-homelab/homelab)
cluster. It sits between the public dashboard and Prometheus, exposing a fixed set of queries as
plain JSON endpoints.

It exists for one reason: the dashboard needs live cluster numbers, and neither Prometheus nor
Grafana should be reachable from a browser with an ad-hoc query string. This service accepts no
query input at all. Every endpoint runs a PromQL expression hardcoded in `main.go`, so the public
surface is a list of readings rather than a query engine.

The service holds a Grafana service account token and reaches Prometheus through Grafana's
datasource proxy, which means the token can be scoped to Viewer and revoked independently of
anything else.

## Endpoints

All under `/api/metrics/`, all returning the `data` object of a Prometheus instant query:

| Endpoint | Query |
|---|---|
| `cluster-uptime` | `time() - kube_node_created{node="vps.carleid.dev"}` |
| `cluster-health` | `up{job="kubelet"}` |
| `argocd-health` | `up{job="argocd-metrics"}` |
| `argocd-apps` | `argocd_app_info` |
| `grafana-health` | `up{job="monitoring-grafana"}` |
| `prometheus-health` | `up{job="monitoring-kube-prometheus-prometheus"}` |
| `memory` | `100 * (1 - node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)` |
| `cpu` | `100 - (avg(rate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)` |

`/healthz` returns `ok` and backs both probes.

## Adding an endpoint

One handler, one query, in `main.go`:

```go
http.HandleFunc("/api/metrics/pod-count", func(w http.ResponseWriter, r *http.Request) {
	data, err := queryGrafana(`count(kube_pod_info)`)
	jsonResponse(w, data, err)
})
```

Keep the query in the handler rather than reading it from the request. That is the property that
makes this safe to expose. Then wire it into the dashboard through `src/flow/liveData.ts` in
`homelab-web`.

## Deployment

Push to `main`. CI builds the image, pushes it to `ghcr.io/carleid-homelab/homelab-api` tagged with
both the commit SHA and `latest`, writes the SHA tag into `deploy/deployment.yaml`, and force-
pushes the result to the `deploy` branch. ArgoCD tracks `deploy` and syncs within a few minutes.
