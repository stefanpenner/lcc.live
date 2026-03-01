import Foundation

/// Utility for parsing and converting YouTube URLs
/// Supports multiple YouTube URL formats:
/// - Embed URLs: youtube.com/embed/VIDEO_ID
/// - Watch URLs: youtube.com/watch?v=VIDEO_ID
/// - Short URLs: youtu.be/VIDEO_ID
/// - Iframe tags: <iframe src="youtube.com/embed/VIDEO_ID">
enum YouTubeURLHelper {
    
    /// Extract video ID from various YouTube URL formats
    /// - Parameter urlString: YouTube URL in any supported format
    /// - Returns: Video ID if found, nil otherwise
    static func extractVideoID(from urlString: String) -> String? {
        // Pattern 1: Embed URL (youtube.com/embed/VIDEO_ID)
        if urlString.contains("youtube.com/embed/") {
            // Try parsing as URL first
            if let url = URL(string: urlString) {
                let videoID = url.lastPathComponent.components(separatedBy: "?").first
                if let videoID = videoID, !videoID.isEmpty, videoID != "embed" {
                    return videoID
                }
            }
            // Also try with https:// prefix if missing
            if !urlString.hasPrefix("http://") && !urlString.hasPrefix("https://") {
                if let url = URL(string: "https://\(urlString)") {
                    let videoID = url.lastPathComponent.components(separatedBy: "?").first
                    if let videoID = videoID, !videoID.isEmpty, videoID != "embed" {
                        return videoID
                    }
                }
            }
            // Also check for embed URL in iframe tags or plain text using regex
            // Extract video ID directly using regex pattern
            if let regex = try? NSRegularExpression(pattern: "youtube\\.com/embed/([a-zA-Z0-9_-]+)", options: []),
               let match = regex.firstMatch(in: urlString, options: [], range: NSRange(urlString.startIndex..., in: urlString)),
               match.numberOfRanges > 1 {
                guard let videoIDRange = Range(match.range(at: 1), in: urlString) else { return nil }
                let videoID = String(urlString[videoIDRange])
                if !videoID.isEmpty {
                    return videoID
                }
            }
        }
        
        // Pattern 2: Watch URL (youtube.com/watch?v=VIDEO_ID)
        if urlString.contains("youtube.com/watch") {
            // Try parsing as URL with components
            if let url = URL(string: urlString),
               let components = URLComponents(url: url, resolvingAgainstBaseURL: false),
               let videoID = components.queryItems?.first(where: { $0.name == "v" })?.value,
               !videoID.isEmpty {
                return videoID
            }
            // Also try with https:// prefix if missing
            if !urlString.hasPrefix("http://") && !urlString.hasPrefix("https://") {
                if let url = URL(string: "https://\(urlString)"),
                   let components = URLComponents(url: url, resolvingAgainstBaseURL: false),
                   let videoID = components.queryItems?.first(where: { $0.name == "v" })?.value,
                   !videoID.isEmpty {
                    return videoID
                }
            }
        }
        
        // Pattern 3: Short URL (youtu.be/VIDEO_ID)
        if urlString.contains("youtu.be/") {
            if let url = URL(string: urlString) {
                let videoID = url.lastPathComponent.components(separatedBy: "?").first
                if let videoID = videoID, !videoID.isEmpty {
                    return videoID
                }
            }
            // Also try with https:// prefix if missing
            if !urlString.hasPrefix("http://") && !urlString.hasPrefix("https://") {
                if let url = URL(string: "https://\(urlString)") {
                    let videoID = url.lastPathComponent.components(separatedBy: "?").first
                    if let videoID = videoID, !videoID.isEmpty {
                        return videoID
                    }
                }
            }
        }
        
        return nil
    }
    
    /// Convert any YouTube URL format to embed URL
    /// - Parameter urlString: YouTube URL in any supported format
    /// - Returns: Embed URL if valid YouTube URL, nil otherwise
    static func extractEmbedURL(from urlString: String) -> String? {
        // Pattern 1: Check for iframe with YouTube embed
        if urlString.contains("iframe") && urlString.contains("youtube.com/embed/") {
            if let range = urlString.range(of: "youtube\\.com/embed/[a-zA-Z0-9_-]+", options: .regularExpression) {
                let embedPath = String(urlString[range])
                return "https://www.\(embedPath)"
            }
        }
        
        // Pattern 2: Direct embed URL (already in embed format)
        if urlString.contains("youtube.com/embed/") {
            // Extract video ID to ensure it's valid
            if let videoID = extractVideoID(from: urlString) {
                // If URL already has proper scheme and is a valid embed URL, preserve it (including query params)
                if urlString.hasPrefix("http://") || urlString.hasPrefix("https://") {
                    // Validate the URL is properly formatted and contains the video ID
                    if let url = URL(string: urlString),
                       url.host?.contains("youtube.com") ?? false,
                       url.path.contains("/embed/") {
                        // Return the original URL to preserve any query parameters
                        return urlString
                    }
                }
                // Construct proper embed URL (without query params - they'll be added by the player)
                return "https://www.youtube.com/embed/\(videoID)"
            }
        }
        
        // Pattern 3: Watch URL (youtube.com/watch?v=VIDEO_ID)
        if urlString.contains("youtube.com/watch") {
            if let videoID = extractVideoID(from: urlString) {
                return "https://www.youtube.com/embed/\(videoID)"
            }
        }
        
        // Pattern 4: Short URL (youtu.be/VIDEO_ID)
        if urlString.contains("youtu.be/") {
            if let videoID = extractVideoID(from: urlString) {
                return "https://www.youtube.com/embed/\(videoID)"
            }
        }
        
        return nil
    }
    
    /// Convert embed URL to watch URL
    /// - Parameter embedURL: YouTube embed URL
    /// - Returns: Watch URL if valid, nil otherwise
    static func watchURL(from embedURL: String) -> String? {
        guard let videoID = extractVideoID(from: embedURL) else {
            return nil
        }
        return "https://www.youtube.com/watch?v=\(videoID)"
    }
    
    /// Generate thumbnail URL for a YouTube video
    /// - Parameter urlString: YouTube URL in any supported format
    /// - Parameter quality: Thumbnail quality (default, mqdefault, hqdefault, sddefault, maxresdefault)
    /// - Returns: Thumbnail URL if valid YouTube URL, nil otherwise
    static func thumbnailURL(from urlString: String, quality: String = "mqdefault") -> URL? {
        guard let videoID = extractVideoID(from: urlString) else {
            return nil
        }
        return URL(string: "https://img.youtube.com/vi/\(videoID)/\(quality).jpg")
    }
    
    /// Check if a URL string is a YouTube URL
    /// - Parameter urlString: URL string to check
    /// - Returns: True if URL appears to be a YouTube URL
    static func isYouTubeURL(_ urlString: String) -> Bool {
        return urlString.contains("youtube.com") || 
               urlString.contains("youtu.be") ||
               (urlString.contains("iframe") && urlString.contains("youtube.com/embed/"))
    }
}

