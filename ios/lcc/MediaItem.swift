import Foundation

/// Represents a media item that can be either an image or a video
struct MediaItem: Identifiable, Hashable {
    let id = UUID()
    let type: MediaType
    let url: String
    let identifier: String?
    let alt: String?
    let weatherStationId: Int?

    /// The slug used for the camera detail endpoint (slugified from alt text, matching server's slugify())
    var slug: String? {
        guard let alt = alt, !alt.isEmpty else { return nil }
        var result = alt.lowercased()
        result = result.replacingOccurrences(of: " ", with: "-")
        result = result.replacingOccurrences(of: "_", with: "-")
        // Remove non-alphanumeric characters except hyphens
        result = result.filter { $0.isASCII && ($0.isLetter || $0.isNumber || $0 == "-") }
        // Collapse consecutive hyphens
        while result.contains("--") {
            result = result.replacingOccurrences(of: "--", with: "-")
        }
        result = result.trimmingCharacters(in: CharacterSet(charactersIn: "-"))
        return result.isEmpty ? nil : result
    }

    /// Returns a display caption for the media item
    var caption: String? {
        return alt
    }
    
    enum MediaType: Hashable {
        case image
        case youtubeVideo(embedURL: String)
        
        var isVideo: Bool {
            if case .youtubeVideo = self {
                return true
            }
            return false
        }
    }
    
    /// Parses a URL string to determine if it's an image or YouTube video
    static func from(urlString: String, identifier: String? = nil, alt: String? = nil, weatherStationId: Int? = nil) -> MediaItem? {
        // Check if it's a YouTube URL
        if YouTubeURLHelper.isYouTubeURL(urlString) {
            if let videoURL = YouTubeURLHelper.extractEmbedURL(from: urlString) {
                Logger.ui.debug("✅ Detected YouTube video: \(urlString) -> embed URL: \(videoURL)")
                return MediaItem(type: .youtubeVideo(embedURL: videoURL), url: urlString, identifier: identifier, alt: alt, weatherStationId: weatherStationId)
            } else {
                Logger.ui.warning("⚠️ YouTube URL detected but failed to extract embed URL: \(urlString)")
            }
        }

        // Default to image
        return MediaItem(type: .image, url: urlString, identifier: identifier, alt: alt, weatherStationId: weatherStationId)
    }
    
    // For Hashable conformance
    func hash(into hasher: inout Hasher) {
        hasher.combine(id)
    }
    
    static func == (lhs: MediaItem, rhs: MediaItem) -> Bool {
        lhs.id == rhs.id
    }
}
