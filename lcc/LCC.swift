//
//  LCC.swift
//  lcc
//
//  Created by Stefan Penner on 4/30/25.
//

import SwiftUI

@main
struct LCC: App {
    @State private var apiService = APIService()
    @State private var preloader = ImagePreloader()
    @State private var networkMonitor = NetworkMonitor.shared

    init() {
        // Initialize metrics service
        _ = MetricsService.shared

        // Print configuration in debug mode
        AppEnvironment.printConfiguration()

        // Track app launch
        MetricsService.shared.track(
            event: .appLaunch,
            tags: [
                "version": AppEnvironment.appVersion,
                "build": AppEnvironment.buildNumber
            ]
        )

        Logger.app.info("🚀 App launched - Version \(AppEnvironment.fullVersion)")
    }

    var body: some Scene {
        WindowGroup {
            ContentView()
                .environment(apiService)
                .environment(preloader)
                .environment(networkMonitor)
                .background(Color.black.ignoresSafeArea(.all))
        }
    }
}

struct ContentView: View {
    @Environment(APIService.self) var apiService
    @Environment(ImagePreloader.self) var preloader

    var body: some View {
        MainView(
            mediaItems: (
                lcc: apiService.lccMedia,
                bcc: apiService.bccMedia
            )
        )
        .background(Color.black.ignoresSafeArea(.all))
        .onChange(of: apiService.lccMedia) { oldValue, newValue in
            if !newValue.isEmpty {
                preloader.preloadMedia(from: newValue)
            }
        }
        .onChange(of: apiService.bccMedia) { oldValue, newValue in
            if !newValue.isEmpty {
                preloader.preloadMedia(from: newValue)
            }
        }
    }
}
