package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Métricas expostas ao Prometheus
var (
	requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total de requisições HTTP recebidas",
		},
		[]string{"method", "path", "status"},
	)

	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duração das requisições HTTP em segundos",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// Gauge que fica em 1 enquanto o serviço está no ar (disponibilidade)
	serviceUp = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "service_up",
		Help: "Indica se o serviço está disponível (1 = sim, 0 = não)",
	})
)

func init() {
	prometheus.MustRegister(requestsTotal, requestDuration, serviceUp)
	serviceUp.Set(1)
}

type Response struct {
	Nome    string `json:"nome"`
	Horario string `json:"horario"`
}

// instrumento envolve um handler e registra métricas de cada requisição
func instrumento(path string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		inicio := time.Now()

		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next(rw, r)

		duracao := time.Since(inicio).Seconds()
		status := http.StatusText(rw.status)

		requestsTotal.WithLabelValues(r.Method, path, status).Inc()
		requestDuration.WithLabelValues(r.Method, path).Observe(duracao)
	}
}

// responseWriter captura o status code para registrar na métrica
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func projetoKorpHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "método não permitido", http.StatusMethodNotAllowed)
		return
	}

	resp := Response{
		Nome:    "Projeto Korp",
		Horario: time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/projeto-korp", instrumento("/projeto-korp", projetoKorpHandler))
	mux.Handle("/metrics", promhttp.Handler())

	log.Println("Servidor iniciado na porta 8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("Erro ao iniciar servidor: %v", err)
	}
}
