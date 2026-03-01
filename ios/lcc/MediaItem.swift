import Foundation

/// Represents a media item that can be either an image or a video
struct MediaItem: Identifiable, Hashable {
    let id = UUID()
    let type: MediaType
    let url: String
    let identifier: String?
    let alt: String?
    
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
    static func from(urlString: String, identifier: String? = nil, alt: String? = nil) -> MediaItem? {
        // Check if it's a YouTube URL
        if YouTubeURLHelper.isYouTubeURL(urlString) {
            if let videoURL = YouTubeURLHelper.extractEmbedURL(from: urlString) {
                Logger.ui.debug("✅ Detected YouTube video: \(urlString) -> embed URL: \(videoURL)")
                return MediaItem(type: .youtubeVideo(embedURL: videoURL), url: urlString, identifier: identifier, alt: alt)
            } else {
                Logger.ui.warning("⚠️ YouTube URL detected but failed to extract embed URL: \(urlString)")
            }
        }
        
        // Default to image
        return MediaItem(type: .image, url: urlString, identifier: identifier, alt: alt)
    }
    
    // For Hashable conformance
    func hash(into hasher: inout Hasher) {
        hasher.combine(id)
    }
    
    static func == (lhs: MediaItem, rhs: MediaItem) -> Bool {
        lhs.id == rhs.id
    }
}
