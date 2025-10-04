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

	// CacheHits tracks HTTP cache hits by path
	CacheHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lcc_http_cache_hits_total",
			Help: "Total number of HTTP cache hits (304 Not Modified responses)",
		},
		[]string{"path"},
	)

	// ResponseSizeBytes measures HTTP response sizes
	ResponseSizeBytes = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "lcc_http_response_size_bytes",
			Help:    "HTTP response size in bytes",
			Buckets: prometheus.ExponentialBuckets(100, 10, 7), // 100B to 10MB
		},
		[]string{"path"},
	)

	// ErrorsByType tracks application errors by type
	ErrorsByType = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lcc_errors_total",
			Help: "Total number of application errors by type",
		},
		[]string{"error_type"},
	)

	// === Per-Camera Origin Metrics ===

	// CameraFetchTotal tracks fetches per camera with status
	CameraFetchTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lcc_camera_fetch_total",
			Help: "Total number of fetches per camera by status",
		},
		[]string{"camera", "canyon", "status"}, // camera name, canyon (LCC/BCC), status (success/error/unchanged)
	)

	// CameraFetchDuration tracks fetch latency per camera
	CameraFetchDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "lcc_camera_fetch_duration_seconds",
			Help:    "Time to fetch image from specific camera",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"camera", "canyon"},
	)

	// CameraAvailability indicates if camera is responding (1=up, 0=down)
	CameraAvailability = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "lcc_camera_availability",
			Help: "Camera availability status (1=up, 0=down)",
		},
		[]string{"camera", "canyon"},
	)

	// CameraLastSuccessTimestamp records when camera was last successfully fetched
	CameraLastSuccessTimestamp = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "lcc_camera_last_success_timestamp_seconds",
			Help: "Unix timestamp of last successful camera fetch",
		},
		[]string{"camera", "canyon"},
	)

	// CameraImageSizeBytes tracks image size per camera
	CameraImageSizeBytes = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "lcc_camera_image_size_bytes",
			Help: "Current image size in bytes per camera",
		},
		[]string{"camera", "canyon"},
	)

	// === Per-Origin Metrics ===

	// OriginFetchTotal tracks fetches per origin domain
	OriginFetchTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lcc_origin_fetch_total",
			Help: "Total number of fetches per origin domain",
		},
		[]string{"origin", "status"}, // origin domain, status (success/error)
	)

	// OriginFetchDuration tracks fetch latency per origin
	OriginFetchDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "lcc_origin_fetch_duration_seconds",
			Help:    "Time to fetch from specific origin",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"origin"},
	)

	// OriginErrorsByType tracks errors per origin by error type
	OriginErrorsByType = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lcc_origin_errors_total",
			Help: "Total errors per origin by error type",
		},
		[]string{"origin", "error_type"}, // origin, error_type (timeout, connection, bad_status, etc.)
	)

	// === Usage & Traffic Metrics ===

	// PageViewsTotal tracks page views by canyon
	PageViewsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lcc_page_views_total",
			Help: "Total page views by canyon",
		},
		[]string{"canyon"}, // LCC, BCC
	)

	// ImageViewsTotal tracks image views per camera
	ImageViewsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lcc_image_views_total",
			Help: "Total image views per camera",
		},
		[]string{"camera", "canyon"},
	)

	// UniqueVisitors tracks unique visitors (based on IP) - approximate
	UniqueVisitors = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "lcc_unique_visitors_approximate",
			Help: "Approximate number of unique visitors in current window",
		},
		[]string{"canyon"},
	)

	// === Performance Metrics ===

	// ImageStalenessSeconds tracks how old served images are
	ImageStalenessSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "lcc_image_staleness_seconds",
			Help:    "Age of served images in seconds",
			Buckets: []float64{1, 3, 5, 10, 30, 60, 120, 300, 600}, // 1s to 10min
		},
		[]string{"canyon"},
	)

	// BandwidthBytesTotal tracks total bandwidth served
	BandwidthBytesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lcc_bandwidth_bytes_total",
			Help: "Total bandwidth served in bytes",
		},
		[]string{"canyon", "type"}, // canyon, type (page/image)
	)

	// === Application Health Metrics ===

	// FetchCycleDurationSeconds tracks entire fetch cycle duration
	FetchCycleDurationSeconds = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "lcc_fetch_cycle_duration_seconds",
			Help: "Duration of last fetch cycle in seconds",
		},
	)

	// ConcurrentFetches tracks number of concurrent image fetches
	ConcurrentFetches = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "lcc_concurrent_fetches",
			Help: "Number of concurrent image fetches in progress",
		},
	)

	// MemoryUsageBytes tracks application memory usage
	MemoryUsageBytes = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "lcc_memory_usage_bytes",
			Help: "Application memory usage in bytes",
		},
	)
)
