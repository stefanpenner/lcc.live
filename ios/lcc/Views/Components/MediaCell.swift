import SwiftUI

struct MediaCell: View {
    let mediaItem: MediaItem
    let imageWidth: CGFloat
    let imageHeight: CGFloat
    let colorScheme: ColorScheme
    let hasCompletedInitialLoad: Bool
    let onTap: () -> Void
    let onRetry: () -> Void

    @Environment(ImagePreloader.self) var preloader
    @State private var isRetrying = false

    var body: some View {
        ZStack(alignment: .bottom) {
            Group {
                if mediaItem.type.isVideo {
                    // Show YouTube thumbnail with play button
                    if case .youtubeVideo(let embedURL) = mediaItem.type {
                        YouTubeThumbnailView(
                            embedURL: embedURL,
                            width: imageWidth,
                            height: imageHeight
                        )
                        .clipped()
                        .onTapGesture {
                            onTap()
                        }
                        .accessibilityLabel("YouTube video")
                        .accessibilityAddTraits(.isButton)
                    }
                } else if let url = URL(string: mediaItem.url) {
                    // Show image
                    let loadedImage = preloader.loadedImages[url]
                    
                    if let uiImage = loadedImage {
                        // Show preloaded image
                        Image(uiImage: uiImage)
                            .resizable()
                            .aspectRatio(contentMode: .fill)
                            .frame(width: imageWidth, height: imageHeight)
                            .clipped()
                            .transition(.opacity.animation(.easeIn(duration: 0.3)))
                            .onTapGesture {
                                onTap()
                            }
                            .accessibilityLabel("Camera image")
                            .accessibilityAddTraits(.isImage)
                    } else if preloader.loading.contains(url) || isRetrying || !hasCompletedInitialLoad {
                        // Show shimmer loading state (including during initial load)
                        ShimmerView(width: imageWidth, height: imageHeight, colorScheme: colorScheme)
                    } else {
                        // Show error state with retry (only after initial load completes)
                        Button(action: {
                            #if os(iOS)
                            UIImpactFeedbackGenerator(style: .light).impactOccurred()
                            #endif
                            isRetrying = true
                            onRetry()
                            // Reset after brief delay to allow loading state to show
                            Task {
                                try? await Task.sleep(for: .milliseconds(300))
                                isRetrying = false
                            }
                        }) {
                            VStack(spacing: 8) {
                                Image(systemName: "arrow.clockwise.circle.fill")
                                    .resizable()
                                    .scaledToFit()
                                    .frame(width: 36, height: 36)
                                    .foregroundColor(Color.accentColor.opacity(0.8))
                                Text("Tap to retry")
                                    .font(.caption)
                                    .foregroundColor(.secondary)
                            }
                            .frame(width: imageWidth, height: imageHeight)
                            .background(
                                RoundedRectangle(cornerRadius: 8)
                                    .fill(
                                        LinearGradient(
                                            colors: colorScheme == .dark ?
                                                [Color(red: 0.18, green: 0.13, blue: 0.13), Color(red: 0.22, green: 0.16, blue: 0.18)] :
                                                [Color(red: 0.99, green: 0.95, blue: 0.92), Color(red: 0.95, green: 0.92, blue: 0.99)],
                                            startPoint: .topLeading,
                                            endPoint: .bottomTrailing
                                        )
                                    )
                            )
                            .clipShape(RoundedRectangle(cornerRadius: 8))
                        }
                        .buttonStyle(.plain)
                        .accessibilityLabel("Failed to load image")
                        .accessibilityHint("Tap to retry loading")
                    }
                    
                    // Subtle border while updating (only for images)
                    let isLoading = preloader.loading.contains(url)
                    let fadeDate = preloader.fadingOut[url]
                    let isFadingOut = fadeDate != nil
                    let fadeProgress: CGFloat = {
                        guard let fadeDate = fadeDate else { return 0 }
                        let elapsed = CGFloat(Date().timeIntervalSince(fadeDate))
                        let duration: CGFloat = 3.0
                        return min(1, max(0, elapsed / duration))
                    }()
                    let borderOpacity: CGFloat = isFadingOut ? (1 - fadeProgress) : (isLoading ? 0.3 : 0)
                    Rectangle()
                        .stroke(Color.accentColor.opacity(0.60), lineWidth: 3)
                        .frame(width: imageWidth, height: imageHeight)
                        .opacity(borderOpacity)
                        .animation(.easeInOut(duration: 0.4), value: borderOpacity)
                }
            }
            
            // Subtle caption overlay
            if let caption = mediaItem.caption {
                VStack {
                    Spacer()
                    HStack {
                        Text(caption)
                            .font(.caption2)
                            .fontWeight(.medium)
                            .foregroundColor(.white)
                            .lineLimit(1)
                            .padding(.horizontal, 8)
                            .padding(.vertical, 4)
                        Spacer()
                    }
                    .background(
                        LinearGradient(
                            gradient: Gradient(colors: [
                                Color.black.opacity(0),
                                Color.black.opacity(0.6)
                            ]),
                            startPoint: .top,
                            endPoint: .bottom
                        )
                    )
                }
                .frame(width: imageWidth, height: imageHeight)
                .allowsHitTesting(false)
            }
        }
    }
}

