package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
)

const grafanaInternalURL = "http://monitoring-grafana.monitoring.svc.cluster.local:80"

type PrometheusResponse struct {
	Status string          `json:"status"`
	Data   json.RawMessage `json:"data"`
}

type MetricResult struct {
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
}

func main() {
	token := os.Getenv("GRAFANA_TOKEN")
	if token == "" {
		log.Fatal("GRAFANA_TOKEN environment variable is required")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	client := &http.Client{Timeout: 10 * time.Second}

	queryGrafana := func(promql string) (json.RawMessage, error) {
		endpoint := fmt.Sprintf("%s/api/datasources/proxy/uid/prometheus/api/v1/query", grafanaInternalURL)
		req, err := http.NewRequest("GET", endpoint, nil)
		if err != nil {
			return nil, err
		}

		q := url.Values{}
		q.Set("query", promql)
		req.URL.RawQuery = q.Encode()
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		var result PrometheusResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, err
		}

		if result.Status != "success" {
			return nil, fmt.Errorf("prometheus query failed: %s", string(body))
		}

		return result.Data, nil
	}

	jsonResponse := func(w http.ResponseWriter, data interface{}, err error) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(data)
	}

	http.HandleFunc("/api/metrics/cluster-uptime", func(w http.ResponseWriter, r *http.Request) {
		data, err := queryGrafana(`time() - kube_node_created{node="vps.carleid.dev"}`)
		jsonResponse(w, data, err)
	})

	http.HandleFunc("/api/metrics/cluster-health", func(w http.ResponseWriter, r *http.Request) {
		data, err := queryGrafana(`up{job="kubelet"}`)
		jsonResponse(w, data, err)
	})

	http.HandleFunc("/api/metrics/argocd-health", func(w http.ResponseWriter, r *http.Request) {
		data, err := queryGrafana(`up{job="argocd-metrics"}`)
		jsonResponse(w, data, err)
	})

	http.HandleFunc("/api/metrics/argocd-apps", func(w http.ResponseWriter, r *http.Request) {
		data, err := queryGrafana(`argocd_app_info`)
		jsonResponse(w, data, err)
	})

	http.HandleFunc("/api/metrics/grafana-health", func(w http.ResponseWriter, r *http.Request) {
		data, err := queryGrafana(`up{job="monitoring-grafana"}`)
		jsonResponse(w, data, err)
	})

	http.HandleFunc("/api/metrics/prometheus-health", func(w http.ResponseWriter, r *http.Request) {
		data, err := queryGrafana(`up{job="monitoring-kube-prometheus-prometheus"}`)
		jsonResponse(w, data, err)
	})

	http.HandleFunc("/api/metrics/memory", func(w http.ResponseWriter, r *http.Request) {
		data, err := queryGrafana(`100 * (1 - node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)`)
		jsonResponse(w, data, err)
	})

	http.HandleFunc("/api/metrics/cpu", func(w http.ResponseWriter, r *http.Request) {
		data, err := queryGrafana(`100 - (avg(rate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)`)
		jsonResponse(w, data, err)
	})

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	log.Printf("Proxy listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
