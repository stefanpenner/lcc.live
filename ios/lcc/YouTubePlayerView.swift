import SwiftUI
import WebKit

/// A view that displays YouTube videos using WKWebView
struct YouTubePlayerView: UIViewRepresentable {
    let embedURL: String
    let autoplay: Bool
    
    init(embedURL: String, autoplay: Bool = false) {
        self.embedURL = embedURL
        self.autoplay = autoplay
    }
    
    func makeUIView(context: Context) -> WKWebView {
        let configuration = WKWebViewConfiguration()
        configuration.allowsInlineMediaPlayback = true
        configuration.allowsAirPlayForMediaPlayback = true
        configuration.allowsPictureInPictureMediaPlayback = true
        configuration.mediaTypesRequiringUserActionForPlayback = []  // Allow autoplay
        
        let webView = WKWebView(frame: .zero, configuration: configuration)
        webView.scrollView.isScrollEnabled = false
        webView.backgroundColor = .black
        webView.isOpaque = false
        
        return webView
    }
    
    func updateUIView(_ webView: WKWebView, context: Context) {
        // Use the exact same iframe embed code as the website
        // Don't modify the URL - use it exactly as provided from the API
        // Escape quotes for safe HTML attribute embedding
        let escapedURL = embedURL
            .replacingOccurrences(of: "\"", with: "&quot;")
            .replacingOccurrences(of: "'", with: "&#39;")
        
        // Use the exact same HTML structure as the website
        let htmlString = """
        <!DOCTYPE html>
        <html>
        <head>
            <meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no">
            <style>
                * {
                    margin: 0;
                    padding: 0;
                    box-sizing: border-box;
                }
                html, body {
                    width: 100%;
                    height: 100%;
                    background-color: black;
                    overflow: hidden;
                }
                iframe {
                    position: absolute;
                    top: 0;
                    left: 0;
                    width: 100%;
                    height: 100%;
                    border: none;
                }
            </style>
        </head>
        <body>
            <iframe 
                loading="lazy" 
                src="\(escapedURL)" 
                frameborder="0"
                allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share" 
                referrerpolicy="strict-origin-when-cross-origin"
                allowfullscreen>
            </iframe>
        </body>
        </html>
        """
        
        // CRITICAL: Set a proper baseURL so WKWebView sends the Referer header
        // Error 153 occurs when YouTube doesn't receive a proper referrer header
        // Using the app's bundle identifier as a custom scheme ensures referrer is sent
        // Alternatively, use a valid HTTPS URL that matches your app's domain
        let baseURL = URL(string: "https://lcc.live")
        
        Logger.ui.debug("🎥 Loading YouTube video (exact website format): \(embedURL)")
        webView.loadHTMLString(htmlString, baseURL: baseURL)
    }
}

/// A thumbnail view for YouTube videos (shows a play button overlay)
struct YouTubeThumbnailView: View {
    let embedURL: String
    let width: CGFloat
    let height: CGFloat
    
    @Environment(\.colorScheme) private var colorScheme
    @State private var thumbnailURL: URL?
    @State private var thumbnailLoadFailed = false
    @State private var isLoading = true
    
    var body: some View {
        ZStack {
            if let thumbnailURL = thumbnailURL, !thumbnailLoadFailed {
                AsyncImage(url: thumbnailURL) { phase in
                    switch phase {
                    case .success(let image):
                        image
                            .resizable()
                            .aspectRatio(contentMode: .fill)
                            .frame(width: width, height: height)
                            .clipped()
                            .onAppear {
                                isLoading = false
                            }
                    case .failure:
                        // Show placeholder on failure
                        placeholderView
                            .task {
                                thumbnailLoadFailed = true
                                isLoading = false
                            }
                    case .empty:
                        // Show shimmer loading state while fetching thumbnail
                        ShimmerView(width: width, height: height, colorScheme: colorScheme)
                    @unknown default:
                        placeholderView
                    }
                }
            } else if isLoading {
                // Show shimmer while extracting thumbnail URL
                ShimmerView(width: width, height: height, colorScheme: colorScheme)
            } else {
                placeholderView
            }
            
            // Enhanced play button overlay with native glass effect
            // Only show when we have a thumbnail or placeholder (not during shimmer)
            if !isLoading {
                Button(action: {}) {
                    Image(systemName: "play.fill")
                        .font(.system(size: 24, weight: .medium))
                        .foregroundColor(.white)
                        .offset(x: 2) // Slight offset to visually center the play icon
                }
                .frame(width: 60, height: 60)
                .glassEffect(
                    Glass.regular
                        .tint(Color.white.opacity(0.15))
                        .interactive(true),
                    in: Circle()
                )
                .disabled(true) // Disabled since this is just a thumbnail
            }
        }
        .frame(width: width, height: height)
        .task(id: embedURL) {
            // Use .task(id:) instead of .onAppear for more reliable execution in LazyVGrid
            // The id parameter ensures it re-runs if embedURL changes and cancels previous tasks
            // This is more reliable than onAppear for lazy-loaded views
            // Reset state when embedURL changes
            thumbnailURL = nil
            thumbnailLoadFailed = false
            isLoading = true
            extractThumbnailURL()
        }
    }
    
    private var placeholderView: some View {
        ZStack {
            Rectangle()
                .fill(
                    LinearGradient(
                        colors: colorScheme == .dark ?
                            [Color(red: 0.18, green: 0.18, blue: 0.20), Color(red: 0.22, green: 0.22, blue: 0.24)] :
                            [Color(red: 0.95, green: 0.95, blue: 0.97), Color(red: 0.92, green: 0.92, blue: 0.94)],
                        startPoint: .topLeading,
                        endPoint: .bottomTrailing
                    )
                )
            
            VStack(spacing: 8) {
                Image(systemName: "video.fill")
                    .font(.system(size: 40))
                    .foregroundColor(colorScheme == .dark ? .white.opacity(0.5) : .black.opacity(0.4))
                
                Text("Video")
                    .font(.caption)
                    .foregroundColor(colorScheme == .dark ? .white.opacity(0.4) : .black.opacity(0.3))
            }
            .glassEffect(
                Glass.regular
                    .tint(Color.white.opacity(0.1)),
                in: RoundedRectangle(cornerRadius: 12)
            )
            .padding()
        }
        .frame(width: width, height: height)
    }
    
    private func extractThumbnailURL() {
        // Extract video ID first to ensure we have it
        guard let videoID = YouTubeURLHelper.extractVideoID(from: embedURL) else {
            Logger.ui.warning("⚠️ Failed to extract video ID from embed URL: \(embedURL)")
            isLoading = false
            return
        }
        
        Logger.ui.debug("✅ Extracted video ID: \(videoID) from URL: \(embedURL)")
        
        // Try maxresdefault first (highest quality), fallback handled by AsyncImage if it fails
        // YouTube thumbnail quality options:
        // - maxresdefault: Maximum resolution (1280x720 or higher)
        // - hqdefault: High quality (480x360)
        // - mqdefault: Medium quality (320x180)
        // - default: Default quality (120x90)
        if let url = URL(string: "https://img.youtube.com/vi/\(videoID)/maxresdefault.jpg") {
            thumbnailURL = url
            Logger.ui.debug("✅ Generated YouTube thumbnail URL (maxresdefault): \(url.absoluteString)")
        } else {
            // Fallback to mqdefault if maxresdefault URL construction fails
            if let url = URL(string: "https://img.youtube.com/vi/\(videoID)/mqdefault.jpg") {
                thumbnailURL = url
                Logger.ui.debug("✅ Generated YouTube thumbnail URL (mqdefault fallback): \(url.absoluteString)")
            } else {
                Logger.ui.warning("⚠️ Failed to generate thumbnail URL for video ID: \(videoID)")
                isLoading = false
            }
        }
    }
}
