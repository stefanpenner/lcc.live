import SwiftUI

/// Detailed connection information popover
struct ConnectionDetailsView: View {
    @Environment(NetworkMonitor.self) var networkMonitor
    @Environment(APIService.self) var apiService
    @Environment(ImagePreloader.self) var preloader
    @Environment(\.dismiss) private var dismiss
    
    var body: some View {
        VStack(alignment: .leading, spacing: 16) {
            // Header
            HStack {
                Text("Connection Status")
                    .font(.headline)
                Spacer()
                Button("Done") {
                    dismiss()
                }
            }
            .padding(.bottom, 4)
            
            Divider()
            
            // Network status
            StatusRow(
                icon: networkMonitor.connectionType.icon,
                label: "Network",
                value: networkMonitor.isConnected ? networkMonitor.connectionType.description : "No Connection",
                color: networkMonitor.isConnected ? .green : .red
            )
            
            // API status
            StatusRow(
                icon: "server.rack",
                label: "API",
                value: apiService.error == nil ? "Connected" : "Error",
                color: apiService.error == nil ? .green : .orange
            )
            
            // Media counts
            StatusRow(
                icon: "photo.stack",
                label: "LCC Media",
                value: "\(apiService.lccMedia.count) items",
                color: .blue
            )
            
            StatusRow(
                icon: "photo.stack",
                label: "BCC Media",
                value: "\(apiService.bccMedia.count) items",
                color: .blue
            )
            
            // Last refresh
            StatusRow(
                icon: "clock.arrow.circlepath",
                label: "Last Updated",
                value: timeAgoString(from: preloader.lastRefreshed),
                color: .secondary
            )
            
            if let error = apiService.error {
                Divider()
                
                VStack(alignment: .leading, spacing: 4) {
                    Label("Error Details", systemImage: "exclamationmark.triangle.fill")
                        .font(.subheadline.bold())
                        .foregroundColor(.orange)
                    
                    Text(error.localizedDescription)
                        .font(.caption)
                        .foregroundColor(.secondary)
                        .fixedSize(horizontal: false, vertical: true)
                }
            }
            
            Divider()
            
            // App info
            VStack(alignment: .leading, spacing: 4) {
                Text("App Version")
                    .font(.caption.bold())
                    .foregroundColor(.secondary)
                Text(AppEnvironment.fullVersion)
                    .font(.caption)
                    .foregroundColor(.secondary)
            }
        }
        .padding(20)
        .frame(width: 320)
        .background(
            ZStack {
                // Liquid Glass popover background
                RoundedRectangle(cornerRadius: 20)
                    .fill(.regularMaterial)
                
                // Subtle refraction
                RoundedRectangle(cornerRadius: 20)
                    .fill(
                        LinearGradient(
                            colors: [
                                Color.white.opacity(0.08),
                                Color.clear
                            ],
                            startPoint: .topLeading,
                            endPoint: .bottomTrailing
                        )
                    )
            }
        )
    }
    
    private func timeAgoString(from date: Date) -> String {
        let seconds = Int(Date().timeIntervalSince(date))
        if seconds < 5 {
            return "Just now"
        } else if seconds < 60 {
            return "\(seconds)s ago"
        } else if seconds < 3600 {
            return "\(seconds / 60)m ago"
        } else {
            return "\(seconds / 3600)h ago"
        }
    }
}

/// Row for status information
struct StatusRow: View {
    let icon: String
    let label: String
    let value: String
    let color: Color
    
    var body: some View {
        HStack {
            Image(systemName: icon)
                .foregroundColor(color)
                .frame(width: 24)
            
            Text(label)
                .font(.subheadline)
                .foregroundColor(.primary)
            
            Spacer()
            
            Text(value)
                .font(.subheadline.monospacedDigit())
                .foregroundColor(.secondary)
        }
    }
}

#Preview {
    ConnectionDetailsView()
        .environment(APIService())
        .environment(ImagePreloader())
        .environment(NetworkMonitor())
}

