package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// ImageFetchTotal counts total image fetches by status
	ImageFetchTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lcc_image_fetch_total",
			Help: "Total number of image fetches",
		},
		[]string{"status"}, // success, error, unchanged
	)

	// ImageFetchDuration measures image fetch latency
	ImageFetchDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "lcc_image_fetch_duration_seconds",
			Help:    "Time spent fetching all images",
			Buckets: prometheus.LinearBuckets(0.1, 0.1, 10), // 0.1s to 1s
		},
	)

	// ImageFetchSizeBytes measures image sizes
	ImageFetchSizeBytes = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "lcc_image_fetch_size_bytes",
			Help:    "Size of fetched images in bytes",
			Buckets: prometheus.ExponentialBuckets(1024, 2, 12), // 1KB to 4MB
		},
	)

	// ImageFetchErrorsTotal counts errors by type
	ImageFetchErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lcc_image_fetch_errors_total",
			Help: "Total number of image fetch errors by reason",
		},
		[]string{"reason"}, // head_request, get_request, bad_status, read_body
	)

	// CamerasTotal tracks number of cameras per canyon
	CamerasTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "lcc_cameras_total",
			Help: "Total number of cameras",
		},
		[]string{"canyon"}, // LCC, BCC
	)

	// StoreEntriesTotal tracks total entries in store
	StoreEntriesTotal = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "lcc_store_entries_total",
			Help: "Total number of entries in the store",
		},
	)

	// StoreFetchCyclesTotal counts fetch cycles
	StoreFetchCyclesTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "lcc_store_fetch_cycles_total",
			Help: "Total number of fetch cycles completed",
		},
	)

	// ImagesReady indicates if images are ready to serve (0 or 1)
	ImagesReady = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "lcc_images_ready",
			Help: "Whether images are ready to serve (0=false, 1=true)",
		},
	)

	// HTTPRequestDuration measures HTTP request latency by path
	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "lcc_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)

	// HTTPRequestsTotal counts HTTP requests
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lcc_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	// HTTPRequestsInFlight tracks active HTTP requests
	HTTPRequestsInFlight = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "lcc_http_requests_in_flight",
			Help: "Number of HTTP requests currently being processed",
		},
	)
)
