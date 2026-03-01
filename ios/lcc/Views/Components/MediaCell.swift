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
    @Environment(WeatherService.self) var weatherService
    @State private var isRetrying = false

    private var weather: WeatherStation? {
        guard let slug = mediaItem.slug else { return nil }
        return weatherService.station(for: slug)
    }

    var body: some View {
        ZStack(alignment: .bottom) {
            Group {
                if mediaItem.type.isVideo {
                    if case .youtubeVideo(let embedURL) = mediaItem.type {
                        YouTubeThumbnailView(
                            embedURL: embedURL,
                            width: imageWidth,
                            height: imageHeight
                        )
                        .clipped()
                        .onTapGesture { onTap() }
                        .accessibilityLabel("YouTube video")
                        .accessibilityAddTraits(.isButton)
                    }
                } else if let url = URL(string: mediaItem.url) {
                    let loadedImage = preloader.loadedImages[url]

                    if let uiImage = loadedImage {
                        Image(uiImage: uiImage)
                            .resizable()
                            .aspectRatio(contentMode: .fill)
                            .frame(width: imageWidth, height: imageHeight)
                            .clipped()
                            .transition(.opacity.animation(.easeIn(duration: 0.3)))
                            .onTapGesture { onTap() }
                            .accessibilityLabel(mediaItem.caption ?? "Camera image")
                            .accessibilityAddTraits(.isImage)
                    } else if preloader.loading.contains(url) || isRetrying || !hasCompletedInitialLoad {
                        ShimmerView(width: imageWidth, height: imageHeight, colorScheme: colorScheme)
                    } else {
                        Button(action: {
                            #if os(iOS)
                            UIImpactFeedbackGenerator(style: .light).impactOccurred()
                            #endif
                            isRetrying = true
                            onRetry()
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

                    // Subtle border while updating
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

            // Caption + weather footer — single row
            VStack(spacing: 0) {
                Spacer()
                HStack(spacing: 5) {
                    if let caption = mediaItem.caption {
                        Text(caption)
                            .font(.caption2)
                            .fontWeight(.medium)
                            .foregroundColor(.white)
                            .lineLimit(1)
                    }

                    if let w = weather {
                        if let temp = w.airTempF {
                            weatherChip("\(temp)\u{00B0}F")
                        }
                        if let wind = w.WindSpeedAvg {
                            let dir = w.WindDirection ?? ""
                            weatherChip("\(wind) mph \(dir)")
                        }
                        if let status = w.SurfaceStatus, !status.isEmpty {
                            weatherChip(status)
                        }
                    }

                    Spacer(minLength: 0)
                }
                .padding(.horizontal, 8)
                .padding(.vertical, 5)
                .frame(maxWidth: .infinity, alignment: .leading)
                .background(
                    LinearGradient(
                        gradient: Gradient(colors: [
                            Color.black.opacity(0),
                            Color.black.opacity(0.65)
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

    @ViewBuilder
    private func weatherChip(_ text: String) -> some View {
        Text(text)
            .font(.system(size: 10, weight: .medium))
            .foregroundStyle(.white.opacity(0.7))
            .padding(.horizontal, 4)
            .padding(.vertical, 1)
            .background(.white.opacity(0.1), in: RoundedRectangle(cornerRadius: 3))
            .lineLimit(1)
            .fixedSize()
    }

    private func precipIcon(_ value: String) -> String {
        if let v = Double(value), v > 0 {
            return "precip"
        }
        return value
    }
}
