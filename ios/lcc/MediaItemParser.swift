import Foundation

/// Parses media items from JSON API responses.
/// Supports multiple formats:
/// - Array of strings: `["url1", "url2", ...]`
/// - Array of objects: `[{"url": "url1", "id": "...", "alt": "..."}, ...]`
/// - Object with images array: `{"images": ["url1", ...]}`
/// - Object with cameras array: `{"cameras": [{"kind": "img", "src": "url"}, ...]}`
struct MediaItemParser {
    private let logger = Logger(category: .networking)

    /// Parse media items from JSON data
    func parseMediaItems(from data: Data) throws -> [MediaItem] {
        let json = try JSONSerialization.jsonObject(with: data)

        // Try parsing as objects with identifiers first
        if let mediaItems = extractMediaItemsWithIdentifiers(from: json), !mediaItems.isEmpty {
            logger.debug("Parsed \(mediaItems.count) media items with identifiers from API")
            return mediaItems
        }

        // Fallback to URL strings format
        if let urlStrings = extractURLStrings(from: json), !urlStrings.isEmpty {
            logger.debug("Parsed \(urlStrings.count) URL strings from API")
            return urlStrings.compactMap { MediaItem.from(urlString: $0) }
        }

        throw MediaItemParserError.invalidJSONFormat
    }

    // MARK: - Top-level format detection

    func extractURLStrings(from json: Any) -> [String]? {
        if let stringArray = json as? [String] {
            return stringArray
        }
        if let objectArray = json as? [[String: Any]] {
            return extractURLs(fromObjectArray: objectArray)
        }
        if let object = json as? [String: Any] {
            return extractURLs(fromObject: object)
        }
        return nil
    }

    func extractMediaItemsWithIdentifiers(from json: Any) -> [MediaItem]? {
        if let objectArray = json as? [[String: Any]] {
            return objectArray.compactMap { extractMediaItem(from: $0) }
        }
        if let object = json as? [String: Any],
           let cameras = object["cameras"] as? [[String: Any]] {
            return cameras.compactMap { extractMediaItem(from: $0) }
        }
        if let object = json as? [String: Any],
           let images = object["images"] as? [[String: Any]] {
            return images.compactMap { extractMediaItem(from: $0) }
        }
        return nil
    }

    // MARK: - Single object extraction

    func extractMediaItem(from object: [String: Any]) -> MediaItem? {
        guard let urlString = extractURL(from: object) else { return nil }

        let identifier: String? = {
            if let id = object["id"] as? String { return id }
            if let id = object["identifier"] as? String { return id }
            if let id = object["idf"] as? String { return id }
            if let id = object["id"] as? Int { return String(id) }
            if let id = object["identifier"] as? Int { return String(id) }
            if let id = object["idf"] as? Int { return String(id) }
            return nil
        }()

        let alt: String? = {
            if let alt = object["alt"] as? String { return alt }
            if let alt = object["altText"] as? String { return alt }
            if let alt = object["alt_text"] as? String { return alt }
            if let alt = object["description"] as? String { return alt }
            if let alt = object["title"] as? String { return alt }
            return nil
        }()

        return MediaItem.from(urlString: urlString, identifier: identifier, alt: alt)
    }

    // MARK: - URL extraction helpers

    func extractURLs(fromObjectArray objects: [[String: Any]]) -> [String] {
        return objects.compactMap { extractURL(from: $0) }
    }

    func extractURLs(fromObject object: [String: Any]) -> [String]? {
        if let cameras = object["cameras"] as? [[String: Any]] {
            return cameras.compactMap { extractURL(from: $0) }
        }
        if let images = object["images"] as? [String] {
            return images
        }
        if let images = object["images"] as? [[String: Any]] {
            return images.compactMap { extractURL(from: $0) }
        }
        return nil
    }

    func extractURL(from object: [String: Any]) -> String? {
        if let iframe = object["iframe"] as? String { return iframe }
        if let url = object["url"] as? String { return url }
        if let src = object["src"] as? String { return src }
        return nil
    }
}

enum MediaItemParserError: LocalizedError {
    case invalidJSONFormat

    var errorDescription: String? {
        switch self {
        case .invalidJSONFormat:
            return "Invalid JSON format: couldn't extract media items"
        }
    }
}
