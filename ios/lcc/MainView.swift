import SwiftUI

struct MainView: View {
    enum Tab: Hashable {
        case lcc, bcc
        
        @ViewBuilder
        var label: some View {
            switch self {
            case .lcc:
                Label("LCC", systemImage: "mountain.2")
            case .bcc:
                Label("BCC", systemImage: "mountain.2.fill")
            }
        }
    }
    
    let mediaItems: (lcc: [MediaItem], bcc: [MediaItem])
    let roadConditions: (lcc: [RoadCondition], bcc: [RoadCondition])
    @Environment(ImagePreloader.self) var preloader
    @Environment(NetworkMonitor.self) var networkMonitor
    @Environment(APIService.self) var apiService
    @Environment(WeatherService.self) var weatherService

    @State private var selectedTab: Tab = .lcc
    @Environment(\.horizontalSizeClass) private var horizontalSizeClass
    @AppStorage("gridMode") private var gridMode: PhotoTabView.GridMode = .compact
    @State private var fullScreenMedia: PresentedMedia? = nil
    @State private var showConnectionDetails = false
    
    private var currentMediaItems: [MediaItem] {
        selectedTab == .lcc ? mediaItems.lcc : mediaItems.bcc
    }
    
    private var statusColor: Color {
        ConnectionStatusHelper.statusColor(
            isConnected: networkMonitor.isConnected,
            hasError: apiService.error != nil,
            isLoading: apiService.isLoading
        )
    }
    
    private func roadConditionsForTab(_ tab: Tab) -> [RoadCondition] {
        tab == .lcc ? roadConditions.lcc : roadConditions.bcc
    }

    @ViewBuilder
    private func tabContent(items: [MediaItem], tab: Tab) -> some View {
        NavigationStack {
            PhotoTabView(
                mediaItems: items,
                roadConditions: roadConditionsForTab(tab),
                gridMode: $gridMode,
                onRequestFullScreen: { media in
                    #if os(iOS)
                    UIImpactFeedbackGenerator(style: .light).impactOccurred()
                    #endif
                    fullScreenMedia = media
                }
            )
            .toolbar {
                ToolbarItemGroup(placement: .bottomBar) {
                    Button {
                        showConnectionDetails = true
                    } label: {
                        Circle()
                            .fill(statusColor)
                            .frame(width: 10, height: 10)
                            .shadow(color: statusColor.opacity(0.6), radius: 4)
                            .padding(8)
                    }
                    .buttonStyle(.plain)

                    Spacer()

                    Button {
                        #if os(iOS)
                        UIImpactFeedbackGenerator(style: .light).impactOccurred()
                        #endif
                        withAnimation {
                            gridMode = gridMode == .single ? .compact : .single
                        }
                    } label: {
                        Image(systemName: gridMode == .single ? "square.grid.2x2" : "rectangle.fill")
                    }
                }
            }
        }
        .tag(tab)
        .tabItem { tab.label }
    }

    private var popoverAnchorPoint: UnitPoint {
        // In landscape (regular width), position further from edge to account for wider screen
        // In portrait (compact width), keep closer to left edge
        let xOffset = horizontalSizeClass == .regular ? 0.08 : 0.1
        return UnitPoint(x: xOffset, y: 0.95)
    }
    
    var body: some View {
        TabView(selection: $selectedTab) {
            tabContent(items: mediaItems.lcc, tab: .lcc)
            tabContent(items: mediaItems.bcc, tab: .bcc)
        }
        .background(Color.black.ignoresSafeArea(.all))
        .popover(isPresented: $showConnectionDetails, attachmentAnchor: .point(popoverAnchorPoint), arrowEdge: .bottom) {
            ConnectionDetailsView()
                .presentationCompactAdaptation(.popover)
        }
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
            .frame(maxWidth: .infinity, maxHeight: 140)
            .ignoresSafeArea(edges: [.top, .leading, .trailing])
            .allowsHitTesting(false)
        }
        .fullScreenCover(item: $fullScreenMedia) { presented in
            GalleryFullScreenView(
                items: currentMediaItems,
                initialIndex: currentMediaItems.firstIndex(where: { $0.url == presented.mediaItem.url }) ?? 0,
                onClose: { fullScreenMedia = nil }
            )
            .environment(preloader)
            .environment(weatherService)
        }
        .onChange(of: fullScreenMedia) { _, newValue in
            // Close popover when navigating to full screen
            if newValue != nil {
                showConnectionDetails = false
            }
        }
        .onChange(of: selectedTab) { _, _ in
            // Close popover when switching tabs to prevent crashes
            showConnectionDetails = false
        }
    }
}

struct MainViewPreview : View {
    var body: some View {
        let lccUrls = [
            "https://lcc.live/image/aHR0cHM6Ly9iMTAuaGRyZWxheS5jb20vY2FtZXJhLzg2MTFlMjc2LTdlZTUtNDJjMC1iOGNkLWQ5ZTE4OTBlMWNkNC9zbmFwc2hvdA==",
            "https://lcc.live/image/aHR0cHM6Ly91ZG90dHJhZmZpYy51dGFoLmdvdi8xX2RldmljZXMvYXV4MTQ2MDQuanBlZw==",
            "https://youtube.com/embed/dQw4w9WgXcQ",
            "https://lcc.live/image/aHR0cHM6Ly91ZG90dHJhZmZpYy51dGFoLmdvdi8xX2RldmljZXMvYXV4MTY2NDcuanBlZw==",
            "https://lcc.live/image/aHR0cHM6Ly91ZG90dHJhZmZpYy51dGFoLmdvdi8xX2RldmljZXMvYXV4MTYyNjUuanBlZw==",
            "https://lcc.live/image/aHR0cHM6Ly91ZG90dHJhZmZpYy51dGFoLmdvdi8xX2RldmljZXMvYXV4MTYyNjYuanBlZw==",
            "https://lcc.live/image/aHR0cHM6Ly91ZG90dHJhZmZpYy51dGFoLmdvdi8xX2RldmljZXMvYXV4MTYyNjguanBlZw==",
            "https://lcc.live/image/aHR0cHM6Ly91ZG90dHJhZmZpYy51dGFoLmdvdi8xX2RldmljZXMvYXV4MTYyNjkuanBlZw=="
        ]
        let bccUrls = [
            "https://lcc.live/image/aHR0cHM6Ly91ZG90dHJhZmZpYy51dGFoLmdvdi8xX2RldmljZXMvYXV4MTQ2MDUuanBlZw==",
            "https://lcc.live/image/aHR0cHM6Ly91ZG90dHJhZmZpYy51dGFoLmdvdi8xX2RldmljZXMvYXV4MTYyMTIuanBlZw==",
            "https://lcc.live/image/aHR0cHM6Ly91ZG90dHJhZmZpYy51dGFoLmdvdi8xX2RldmljZXMvYXV4MTYyMTMuanBlZw==",
            "https://lcc.live/image/aHR0cHM6Ly91ZG90dHJhZmZpYy51dGFoLmdvdi8xX2RldmljZXMvYXV4MTYyMTUuanBlZw==",
            "https://lcc.live/image/aHR0cHM6Ly91ZG90dHJhZmZpYy51dGFoLmdvdi8xX2RldmljZXMvYXV4MTYyMTYuanBlZw==",
        ]
        
        MainView(
            mediaItems: (
                lcc: lccUrls.compactMap { MediaItem.from(urlString: $0) },
                bcc: bccUrls.compactMap { MediaItem.from(urlString: $0) }
            ),
            roadConditions: (lcc: [], bcc: [])
        )
        .environment(ImagePreloader())
        .environment(NetworkMonitor.shared)
        .environment(APIService())
        .environment(WeatherService())
    }
}

#Preview {
    MainViewPreview()
}

