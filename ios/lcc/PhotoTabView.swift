import Foundation
import SwiftUI

struct PhotoTabView: View {
    let mediaItems: [MediaItem]
    let roadConditions: [RoadCondition]

    @Binding public var gridMode: GridMode
    var onRequestFullScreen: (PresentedMedia) -> Void
    @Environment(ImagePreloader.self) var preloader
    @Environment(APIService.self) var apiService

    @Environment(\.colorScheme) var colorScheme
    @State private var isInitialLoadComplete = false

    private let logger = Logger(category: .ui)

    // User grid mode
    public enum GridMode: String, CaseIterable, Identifiable {
        case compact = "Compact"
        case single = "Single"
        var id: String { rawValue }
    }

    private let spacing: CGFloat = 2 // Minimal spacing for edge-to-edge feel

    var body: some View {
        GeometryReader { geometry in
            let availableWidth = geometry.size.width
            let availableHeight = geometry.size.height
            let columns = gridMode == .single ? 1 : max(1, Int(availableWidth / 180))
            let imageWidth = (availableWidth - CGFloat(columns - 1) * spacing) / CGFloat(columns)
            let imageHeight = imageWidth * (gridMode == .single ? 0.9 : 0.9)
            let gridItems = Array(repeating: GridItem(.fixed(imageWidth), spacing: spacing), count: columns)

            ZStack {
                Color.black.ignoresSafeArea(.all)

                // Main content
                ScrollViewReader { proxy in
                    ScrollView {
                        VStack(spacing: 0) {
                            if !roadConditions.isEmpty {
                                RoadConditionsBannerView(conditions: roadConditions)
                                    .padding(.top, 60)
                                    .animation(.spring(duration: 0.4), value: roadConditions.count)
                            }

                            if mediaItems.isEmpty {
                                if isInitialLoadComplete {
                                    EmptyStateView()
                                        .frame(width: availableWidth, height: availableHeight * 0.6)
                                }
                            } else {
                                LazyVGrid(columns: gridItems, spacing: spacing) {
                                    ForEach(mediaItems, id: \.id) { mediaItem in
                                        MediaCell(
                                            mediaItem: mediaItem,
                                            imageWidth: imageWidth,
                                            imageHeight: imageHeight,
                                            colorScheme: colorScheme,
                                            hasCompletedInitialLoad: isInitialLoadComplete,
                                            onTap: {
                                                onRequestFullScreen(PresentedMedia(mediaItem: mediaItem))
                                            },
                                            onRetry: {
                                                if !mediaItem.type.isVideo, let url = URL(string: mediaItem.url) {
                                                    preloader.retryImage(for: url)
                                                }
                                            }
                                        )
                                    }

                                    // Bottom spacer for grid mode toggle
                                    Color.clear
                                        .frame(height: 70)
                                        .gridCellColumns(columns)
                                }
                                .frame(maxWidth: .infinity)
                            }
                        }
                    }
                    .frame(minHeight: availableHeight)
                }
                .refreshable {
                    await performRefresh()
                }
                .scrollContentBackground(.hidden)
                .background(ScrollViewConfigurator())
                .ignoresSafeArea(edges: .all)
                .modifier(ZeroScrollContentMarginsIfAvailable())

                // Loading overlay — visible until API responds
                if !isInitialLoadComplete {
                    VStack(spacing: 0) {
                        InitialLoadingView()
                            .frame(maxWidth: .infinity, maxHeight: .infinity)
                    }
                    .frame(maxWidth: .infinity, maxHeight: .infinity)
                    .background(.ultraThinMaterial)
                    .ignoresSafeArea()
                    .transition(.opacity)
                }

                // Fades pinned to device edges
                Color.clear
                    .overlay(alignment: .top) {
                        LinearGradient(
                            gradient: Gradient(stops: [
                                .init(color: Color.black.opacity(0.7), location: 0.0),
                                .init(color: Color.black.opacity(0.35), location: 0.22),
                                .init(color: Color.clear, location: 1.0)
                            ]),
                            startPoint: .top,
                            endPoint: .bottom
                        )
                        .frame(height: 140)
                        .ignoresSafeArea(edges: .top)
                    }
                    .overlay(alignment: .bottom) {
                        LinearGradient(
                            gradient: Gradient(stops: [
                                .init(color: Color.clear, location: 0.0),
                                .init(color: Color.black.opacity(0.4), location: 0.88),
                                .init(color: Color.black.opacity(0.9), location: 1.0)
                            ]),
                            startPoint: .top,
                            endPoint: .bottom
                        )
                        .frame(height: 220)
                        .ignoresSafeArea(edges: .bottom)
                        .offset(y: 60)
                    }
                    .allowsHitTesting(false)
            }
            .ignoresSafeArea(edges: .all)
        }
        .onAppear {
            logger.info("📱 PhotoTabView appeared with \(mediaItems.count) media items")
            preloader.preloadMedia(from: mediaItems)

            // If data already available when tab appears (e.g. switching tabs), complete immediately
            if !isInitialLoadComplete {
                if !mediaItems.isEmpty || !apiService.isLoading {
                    isInitialLoadComplete = true
                }
            }
        }
        .onChange(of: mediaItems.isEmpty) { _, isEmpty in
            if !isEmpty && !isInitialLoadComplete {
                isInitialLoadComplete = true
            }
        }
        .onChange(of: apiService.isLoading) { _, isLoading in
            if !isLoading && !isInitialLoadComplete {
                isInitialLoadComplete = true
            }
        }
        .task {
            // Safety timeout: if initial load hasn't completed after 5 seconds, force it
            try? await Task.sleep(for: .seconds(5))
            if !isInitialLoadComplete {
                logger.warning("⏱️ Safety timeout triggered - forcing initial load completion")
                withAnimation(.easeOut(duration: 0.4)) {
                    isInitialLoadComplete = true
                }
            }
        }
    }

    private func performRefresh() async {
        #if os(iOS)
        UIImpactFeedbackGenerator(style: .medium).impactOccurred()
        #endif
        preloader.refreshImages()
        try? await Task.sleep(for: .milliseconds(500))
    }
}


struct PresentedMedia: Identifiable, Equatable {
    let id = UUID()
    let mediaItem: MediaItem
}

// Helper to find and configure UIScrollView to remove insets
struct ScrollViewConfigurator: UIViewRepresentable {
    func makeUIView(context: Context) -> UIView {
        let view = UIView()
        view.backgroundColor = .clear

        // Delay to let SwiftUI create the ScrollView
        DispatchQueue.main.async {
            self.configureScrollView(in: view)
        }

        return view
    }

    func updateUIView(_ uiView: UIView, context: Context) {
        configureScrollView(in: uiView)
    }

    private func configureScrollView(in view: UIView) {
        var responder: UIResponder? = view
        while responder != nil {
            if let scrollView = responder as? UIScrollView {
                scrollView.backgroundColor = .clear
                scrollView.contentInsetAdjustmentBehavior = .never
                scrollView.contentInset = .zero
                scrollView.scrollIndicatorInsets = .zero
                scrollView.automaticallyAdjustsScrollIndicatorInsets = false
                break
            }
            responder = responder?.next
        }
    }
}

// Apply zero content margins so scroll content extends edge-to-edge
private struct ZeroScrollContentMarginsIfAvailable: ViewModifier {
    func body(content: Content) -> some View {
        content.contentMargins(.zero, for: .scrollContent)
    }
}

#Preview {
    @Previewable @State var preloader = ImagePreloader()
    let mediaItems = [
        "https://lcc.live/image/aHR0cHM6Ly9iMTAuaGRyZWxheS5jb20vY2FtZXJhLzg2MTFlMjc2LTdlZTUtNDJjMC1iOGNkLWQ5ZTE4OTBlMWNkNC9zbmFwc2hvdA==",
        "https://lcc.live/image/aHR0cHM6Ly91ZG90dHJhZmZpYy51dGFoLmdvdi8xX2RldmljZXMvYXV4MTQ2MDQuanBlZw==",
        "https://youtube.com/embed/dQw4w9WgXcQ",
        "https://lcc.live/image/aHR0cHM6Ly91ZG90dHJhZmZpYy51dGFoLmdvdi8xX2RldmljZXMvYXV4MTY2NDcuanBlZw==",
        "https://lcc.live/image/aHR0cHM6Ly91ZG90dHJhZmZpYy51dGFoLmdvdi8xX2RldmljZXMvYXV4MTYyNjUuanBlZw==",
        "https://lcc.live/image/aHR0cHM6Ly91ZG90dHJhZmZpYy51dGFoLmdvdi8xX2RldmljZXMvYXV4MTYyNjYuanBlZw==",
        "https://lcc.live/image/aHR0cHM6Ly91ZG90dHJhZmZpYy51dGFoLmdvdi8xX2RldmljZXMvYXV4MTYyNjguanBlZw==",
        "https://lcc.live/image/aHR0cHM6Ly91ZG90dHJhZmZpYy51dGFoLmdvdi8xX2RldmljZXMvYXV4MTYyNjkuanBlZw=="
    ].compactMap { MediaItem.from(urlString: $0) }

    PhotoTabView(
        mediaItems: mediaItems,
        roadConditions: [],
        gridMode: .constant(PhotoTabView.GridMode.single),
        onRequestFullScreen: { _ in })
    .environment(preloader)
    .environment(APIService())
    .environment(WeatherService())
}

#Preview("With Mock Images") {
    let mediaItems = [
        "https://lcc.live/image/aHR0cHM6Ly9iMTAuaGRyZWxheS5jb20vY2FtZXJhLzg2MTFlMjc2LTdlZTUtNDJjMC1iOGNkLWQ5ZTE4OTBlMWNkNC9zbmFwc2hvdA==",
        "https://lcc.live/image/aHR0cHM6Ly91ZG90dHJhZmZpYy51dGFoLmdvdi8xX2RldmljZXMvYXV4MTQ2MDQuanBlZw==",
        "https://lcc.live/image/aHR0cHM6Ly91ZG90dHJhZmZpYy51dGFoLmdvdi8xX2RldmljZXMvYXV4MTY2NDcuanBlZw=="
    ].compactMap { MediaItem.from(urlString: $0) }

    PhotoTabView(
        mediaItems: mediaItems,
        roadConditions: [],
        gridMode: .constant(PhotoTabView.GridMode.compact),
        onRequestFullScreen: { _ in })
    .environment(ImagePreloader())
    .environment(APIService())
    .environment(WeatherService())
}
