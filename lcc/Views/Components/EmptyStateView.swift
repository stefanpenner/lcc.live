import SwiftUI

struct EmptyStateView: View {
    var body: some View {
        VStack(spacing: 16) {
            Image(systemName: "video.slash")
                .font(.system(size: 60))
                .foregroundColor(.secondary.opacity(0.5))
            Text("No Streams Available")
                .font(.title3)
                .fontWeight(.medium)
                .foregroundColor(.secondary)
            Text("Pull down to refresh")
                .font(.subheadline)
                .foregroundColor(.secondary.opacity(0.7))
        }
        .accessibilityElement(children: .combine)
        .accessibilityLabel("No streams available. Pull down to refresh.")
    }
}

