import SwiftUI

/// Helper for calculating connection status color based on network and API state
enum ConnectionStatusHelper {
    static func statusColor(
        isConnected: Bool,
        hasError: Bool,
        isLoading: Bool
    ) -> Color {
        if !isConnected {
            return .red
        } else if hasError {
            return .orange
        } else if isLoading {
            return .yellow
        } else {
            return .green
        }
    }
    
    static func statusText(
        isConnected: Bool,
        hasError: Bool,
        isLoading: Bool
    ) -> String {
        if !isConnected {
            return "Offline"
        } else if hasError {
            return "Error"
        } else if isLoading {
            return "Updating..."
        } else {
            return "Live"
        }
    }
}

