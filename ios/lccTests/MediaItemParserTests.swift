@testable import lcc
import Testing
import Foundation

@Suite("MediaItemParser Tests")
struct MediaItemParserTests {
    let parser = MediaItemParser()

    // MARK: - Array of strings format

    @Test("Parses array of URL strings")
    func parseStringArray() throws {
        let json = """
        ["https://example.com/img1.jpg", "https://example.com/img2.jpg"]
        """.data(using: .utf8)!

        let items = try parser.parseMediaItems(from: json)
        #expect(items.count == 2)
        #expect(items[0].url == "https://example.com/img1.jpg")
        #expect(items[1].url == "https://example.com/img2.jpg")
    }

    @Test("Parses single URL string array")
    func parseSingleStringArray() throws {
        let json = """
        ["https://example.com/img.jpg"]
        """.data(using: .utf8)!

        let items = try parser.parseMediaItems(from: json)
        #expect(items.count == 1)
    }

    // MARK: - Array of objects format

    @Test("Parses array of objects with url field")
    func parseObjectArrayWithURL() throws {
        let json = """
        [{"url": "https://example.com/img1.jpg"}, {"url": "https://example.com/img2.jpg"}]
        """.data(using: .utf8)!

        let items = try parser.parseMediaItems(from: json)
        #expect(items.count == 2)
        #expect(items[0].url == "https://example.com/img1.jpg")
    }

    @Test("Parses objects with identifier and alt text")
    func parseObjectsWithMetadata() throws {
        let json = """
        [{"url": "https://example.com/img.jpg", "id": "cam-1", "alt": "Mountain view"}]
        """.data(using: .utf8)!

        let items = try parser.parseMediaItems(from: json)
        #expect(items.count == 1)
        #expect(items[0].identifier == "cam-1")
        #expect(items[0].alt == "Mountain view")
    }

    @Test("Parses objects with src field")
    func parseObjectsWithSrc() throws {
        let json = """
        [{"src": "https://example.com/img.jpg", "kind": "img"}]
        """.data(using: .utf8)!

        let items = try parser.parseMediaItems(from: json)
        #expect(items.count == 1)
        #expect(items[0].url == "https://example.com/img.jpg")
    }

    @Test("Parses objects with iframe field")
    func parseObjectsWithIframe() throws {
        let json = """
        [{"iframe": "https://youtube.com/embed/abc123"}]
        """.data(using: .utf8)!

        let items = try parser.parseMediaItems(from: json)
        #expect(items.count == 1)
    }

    // MARK: - Object with cameras array

    @Test("Parses cameras array format")
    func parseCamerasArray() throws {
        let json = """
        {"cameras": [{"src": "https://example.com/cam1.jpg", "kind": "img"}, {"src": "https://example.com/cam2.jpg", "kind": "img"}]}
        """.data(using: .utf8)!

        let items = try parser.parseMediaItems(from: json)
        #expect(items.count == 2)
    }

    // MARK: - Object with images array

    @Test("Parses images string array format")
    func parseImagesStringArray() throws {
        let json = """
        {"images": ["https://example.com/img1.jpg", "https://example.com/img2.jpg"]}
        """.data(using: .utf8)!

        let items = try parser.parseMediaItems(from: json)
        #expect(items.count == 2)
    }

    @Test("Parses images object array format")
    func parseImagesObjectArray() throws {
        let json = """
        {"images": [{"url": "https://example.com/img1.jpg", "id": "1"}]}
        """.data(using: .utf8)!

        let items = try parser.parseMediaItems(from: json)
        #expect(items.count == 1)
        #expect(items[0].identifier == "1")
    }

    // MARK: - Identifier extraction variants

    @Test("Extracts identifier from 'identifier' field")
    func parseIdentifierField() throws {
        let json = """
        [{"url": "https://example.com/img.jpg", "identifier": "cam-abc"}]
        """.data(using: .utf8)!

        let items = try parser.parseMediaItems(from: json)
        #expect(items[0].identifier == "cam-abc")
    }

    @Test("Extracts identifier from 'idf' field")
    func parseIdfField() throws {
        let json = """
        [{"url": "https://example.com/img.jpg", "idf": "cam-xyz"}]
        """.data(using: .utf8)!

        let items = try parser.parseMediaItems(from: json)
        #expect(items[0].identifier == "cam-xyz")
    }

    @Test("Extracts integer identifiers")
    func parseIntegerIdentifier() throws {
        let json = """
        [{"url": "https://example.com/img.jpg", "id": 42}]
        """.data(using: .utf8)!

        let items = try parser.parseMediaItems(from: json)
        #expect(items[0].identifier == "42")
    }

    // MARK: - Alt text extraction variants

    @Test("Extracts alt from 'altText' field")
    func parseAltTextField() throws {
        let json = """
        [{"url": "https://example.com/img.jpg", "altText": "A snowy road"}]
        """.data(using: .utf8)!

        let items = try parser.parseMediaItems(from: json)
        #expect(items[0].alt == "A snowy road")
    }

    @Test("Extracts alt from 'alt_text' field")
    func parseAltUnderscoreField() throws {
        let json = """
        [{"url": "https://example.com/img.jpg", "alt_text": "Winter scene"}]
        """.data(using: .utf8)!

        let items = try parser.parseMediaItems(from: json)
        #expect(items[0].alt == "Winter scene")
    }

    @Test("Extracts alt from 'description' field")
    func parseDescriptionField() throws {
        let json = """
        [{"url": "https://example.com/img.jpg", "description": "Canyon camera"}]
        """.data(using: .utf8)!

        let items = try parser.parseMediaItems(from: json)
        #expect(items[0].alt == "Canyon camera")
    }

    @Test("Extracts alt from 'title' field")
    func parseTitleField() throws {
        let json = """
        [{"url": "https://example.com/img.jpg", "title": "LCC Camera 1"}]
        """.data(using: .utf8)!

        let items = try parser.parseMediaItems(from: json)
        #expect(items[0].alt == "LCC Camera 1")
    }

    // MARK: - Error cases

    @Test("Throws on invalid JSON format")
    func invalidJSONThrows() {
        let json = """
        {"unexpected": "format"}
        """.data(using: .utf8)!

        #expect(throws: MediaItemParserError.self) {
            try parser.parseMediaItems(from: json)
        }
    }

    @Test("Throws on empty array")
    func emptyArrayThrows() {
        let json = "[]".data(using: .utf8)!

        #expect(throws: MediaItemParserError.self) {
            try parser.parseMediaItems(from: json)
        }
    }

    @Test("Throws on invalid JSON")
    func invalidJSONDataThrows() {
        let json = "not json".data(using: .utf8)!

        #expect(throws: (any Error).self) {
            try parser.parseMediaItems(from: json)
        }
    }

    // MARK: - YouTube detection through parser

    @Test("Detects YouTube URLs as videos")
    func youtubeDetection() throws {
        let json = """
        ["https://youtube.com/embed/dQw4w9WgXcQ", "https://example.com/img.jpg"]
        """.data(using: .utf8)!

        let items = try parser.parseMediaItems(from: json)
        #expect(items.count == 2)
        #expect(items[0].type.isVideo == true)
        #expect(items[1].type.isVideo == false)
    }

    // MARK: - URL extraction helpers

    @Test("extractURL prefers iframe over url over src")
    func urlPriority() {
        let obj: [String: Any] = ["iframe": "iframe_url", "url": "url_val", "src": "src_val"]
        #expect(parser.extractURL(from: obj) == "iframe_url")

        let obj2: [String: Any] = ["url": "url_val", "src": "src_val"]
        #expect(parser.extractURL(from: obj2) == "url_val")

        let obj3: [String: Any] = ["src": "src_val"]
        #expect(parser.extractURL(from: obj3) == "src_val")
    }

    @Test("extractURL returns nil for empty object")
    func extractURLEmpty() {
        #expect(parser.extractURL(from: [:]) == nil)
    }

    // MARK: - Mixed content

    @Test("Handles mixed images and videos")
    func mixedContent() throws {
        let json = """
        [
            {"url": "https://example.com/cam1.jpg", "id": "cam1", "alt": "Camera 1"},
            {"iframe": "https://youtube.com/embed/abc123", "id": "vid1"},
            {"src": "https://example.com/cam2.jpg", "kind": "img"}
        ]
        """.data(using: .utf8)!

        let items = try parser.parseMediaItems(from: json)
        #expect(items.count == 3)
        #expect(items[0].type.isVideo == false)
        #expect(items[0].identifier == "cam1")
        #expect(items[0].alt == "Camera 1")
        #expect(items[1].type.isVideo == true)
        #expect(items[1].identifier == "vid1")
    }

    // MARK: - Objects without URL skipped

    @Test("Skips objects with no recognizable URL field")
    func skipsObjectsWithoutURL() throws {
        let json = """
        [{"name": "no-url"}, {"url": "https://example.com/img.jpg"}]
        """.data(using: .utf8)!

        let items = try parser.parseMediaItems(from: json)
        #expect(items.count == 1)
    }
}
