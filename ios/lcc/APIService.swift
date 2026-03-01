import Foundation
import Observation

@Observable
class APIService {
    var lccMedia: [MediaItem] = []
    var bccMedia: [MediaItem] = []
    var isLoading = false
    var error: Error?
    var isUsingFallback = false

    @ObservationIgnored private var serverVersion: String?
    @ObservationIgnored private var pollingTask: Task<Void, Never>?
    @ObservationIgnored private let logger = Logger(category: .networking)
    @ObservationIgnored private let parser = MediaItemParser()

    // Track errors per endpoint to avoid clearing one endpoint's error when another succeeds
    @ObservationIgnored private var lccError: Error?
    @ObservationIgnored private var bccError: Error?

    @ObservationIgnored private var baseURL: String {
        AppEnvironment.apiBaseURL
    }

    @ObservationIgnored private var checkInterval: TimeInterval {
        AppEnvironment.apiCheckInterval
    }

    init() {
        startVersionMonitoring()
    }

    deinit {
        pollingTask?.cancel()
    }

    /// Start monitoring for server version changes
    private func startVersionMonitoring() {
        // Fetch immediately on init
        Task {
            await fetchAllImages()
        }

        // Poll for version changes using structured concurrency
        pollingTask = Task { [weak self] in
            while !Task.isCancelled {
                try? await Task.sleep(for: .seconds(self?.checkInterval ?? 30))
                guard !Task.isCancelled else { break }
                await self?.checkServerVersionAndRefresh()
            }
        }
    }

    /// Check if server version has changed and refresh if needed
    private func checkServerVersionAndRefresh() async {
        do {
            let currentVersion = try await fetchServerVersion()

            if let previousVersion = serverVersion {
                if currentVersion != previousVersion {
                    logger.info("Server version changed from \(previousVersion) to \(currentVersion). Refreshing data...")
                    await fetchAllImages()
                }
            }
        } catch {
            logger.error("Error checking server version", error: error)
        }
    }

    /// Fetch the server version from headers
    private func fetchServerVersion() async throws -> String {
        guard let url = URL(string: baseURL) else {
            throw APIError.invalidResponse
        }
        var request = URLRequest(url: url)
        request.httpMethod = "HEAD"
        request.setValue("application/json", forHTTPHeaderField: "Accept")
        request.timeoutInterval = AppEnvironment.networkTimeout

        let (_, response) = try await URLSession.shared.data(for: request)

        guard let httpResponse = response as? HTTPURLResponse else {
            throw APIError.invalidResponse
        }

        // Check common version headers
        if let version = httpResponse.value(forHTTPHeaderField: "X-Server-Version") ??
                         httpResponse.value(forHTTPHeaderField: "X-Version") ??
                         httpResponse.value(forHTTPHeaderField: "ETag") {
            return version
        }

        // Fallback to Last-Modified
        if let lastModified = httpResponse.value(forHTTPHeaderField: "Last-Modified") {
            return lastModified
        }

        // If no version header, use a timestamp-based approach
        return String(Date().timeIntervalSince1970)
    }

    /// Fetch all media from both endpoints
    func fetchAllImages() async {
        await MainActor.run {
            self.isLoading = true
            self.error = nil // Clear previous errors
        }

        await withTaskGroup(of: Void.self) { group in
            group.addTask {
                await self.fetchLCCMedia()
            }
            group.addTask {
                await self.fetchBCCMedia()
            }
        }

        await MainActor.run {
            self.isLoading = false
        }
    }

    /// Fetch LCC media from the default endpoint
    func fetchLCCMedia() async {
        await fetchMedia(from: "\(baseURL)/lcc.json", updateType: .lcc)
    }

    /// Fetch BCC media from the /bcc endpoint
    func fetchBCCMedia() async {
        await fetchMedia(from: "\(baseURL)/bcc.json", updateType: .bcc)
    }

    private enum UpdateType {
        case lcc
        case bcc
    }

    /// Generic method to fetch media from an endpoint
    private func fetchMedia(from urlString: String, updateType: UpdateType) async {
        let endpointName = updateType == .lcc ? "LCC" : "BCC"
        logger.info("🔄 Fetching \(endpointName) media from \(urlString)")

        guard let url = URL(string: urlString) else {
            logger.error("❌ Invalid URL: \(urlString)")
            return
        }

        var request = URLRequest(url: url)
        request.setValue("application/json", forHTTPHeaderField: "Accept")
        request.timeoutInterval = AppEnvironment.networkTimeout

        // Track API request
        let startTime = Date()

        do {
            let (data, response) = try await URLSession.shared.data(for: request)

            // Track successful API call
            let duration = Date().timeIntervalSince(startTime)
            MetricsService.shared.track(
                event: .apiSuccess,
                duration: duration,
                tags: ["endpoint": updateType == .lcc ? "lcc" : "bcc"]
            )

            guard let httpResponse = response as? HTTPURLResponse else {
                throw APIError.invalidResponse
            }

            guard httpResponse.statusCode == 200 else {
                logger.error("❌ \(endpointName) HTTP error: \(httpResponse.statusCode)")
                throw APIError.httpError(statusCode: httpResponse.statusCode)
            }

            // Log response size for debugging
            logger.debug("📦 \(endpointName) response: \(data.count) bytes, Content-Type: \(httpResponse.value(forHTTPHeaderField: "Content-Type") ?? "unknown")")

            // Update server version from response headers
            if updateType == .lcc {
                if let version = httpResponse.value(forHTTPHeaderField: "X-Server-Version") ??
                                 httpResponse.value(forHTTPHeaderField: "X-Version") ??
                                 httpResponse.value(forHTTPHeaderField: "ETag") ??
                                 httpResponse.value(forHTTPHeaderField: "Last-Modified") {
                    await MainActor.run {
                        self.serverVersion = version
                    }
                }
            }

            // Parse JSON - expecting array of strings or objects
            let mediaItems = try parser.parseMediaItems(from: data)

            if mediaItems.isEmpty {
                logger.warning("⚠️ \(endpointName) returned empty media items array")
            }

            await MainActor.run {
                // Mark that we successfully fetched from API
                self.isUsingFallback = false

                // Clear error for this specific endpoint on success
                switch updateType {
                case .lcc:
                    self.lccMedia = mediaItems
                    self.lccError = nil
                    self.logger.info("✅ Fetched \(mediaItems.count) LCC media items from API")
                case .bcc:
                    self.bccMedia = mediaItems
                    self.bccError = nil
                    self.logger.info("✅ Fetched \(mediaItems.count) BCC media items from API")
                }

                // Update the published error to reflect the most recent error (if any)
                self.error = self.bccError ?? self.lccError
            }
        } catch {
            // Track API failure
            let duration = Date().timeIntervalSince(startTime)
            MetricsService.shared.track(
                event: .apiFailure,
                duration: duration,
                tags: [
                    "endpoint": updateType == .lcc ? "lcc" : "bcc",
                    "error": error.localizedDescription
                ]
            )

            let endpointName = updateType == .lcc ? "LCC" : "BCC"
            logger.error("⚠️ Error fetching \(endpointName) media from \(urlString)", error: error)

            if self.isUsingFallback {
                logger.info("ℹ️ Using fallback data")
            }

            await MainActor.run {
                // Track error per endpoint
                switch updateType {
                case .lcc:
                    self.lccError = error
                case .bcc:
                    self.bccError = error
                }

                // Update published error to show the most recent error
                self.error = error

                // Log which endpoint failed for debugging
                logger.warning("❌ \(endpointName) fetch failed: \(error.localizedDescription)")
            }
        }
    }
    /// Manually trigger a refresh
    func refresh() async {
        await fetchAllImages()
    }
}

enum APIError: LocalizedError {
    case invalidResponse
    case httpError(statusCode: Int)

    var errorDescription: String? {
        switch self {
        case .invalidResponse:
            return "Invalid server response"
        case .httpError(let statusCode):
            return "HTTP error: \(statusCode)"
        }
    }
}
