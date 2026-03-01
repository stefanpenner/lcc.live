import Foundation
import UIKit
import Observation

@Observable
@MainActor
class ImagePreloader {
    var loadedImages: [URL: UIImage] = [:]
    var lastRefreshed: Date = .init()
    var loading: Set<URL> = []
    var fadingOut: [URL: Date] = [:]

    @ObservationIgnored private var urls: [URL] = []
    @ObservationIgnored private var etags: [URL: String] = [:]
    @ObservationIgnored private var lastModifieds: [URL: String] = [:]
    @ObservationIgnored private var hasLoadedOnce: Set<URL> = []
    @ObservationIgnored private var cacheExpirationDates: [URL: Date] = [:]
    @ObservationIgnored private let logger = Logger(category: .imageLoading)

    @ObservationIgnored private var refreshInterval: TimeInterval {
        AppEnvironment.imageRefreshInterval
    }

    // Memory management
    @ObservationIgnored private let maxCachedImages = 100
    @ObservationIgnored private var cacheAccessTimes: [URL: Date] = [:]

    // Efficient refresh queue
    @ObservationIgnored private var refreshQueue: [URL] = []
    @ObservationIgnored private let maxConcurrentLoads = 6
    @ObservationIgnored private let batchRefreshInterval: TimeInterval = 0.5

    // Task-based polling (replaces 3 Timers)
    @ObservationIgnored private var refreshCycleTask: Task<Void, Never>?
    @ObservationIgnored private var batchProcessorTask: Task<Void, Never>?
    @ObservationIgnored private var cacheCleanupTask: Task<Void, Never>?

    init() {
        startBackgroundRefresh()
        setupMemoryWarningHandler()
    }

    deinit {
        refreshCycleTask?.cancel()
        batchProcessorTask?.cancel()
        cacheCleanupTask?.cancel()
        NotificationCenter.default.removeObserver(self)
    }

    // MARK: - Memory Management

    private func setupMemoryWarningHandler() {
        NotificationCenter.default.addObserver(
            forName: UIApplication.didReceiveMemoryWarningNotification,
            object: nil,
            queue: .main
        ) { [weak self] _ in
            Task { @MainActor in
                self?.handleMemoryWarning()
            }
        }
    }

    private func handleMemoryWarning() {
        logger.warning("Memory warning received - clearing image cache")

        // Keep only the most recently accessed images
        let sortedByAccess = cacheAccessTimes.sorted { $0.value > $1.value }
        let urlsToKeep = Set(sortedByAccess.prefix(20).map { $0.key })

        loadedImages = loadedImages.filter { urlsToKeep.contains($0.key) }
        cacheAccessTimes = cacheAccessTimes.filter { urlsToKeep.contains($0.key) }
        cacheExpirationDates = cacheExpirationDates.filter { urlsToKeep.contains($0.key) }

        logger.info("Cleared cache, kept \(loadedImages.count) images")

        MetricsService.shared.track(event: .memoryWarning, tags: ["source": "image_cache"])
    }

    private func pruneCache() {
        guard loadedImages.count > maxCachedImages else { return }

        let imagesToRemove = loadedImages.count - maxCachedImages
        let sortedByAccess = cacheAccessTimes.sorted { $0.value < $1.value }
        let urlsToRemove = sortedByAccess.prefix(imagesToRemove).map { $0.key }

        for url in urlsToRemove {
            loadedImages.removeValue(forKey: url)
            cacheAccessTimes.removeValue(forKey: url)
            cacheExpirationDates.removeValue(forKey: url)
        }

        logger.debug("Pruned cache: removed \(urlsToRemove.count) images")
    }

    func recordAccess(for url: URL) {
        cacheAccessTimes[url] = Date()
    }

    // MARK: - Cache Expiration

    private func isCacheValid(for url: URL) -> Bool {
        guard let expirationDate = cacheExpirationDates[url] else {
            return true
        }
        if expirationDate == Date.distantFuture {
            return true
        }
        return Date() < expirationDate
    }

    private func cleanupExpiredCache() {
        let now = Date()
        let expiredURLs: [URL] = cacheExpirationDates.compactMap { url, expirationDate in
            if expirationDate == Date.distantFuture {
                return nil
            }
            return now >= expirationDate ? url : nil
        }

        guard !expiredURLs.isEmpty else { return }

        for url in expiredURLs {
            loadedImages.removeValue(forKey: url)
            cacheExpirationDates.removeValue(forKey: url)
            cacheAccessTimes.removeValue(forKey: url)
            etags.removeValue(forKey: url)
            lastModifieds.removeValue(forKey: url)
        }

        logger.debug("Cleaned up \(expiredURLs.count) expired cache entries")
    }

    private nonisolated func parseCacheExpiration(from response: HTTPURLResponse, requestDate: Date) -> Date? {
        if let cacheControl = response.value(forHTTPHeaderField: "Cache-Control") {
            let directives = cacheControl.lowercased().split(separator: ",").map { $0.trimmingCharacters(in: .whitespaces) }

            for directive in directives {
                if directive == "no-cache" || directive == "no-store" {
                    return Date.distantPast
                }
                if directive.hasPrefix("max-age=") {
                    let maxAgeString = String(directive.dropFirst(8))
                    if let maxAge = TimeInterval(maxAgeString) {
                        return requestDate.addingTimeInterval(maxAge)
                    }
                }
                if directive.hasPrefix("s-maxage=") {
                    let sMaxAgeString = String(directive.dropFirst(9))
                    if let sMaxAge = TimeInterval(sMaxAgeString) {
                        return requestDate.addingTimeInterval(sMaxAge)
                    }
                }
            }
        }

        if let expiresString = response.value(forHTTPHeaderField: "Expires") {
            let formatter = DateFormatter()
            formatter.locale = Locale(identifier: "en_US_POSIX")
            formatter.dateFormat = "EEE, dd MMM yyyy HH:mm:ss zzz"
            formatter.timeZone = TimeZone(secondsFromGMT: 0)

            if let expiresDate = formatter.date(from: expiresString) {
                return expiresDate
            }
        }

        return Date.distantFuture
    }

    private func removeFromCache(_ url: URL) {
        loadedImages.removeValue(forKey: url)
        cacheExpirationDates.removeValue(forKey: url)
        cacheAccessTimes.removeValue(forKey: url)
        etags.removeValue(forKey: url)
        lastModifieds.removeValue(forKey: url)
        hasLoadedOnce.remove(url)
    }

    func preloadImages(from urlStrings: [String]) {
        let urls = urlStrings.compactMap { URL(string: $0) }
        self.urls = urls
        for url in urls {
            loadImage(for: url)
        }
        lastRefreshed = Date()
    }

    /// Preload media items (only loads images, skips videos)
    func preloadMedia(from mediaItems: [MediaItem]) {
        let urls = mediaItems
            .filter { !$0.type.isVideo }
            .compactMap { URL(string: $0.url) }
        self.urls = urls
        for url in urls {
            loadImage(for: url)
        }
        lastRefreshed = Date()
    }

    func refreshImages() {
        for url in urls {
            loadImage(for: url, forceRefresh: true)
        }
        lastRefreshed = Date()
    }

    func retryImage(for url: URL) {
        loadImage(for: url, forceRefresh: true)
    }

    /// Load image immediately with high priority (for visible images)
    func loadImageImmediately(for url: URL) {
        refreshQueue.removeAll { $0 == url }
        loadImage(for: url, forceRefresh: false)
    }

    private func startBackgroundRefresh() {
        // Main refresh cycle — repopulate queue
        refreshCycleTask = Task { [weak self] in
            while !Task.isCancelled {
                try? await Task.sleep(for: .seconds(self?.refreshInterval ?? 30))
                guard !Task.isCancelled else { break }
                self?.queueBackgroundRefresh()
            }
        }

        // Batch processor — continuously process queue
        batchProcessorTask = Task { [weak self] in
            while !Task.isCancelled {
                try? await Task.sleep(for: .seconds(self?.batchRefreshInterval ?? 0.5))
                guard !Task.isCancelled else { break }
                self?.processRefreshQueue()
            }
        }

        // Cleanup expired cache entries periodically
        cacheCleanupTask = Task { [weak self] in
            while !Task.isCancelled {
                try? await Task.sleep(for: .seconds(60))
                guard !Task.isCancelled else { break }
                self?.cleanupExpiredCache()
            }
        }
    }

    private func queueBackgroundRefresh() {
        let urlsToQueue = urls.filter { url in
            !refreshQueue.contains(url) && !loading.contains(url)
        }

        let sortedUrls = urlsToQueue.sorted { url1, url2 in
            let time1 = cacheAccessTimes[url1] ?? .distantPast
            let time2 = cacheAccessTimes[url2] ?? .distantPast
            return time1 > time2
        }

        refreshQueue.append(contentsOf: sortedUrls)

        if !sortedUrls.isEmpty {
            logger.debug("Queued \(sortedUrls.count) images for refresh")
        }
    }

    private func processRefreshQueue() {
        guard loading.count < maxConcurrentLoads else { return }

        let availableSlots = maxConcurrentLoads - loading.count
        let batch = Array(refreshQueue.prefix(availableSlots))

        if !batch.isEmpty {
            refreshQueue.removeFirst(batch.count)

            for url in batch {
                loadImage(for: url)
            }

            lastRefreshed = Date()
        }
    }

    private func loadImage(for url: URL, forceRefresh: Bool = false) {
        if loading.contains(url) && !forceRefresh {
            recordAccess(for: url)
            return
        }

        if !forceRefresh, loadedImages[url] != nil, isCacheValid(for: url) {
            recordAccess(for: url)
            return
        }

        // If cache expired, remove stale entry
        if !forceRefresh, loadedImages[url] != nil, !isCacheValid(for: url) {
            loadedImages.removeValue(forKey: url)
            cacheExpirationDates.removeValue(forKey: url)
        }

        var request = URLRequest(url: url)
        request.cachePolicy = forceRefresh ? .reloadIgnoringLocalCacheData : .useProtocolCachePolicy
        request.timeoutInterval = AppEnvironment.networkTimeout

        if !forceRefresh {
            if let etag = etags[url] {
                request.setValue(etag, forHTTPHeaderField: "If-None-Match")
            }
            if let lastModified = lastModifieds[url] {
                request.setValue(lastModified, forHTTPHeaderField: "If-Modified-Since")
            }
        }

        loading.insert(url)

        let startTime = Date()
        let capturedEtag = etags[url]
        let capturedLastModified = lastModifieds[url]

        Task {
            await fetchAndProcessImage(
                url: url,
                request: request,
                startTime: startTime,
                prevEtag: capturedEtag,
                prevLastModified: capturedLastModified
            )
        }
    }

    /// Fetch image data using async/await and process the result
    private func fetchAndProcessImage(
        url: URL,
        request: URLRequest,
        startTime: Date,
        prevEtag: String?,
        prevLastModified: String?
    ) async {
        do {
            let (data, response) = try await URLSession.shared.data(for: request)
            let duration = Date().timeIntervalSince(startTime)

            guard let httpResponse = response as? HTTPURLResponse else {
                logger.warning("No HTTP response for URL: \(url.absoluteString)")
                removeFromCache(url)
                loading.remove(url)
                return
            }

            // HTTP error
            if httpResponse.statusCode >= 400 {
                logger.error("HTTP error \(httpResponse.statusCode) for URL: \(url.absoluteString)")
                MetricsService.shared.track(
                    event: .imageLoadFailure,
                    duration: duration,
                    tags: ["url": url.absoluteString, "error": "HTTP \(httpResponse.statusCode)"]
                )
                removeFromCache(url)
                loading.remove(url)
                return
            }

            // 304 Not Modified
            if httpResponse.statusCode == 304 {
                logger.debug("Image unchanged (304) for URL: \(url.absoluteString)")
                MetricsService.shared.track(
                    event: .imageLoadSuccess,
                    duration: duration,
                    tags: ["changed": "false", "cached": "true"]
                )
                loading.remove(url)
                recordAccess(for: url)
                lastRefreshed = Date()
                return
            }

            // Validate data
            guard data.count > 100 else {
                logger.warning("Image data for URL \(url.absoluteString) is too small or missing (size: \(data.count))")
                MetricsService.shared.track(
                    event: .imageLoadFailure,
                    duration: duration,
                    tags: ["url": url.absoluteString, "error": "Invalid data size: \(data.count)"]
                )
                removeFromCache(url)
                loading.remove(url)
                return
            }

            // Decode image off main actor
            let image = await decodeImage(from: data)
            guard let decodedImage = image else {
                logger.error("Failed to decode image for URL: \(url.absoluteString). Data length: \(data.count)")
                MetricsService.shared.track(
                    event: .imageLoadFailure,
                    duration: duration,
                    tags: ["url": url.absoluteString, "error": "Decode failure", "data_size": "\(data.count)"]
                )
                removeFromCache(url)
                loading.remove(url)
                return
            }

            let newEtag = httpResponse.allHeaderFields["Etag"] as? String
            let newLastModified = httpResponse.allHeaderFields["Last-Modified"] as? String
            let expirationDate = parseCacheExpiration(from: httpResponse, requestDate: startTime)

            // Determine if content changed
            let changed: Bool = {
                var changed = false
                if let newEtag = newEtag {
                    changed = newEtag != prevEtag
                }
                if let newLastModified = newLastModified {
                    changed = changed || (newLastModified != prevLastModified)
                }
                return changed
            }()

            let isFirstLoad = !hasLoadedOnce.contains(url)

            // Handle no-cache/no-store: display image but treat as hot cache
            if expirationDate == Date.distantPast {
                loadedImages[url] = decodedImage
                cacheExpirationDates.removeValue(forKey: url)
                recordAccess(for: url)
                lastRefreshed = Date()
                hasLoadedOnce.insert(url)
                loading.remove(url)
                if let newEtag = newEtag { etags[url] = newEtag }
                if let newLastModified = newLastModified { lastModifieds[url] = newLastModified }
                return
            }

            loadedImages[url] = decodedImage
            cacheExpirationDates[url] = expirationDate
            recordAccess(for: url)
            lastRefreshed = Date()
            hasLoadedOnce.insert(url)

            MetricsService.shared.track(
                event: .imageLoadSuccess,
                duration: duration,
                tags: ["changed": changed ? "true" : "false"]
            )

            loading.remove(url)
            if changed && !isFirstLoad {
                fadingOut[url] = Date()
                Task {
                    try? await Task.sleep(for: .milliseconds(500))
                    fadingOut.removeValue(forKey: url)
                }
            }

            if let newEtag = newEtag { etags[url] = newEtag }
            if let newLastModified = newLastModified { lastModifieds[url] = newLastModified }

            if loadedImages.count > maxCachedImages {
                pruneCache()
            }

        } catch {
            let duration = Date().timeIntervalSince(startTime)
            logger.error("Failed to download image for URL: \(url.absoluteString)", error: error)
            MetricsService.shared.track(
                event: .imageLoadFailure,
                duration: duration,
                tags: ["url": url.absoluteString, "error": error.localizedDescription]
            )
            removeFromCache(url)
            loading.remove(url)
        }
    }

    /// Decode image data off the main actor to avoid UI stutter
    private nonisolated func decodeImage(from data: Data) async -> UIImage? {
        guard let image = UIImage(data: data) else { return nil }
        return image.preparingForDisplay() ?? image
    }
}
