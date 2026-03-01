import Foundation
import UIKit

/// 📊 Metrics Collection Service
///
/// Automatically tracks app performance and sends to your Grafana instance.
///
/// **What's tracked:**
/// - App launches, API calls, image loads
/// - User interactions (tabs, grid mode, fullscreen)
/// - Errors, memory warnings, network issues
///
/// **How it works:**
/// 1. Events are batched (every 30s or 100 events)
/// 2. Sent to POST https://lcc.live/api/metrics
/// 3. Gracefully fails if backend not ready
///
/// **Usage:**
/// ```swift
/// MetricsService.shared.track(event: .tabSwitch, tags: ["tab": "bcc"])
/// ```
///
/// **Privacy:** Anonymous only, no PII collected.
class MetricsService: ObservableObject {
    
    // MARK: - Metric Types
    
    struct Metric: Codable {
        let app: String
        let version: String
        let build: String
        let event: String
        let value: Double
        let durationMs: Double?
        let tags: [String: String]
        let timestamp: String
        
        enum CodingKeys: String, CodingKey {
            case app, version, build, event, value
            case durationMs = "duration_ms"
            case tags, timestamp
        }
    }
    
    enum Event: String {
        // App lifecycle
        case appLaunch = "app_launch"
        case appBackground = "app_background"
        case appForeground = "app_foreground"
        case appTerminate = "app_terminate"
        
        // API events
        case apiRequest = "api_request"
        case apiSuccess = "api_success"
        case apiFailure = "api_failure"
        
        // Image events
        case imageLoadStart = "image_load_start"
        case imageLoadSuccess = "image_load_success"
        case imageLoadFailure = "image_load_failure"
        
        // UI events
        case tabSwitch = "tab_switch"
        case gridModeChange = "grid_mode_change"
        case fullscreenOpen = "fullscreen_open"
        case refreshPull = "refresh_pull"
        
        // Performance
        case memoryWarning = "memory_warning"
        case networkTimeout = "network_timeout"
    }
    
    // MARK: - Properties
    
    private var metricsBuffer: [Metric] = []
    private let bufferLock = NSLock()
    private var timer: Timer?
    private let batchInterval: TimeInterval = 30.0
    private let maxBatchSize = 100
    
    private let logger = Logger(category: .metrics)
    
    // MARK: - Initialization
    
    init() {
        guard AppEnvironment.metricsEnabled else {
            logger.info("Metrics collection disabled")
            return
        }
        
        startBatchTimer()
        setupLifecycleObservers()
        logger.info("Metrics service initialized")
    }
    
    deinit {
        timer?.invalidate()
        // Send remaining metrics before shutdown
        sendBatch(force: true)
    }
    
    // MARK: - Public API
    
    /// Track a simple event
    func track(event: Event, value: Double = 1.0, tags: [String: String] = [:]) {
        guard AppEnvironment.metricsEnabled else { return }
        
        let metric = createMetric(
            event: event.rawValue,
            value: value,
            duration: nil,
            tags: tags
        )
        
        addToBuffer(metric)
    }
    
    /// Track an event with duration
    func track(event: Event, duration: TimeInterval, tags: [String: String] = [:]) {
        guard AppEnvironment.metricsEnabled else { return }
        
        let metric = createMetric(
            event: event.rawValue,
            value: 1.0,
            duration: duration * 1000, // Convert to milliseconds
            tags: tags
        )
        
        addToBuffer(metric)
    }
    
    /// Track timing for a code block
    func measure<T>(event: Event, tags: [String: String] = [:], block: () throws -> T) rethrows -> T {
        let start = Date()
        let result = try block()
        let duration = Date().timeIntervalSince(start)
        track(event: event, duration: duration, tags: tags)
        return result
    }
    
    /// Track async timing
    func measure<T>(event: Event, tags: [String: String] = [:], block: () async throws -> T) async rethrows -> T {
        let start = Date()
        let result = try await block()
        let duration = Date().timeIntervalSince(start)
        track(event: event, duration: duration, tags: tags)
        return result
    }
    
    /// Manually trigger sending metrics
    func flush() {
        sendBatch(force: true)
    }
    
    // MARK: - Private Methods
    
    private func createMetric(event: String, value: Double, duration: Double?, tags: [String: String]) -> Metric {
        let timestamp = ISO8601DateFormatter().string(from: Date())
        
        return Metric(
            app: "lcc-ios",
            version: AppEnvironment.appVersion,
            build: AppEnvironment.buildNumber,
            event: event,
            value: value,
            durationMs: duration,
            tags: tags,
            timestamp: timestamp
        )
    }
    
    private func addToBuffer(_ metric: Metric) {
        bufferLock.lock()
        defer { bufferLock.unlock() }
        
        metricsBuffer.append(metric)
        
        // Send immediately if buffer is full
        if metricsBuffer.count >= maxBatchSize {
            sendBatch(force: false)
        }
    }
    
    private func startBatchTimer() {
        timer = Timer.scheduledTimer(withTimeInterval: batchInterval, repeats: true) { [weak self] _ in
            self?.sendBatch(force: false)
        }
    }
    
    private func sendBatch(force: Bool) {
        bufferLock.lock()
        guard !metricsBuffer.isEmpty else {
            bufferLock.unlock()
            return
        }
        
        let batch = metricsBuffer
        metricsBuffer.removeAll()
        bufferLock.unlock()
        
        // Send in background
        Task {
            await sendMetrics(batch)
        }
    }
    
    private func sendMetrics(_ metrics: [Metric]) async {
        guard let url = URL(string: AppEnvironment.metricsURL) else {
            logger.error("Invalid metrics URL: \(AppEnvironment.metricsURL)")
            return
        }
        
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.timeoutInterval = 10
        
        do {
            let encoder = JSONEncoder()
            encoder.dateEncodingStrategy = .iso8601
            request.httpBody = try encoder.encode(metrics)
            
            let (_, response) = try await URLSession.shared.data(for: request)
            
            if let httpResponse = response as? HTTPURLResponse {
                if httpResponse.statusCode == 200 {
                    logger.debug("Sent \(metrics.count) metrics successfully")
                } else {
                    logger.warning("Metrics endpoint returned status \(httpResponse.statusCode)")
                }
            }
        } catch {
            // Log error but don't crash - metrics are best-effort
            logger.error("Failed to send metrics", error: error)
            
            // TODO: When backend is ready, uncomment retry logic
            // Consider re-adding failed metrics to buffer for retry
        }
    }
    
    private func setupLifecycleObservers() {
        NotificationCenter.default.addObserver(
            forName: UIApplication.didEnterBackgroundNotification,
            object: nil,
            queue: .main
        ) { [weak self] _ in
            self?.track(event: .appBackground)
            self?.sendBatch(force: true)
        }
        
        NotificationCenter.default.addObserver(
            forName: UIApplication.willEnterForegroundNotification,
            object: nil,
            queue: .main
        ) { [weak self] _ in
            self?.track(event: .appForeground)
        }
        
        NotificationCenter.default.addObserver(
            forName: UIApplication.didReceiveMemoryWarningNotification,
            object: nil,
            queue: .main
        ) { [weak self] _ in
            self?.track(event: .memoryWarning)
        }
    }
}

// MARK: - Global Instance

extension MetricsService {
    /// Shared metrics service instance
    static let shared = MetricsService()
}

