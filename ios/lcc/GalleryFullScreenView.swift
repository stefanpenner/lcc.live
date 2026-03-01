import SwiftUI

struct GalleryFullScreenView: View {
    let items: [MediaItem]
    let initialIndex: Int
    let onClose: () -> Void

    @State private var index: Int
    @State private var dragOffset: CGFloat = 0
    @State private var isDragging: Bool = false
    @State private var dismissOpacity: Double = 1.0
    @State private var backgroundOpacity: Double = 1.0
    @State private var maxDragDistance: CGFloat = 0
    @State private var hasTriggeredDismissHaptic: Bool = false
    @Environment(ImagePreloader.self) var preloader
    @Environment(WeatherService.self) var weatherService
    @Environment(\.verticalSizeClass) private var verticalSizeClass
    
    // MARK: - Constants
    private enum DragConstants {
        static let minimumDistance: CGFloat = 20
        static let verticalPreferenceRatio: CGFloat = 1.5
        static let dismissalThreshold: CGFloat = 200
        static let dismissalVelocityThreshold: CGFloat = 800
        static let dragBackCancelRatio: CGFloat = 0.5
        static let rubberBandResistance: CGFloat = 0.3
        static let minDismissOpacity: Double = 0.3
        static let dismissAnimationDuration: TimeInterval = 0.25
        static let springResponse: Double = 0.35
        static let springDamping: Double = 0.85
    }
    
    init(items: [MediaItem], initialIndex: Int, onClose: @escaping () -> Void) {
        self.items = items
        self.initialIndex = clampIndex(initialIndex, count: items.count)
        self.onClose = onClose
        _index = State(initialValue: self.initialIndex)
    }

    var body: some View {
        GeometryReader { geometry in
            NavigationStack {
                ZStack {
                    // Black background that fades out when dragging
                    Color.black
                        .opacity(backgroundOpacity)
                        .zIndex(0)
                    
                    TabView(selection: $index) {
                        ForEach(items.indices, id: \.self) { i in
                            page(for: items[i])
                                .frame(maxWidth: .infinity, maxHeight: .infinity)
                                .background(Color.black)
                                .ignoresSafeArea(edges: .top)
                                .overlay(alignment: .bottom) {
                                    captionOverlay(item: items[i])
                                }
                                .tag(i)
                                .onAppear {
                                    preloadAdjacentImages(currentIndex: i)
                                }
                        }
                    }
                    .tabViewStyle(.page(indexDisplayMode: .never))
                    .background(Color.black)
                    .zIndex(1)
                    .disabled(isDragging)
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
                .background(Color.black)
                .ignoresSafeArea(edges: .top)
                .toolbar {
                    ToolbarItem(placement: .topBarLeading) {
                        Button("Close", action: onClose)
                            .foregroundStyle(.white)
                            .accessibilityLabel("Close gallery")
                    }
                    ToolbarItem(placement: .topBarTrailing) {
                        if let url = galleryShareURL(for: currentItem) {
                            ShareLink(item: url) {
                                Image(systemName: "square.and.arrow.up")
                                    .foregroundStyle(.white)
                            }
                            .accessibilityLabel("Share image")
                        }
                    }
                }
                .toolbarBackground(.hidden, for: .navigationBar)
                .navigationBarTitleDisplayMode(.inline)
                .offset(y: dragOffset)
                .opacity(dismissOpacity)
                .gesture(
                    dragGesture(geometry: geometry),
                    including: .all
                )
            }
        }
        .ignoresSafeArea(edges: .all)
        .preferredColorScheme(.dark)
    }
    
    // MARK: - View Builders
    
    private func weather(for item: MediaItem) -> WeatherStation? {
        guard let slug = item.slug else { return nil }
        return weatherService.station(for: slug)
    }

    private var isLandscape: Bool {
        verticalSizeClass == .compact
    }

    @ViewBuilder
    private func captionOverlay(item: MediaItem) -> some View {
        let ws = weather(for: item)
        let hasContent = item.caption != nil || ws != nil

        if hasContent {
            VStack(alignment: .leading, spacing: isLandscape ? 5 : 8) {
                Spacer()

                if let caption = item.caption {
                    Text(caption)
                        .font(isLandscape ? .caption : .subheadline)
                        .fontWeight(.semibold)
                        .foregroundColor(.white)
                        .shadow(color: .black.opacity(0.5), radius: 4, x: 0, y: 2)
                }

                if let ws = ws {
                    if isLandscape {
                        weatherChipsRow(ws)
                    } else {
                        weatherDetailsGrid(ws)
                    }
                }
            }
            .padding(.horizontal, 20)
            .padding(.bottom, isLandscape ? 16 : 60)
            .padding(.top, 40)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(
                LinearGradient(
                    gradient: Gradient(colors: [
                        Color.clear,
                        Color.black.opacity(0.5)
                    ]),
                    startPoint: .top,
                    endPoint: .bottom
                )
                .ignoresSafeArea(edges: .bottom)
            )
            .allowsHitTesting(false)
        }
    }

    // Landscape: single horizontal row of compact chips
    @ViewBuilder
    private func weatherChipsRow(_ ws: WeatherStation) -> some View {
        HStack(spacing: 8) {
            if let temp = ws.AirTemperature {
                weatherChip("thermometer", "\(temp)\u{00B0}F")
            }
            if let surface = ws.SurfaceTemp {
                weatherChip("road.lanes", "Sfc \(surface)\u{00B0}F")
            }
            if let status = ws.SurfaceStatus, !status.isEmpty {
                weatherChip("circle.dotted", status)
            }
            if let wind = ws.WindSpeedAvg {
                let dir = ws.WindDirection ?? ""
                weatherChip("wind", "\(wind) mph \(dir)")
            }
            if let gust = ws.WindSpeedGust {
                weatherChip("wind", "G \(gust)")
            }
            if let humidity = ws.RelativeHumidity {
                weatherChip("humidity", "\(humidity)%")
            }
            if let dewpoint = ws.DewpointTemp {
                weatherChip("thermometer.snowflake", "\(dewpoint)\u{00B0}F")
            }
            if let precip = ws.Precipitation, !precip.isEmpty {
                weatherChip("drop.fill", precip)
            }
        }
    }

    @ViewBuilder
    private func weatherChip(_ icon: String, _ value: String) -> some View {
        HStack(spacing: 4) {
            Image(systemName: icon)
                .font(.system(size: 10))
                .foregroundStyle(.white.opacity(0.6))
            Text(value)
                .font(.system(size: 11, weight: .medium))
                .foregroundStyle(.white.opacity(0.85))
                .lineLimit(1)
        }
        .padding(.horizontal, 6)
        .padding(.vertical, 3)
        .background(.white.opacity(0.12), in: RoundedRectangle(cornerRadius: 5))
        .fixedSize()
    }

    // Portrait: 2-column grid
    private let weatherColumns = [
        GridItem(.flexible(), spacing: 10),
        GridItem(.flexible(), spacing: 10)
    ]

    @ViewBuilder
    private func weatherDetailsGrid(_ ws: WeatherStation) -> some View {
        LazyVGrid(columns: weatherColumns, alignment: .leading, spacing: 8) {
            if let temp = ws.AirTemperature {
                weatherDetail("thermometer", "\(temp)\u{00B0}F")
            }
            if let surface = ws.SurfaceTemp {
                weatherDetail("road.lanes", "Sfc \(surface)\u{00B0}F")
            }
            if let status = ws.SurfaceStatus, !status.isEmpty {
                weatherDetail("circle.dotted", status)
            }
            if let wind = ws.WindSpeedAvg {
                let dir = ws.WindDirection ?? ""
                weatherDetail("wind", "\(wind) mph \(dir)")
            }
            if let gust = ws.WindSpeedGust {
                weatherDetail("wind", "Gust \(gust) mph")
            }
            if let humidity = ws.RelativeHumidity {
                weatherDetail("humidity", "\(humidity)%")
            }
            if let dewpoint = ws.DewpointTemp {
                weatherDetail("thermometer.snowflake", "Dew \(dewpoint)\u{00B0}F")
            }
            if let precip = ws.Precipitation, !precip.isEmpty {
                weatherDetail("drop.fill", precip)
            }
        }
    }

    @ViewBuilder
    private func weatherDetail(_ icon: String, _ value: String) -> some View {
        HStack(spacing: 5) {
            Image(systemName: icon)
                .font(.system(size: 12))
                .foregroundStyle(.white.opacity(0.55))
                .frame(width: 16)
            Text(value)
                .font(.callout)
                .foregroundStyle(.white.opacity(0.9))
                .lineLimit(1)
        }
    }

    @ViewBuilder
    private func page(for media: MediaItem) -> some View {
        switch media.type {
        case .image:
            if let url = URL(string: media.url) {
                imagePage(url: url)
            } else {
                Color.black
            }
        case .youtubeVideo(let embedURL):
            YouTubePlayerView(embedURL: embedURL, autoplay: true)
        }
    }
    
    @ViewBuilder
    private func imagePage(url: URL) -> some View {
        if let uiImage = preloader.loadedImages[url] {
            ZoomableImageView(image: Image(uiImage: uiImage))
                .onAppear {
                    preloader.recordAccess(for: url)
                }
        } else if preloader.loading.contains(url) {
            loadingView
        } else {
            errorView(url: url)
        }
    }
    
    private var loadingView: some View {
        ZStack {
            Color.black
            ProgressView()
                .tint(.white)
        }
    }
    
    private func errorView(url: URL) -> some View {
        ZStack {
            Color.black
            VStack(spacing: 12) {
                Image(systemName: "photo.circle")
                    .font(.system(size: 48))
                    .foregroundColor(.white.opacity(0.6))
                Button("Tap to Load") {
                    preloader.retryImage(for: url)
                }
                .foregroundColor(.white)
                .padding(.horizontal, 20)
                .padding(.vertical, 10)
                .background(Color.white.opacity(0.2))
                .cornerRadius(10)
                .accessibilityLabel("Retry loading image")
            }
        }
        .onAppear {
            preloader.loadImageImmediately(for: url)
        }
    }

    // MARK: - Gestures
    
    private func dragGesture(geometry: GeometryProxy) -> some Gesture {
        DragGesture(minimumDistance: DragConstants.minimumDistance)
            .onChanged { value in
                handleDragChanged(value: value)
            }
            .onEnded { value in
                handleDragEnded(value: value, geometry: geometry)
            }
    }
    
    private func handleDragChanged(value: DragGesture.Value) {
        let translation = value.translation.height
        let horizontal = abs(value.translation.width)
        let vertical = abs(translation)
        
        // Start dragging if downward swipe detected with vertical preference
        if !isDragging && translation > 0 && vertical > horizontal * DragConstants.verticalPreferenceRatio {
            isDragging = true
            maxDragDistance = 0
            hasTriggeredDismissHaptic = false
        }
        
        // Continue tracking if already dragging
        if isDragging {
            if translation > 0 {
                maxDragDistance = max(maxDragDistance, translation)
            }
            
            let dragDistance = max(0, translation)
            #if os(iOS)
            if !hasTriggeredDismissHaptic && dragDistance > DragConstants.dismissalThreshold {
                hasTriggeredDismissHaptic = true
                UIImpactFeedbackGenerator(style: .medium).impactOccurred()
            }
            #endif
            let resistance: CGFloat = dragDistance > DragConstants.dismissalThreshold
                ? DragConstants.rubberBandResistance 
                : 1.0
            let effectiveDrag = dragDistance * resistance
            
            var transaction = Transaction()
            transaction.disablesAnimations = true
            withTransaction(transaction) {
                dragOffset = effectiveDrag
                let progress = min(1.0, Double(abs(effectiveDrag)) / Double(DragConstants.dismissalThreshold))
                dismissOpacity = max(DragConstants.minDismissOpacity, 1.0 - (progress * 0.7))
                backgroundOpacity = max(0.0, 1.0 - progress)
            }
        }
    }
    
    private func handleDragEnded(value: DragGesture.Value, geometry: GeometryProxy) {
        let translation = value.translation.height
        let velocity = value.predictedEndTranslation.height - translation
        
        let dragBackRatio = maxDragDistance > 0 ? translation / maxDragDistance : 1.0
        let cancelledByDragBack = dragBackRatio < DragConstants.dragBackCancelRatio 
            && maxDragDistance > DragConstants.dismissalThreshold
        
        if !cancelledByDragBack && (translation > DragConstants.dismissalThreshold || velocity > DragConstants.dismissalVelocityThreshold) {
            withAnimation(.easeOut(duration: DragConstants.dismissAnimationDuration)) {
                dragOffset = geometry.size.height
                dismissOpacity = 0
                backgroundOpacity = 0
            }
            DispatchQueue.main.asyncAfter(deadline: .now() + DragConstants.dismissAnimationDuration) {
                onClose()
            }
        } else {
            withAnimation(.spring(response: DragConstants.springResponse, dampingFraction: DragConstants.springDamping)) {
                dragOffset = 0
                dismissOpacity = 1.0
                backgroundOpacity = 1.0
                isDragging = false
                maxDragDistance = 0
            }
        }
    }

    // MARK: - Helpers
    
    private var currentItem: MediaItem? {
        guard items.indices.contains(index) else { return nil }
        return items[index]
    }
    
    private func preloadAdjacentImages(currentIndex: Int) {
        let indicesToPreload = [
            currentIndex - 1,
            currentIndex + 1
        ].filter { items.indices.contains($0) }
        
        for idx in indicesToPreload {
            let item = items[idx]
            if case .image = item.type, let url = URL(string: item.url) {
                if preloader.loadedImages[url] == nil && !preloader.loading.contains(url) {
                    preloader.loadImageImmediately(for: url)
                }
            }
        }
    }
}

// MARK: - ZoomableImageView

struct ZoomableImageView: View {
    let image: Image
    
    @State private var scale: CGFloat = 1.0
    @State private var lastScale: CGFloat = 1.0
    @State private var offset: CGSize = .zero
    @State private var lastOffset: CGSize = .zero
    
    private enum ZoomConstants {
        static let minScale: CGFloat = 1.0
        static let maxScale: CGFloat = 4.0
       static let panMinimumDistance: CGFloat = 30
        static let springResponse: Double = 0.3
        static let springDamping: Double = 0.8
    }
    
    var body: some View {
        GeometryReader { geometry in
            ZStack(alignment: .center) {
                Color.black
                
                image
                    .resizable()
                    .scaledToFit()
                    .frame(width: geometry.size.width, height: geometry.size.height)
                    .scaleEffect(scale)
                    .offset(offset)
                    .clipped()
                    .gesture(magnificationGesture)
                    .onTapGesture(count: 2) {
                        toggleZoom()
                    }
                    .gesture(panGesture(minimumDistance: scale > ZoomConstants.minScale ? 0 : ZoomConstants.panMinimumDistance, geometry: geometry))
            }
            .frame(width: geometry.size.width, height: geometry.size.height)
            .onChange(of: geometry.size) { oldSize, newSize in
                if oldSize != newSize {
                    resetZoom()
                }
            }
        }
        .ignoresSafeArea(edges: .all)
    }
    
    private var magnificationGesture: some Gesture {
        MagnificationGesture()
            .onChanged { value in
                let delta = value / lastScale
                lastScale = value
                let newScale = scale * delta
                scale = min(max(newScale, ZoomConstants.minScale), ZoomConstants.maxScale)
            }
            .onEnded { _ in
                lastScale = 1.0
                if scale < ZoomConstants.minScale {
                    resetZoom()
                }
            }
    }
    
    private func panGesture(minimumDistance: CGFloat, geometry: GeometryProxy) -> some Gesture {
        DragGesture(minimumDistance: minimumDistance)
            .onChanged { value in
                guard scale > ZoomConstants.minScale else { return }
                let newOffset = CGSize(
                    width: lastOffset.width + value.translation.width,
                    height: lastOffset.height + value.translation.height
                )
                offset = limitOffset(newOffset, geometry: geometry)
            }
            .onEnded { _ in
                guard scale > ZoomConstants.minScale else { return }
                lastOffset = offset
            }
    }
    
    private func resetZoom() {
        withAnimation(.spring(response: ZoomConstants.springResponse, dampingFraction: ZoomConstants.springDamping)) {
            scale = ZoomConstants.minScale
            offset = .zero
            lastOffset = .zero
        }
    }

    private func toggleZoom() {
        withAnimation(.spring(response: ZoomConstants.springResponse, dampingFraction: ZoomConstants.springDamping)) {
            if scale > ZoomConstants.minScale {
                scale = ZoomConstants.minScale
                offset = .zero
                lastOffset = .zero
            } else {
                scale = 2.0
            }
        }
    }
    
    private func limitOffset(_ offset: CGSize, geometry: GeometryProxy) -> CGSize {
        let maxOffsetX = (geometry.size.width * (scale - 1)) / 2
        let maxOffsetY = (geometry.size.height * (scale - 1)) / 2
        
        return CGSize(
            width: min(max(offset.width, -maxOffsetX), maxOffsetX),
            height: min(max(offset.height, -maxOffsetY), maxOffsetY)
        )
    }
}

