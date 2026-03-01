import Foundation

/// 🔧 Environment Configuration
///
/// All app configuration in one place - no hardcoded values!
///
/// **How to use:**
/// 1. Edit `.env.local` with your settings
/// 2. Settings are automatically picked up in Debug builds
/// 3. CI/CD sets environment variables in GitHub Secrets
///
/// **Example:**
/// ```swift
/// let url = AppEnvironment.apiBaseURL  // Uses .env.local in debug
/// ```
enum AppEnvironment {
    
    // MARK: - API Configuration
    
    /// Base URL for the LCC API
    /// Set via LCC_API_BASE_URL environment variable
    /// Default: https://lcc.live
    static var apiBaseURL: String {
        if let envValue = ProcessInfo.processInfo.environment["LCC_API_BASE_URL"] {
            return envValue
        }
        
        #if DEBUG
        // In debug, you can override with localhost if needed
        return ProcessInfo.processInfo.environment["USE_LOCALHOST"] == "1" 
            ? "http://localhost:3000" 
            : "https://lcc.live"
        #else
        return "https://lcc.live"
        #endif
    }
    
    /// Metrics endpoint URL
    /// Set via GRAFANA_METRICS_URL environment variable
    /// Default: https://lcc.live/api/metrics
    static var metricsURL: String {
        if let envValue = ProcessInfo.processInfo.environment["GRAFANA_METRICS_URL"] {
            return envValue
        }
        return "\(apiBaseURL)/api/metrics"
    }
    
    // MARK: - App Configuration
    
    /// Whether this is a production build
    static var isProduction: Bool {
        #if DEBUG
        return false
        #else
        return true
        #endif
    }
    
    /// Current app version
    static var appVersion: String {
        Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "unknown"
    }
    
    /// Current build number
    static var buildNumber: String {
        Bundle.main.infoDictionary?["CFBundleVersion"] as? String ?? "unknown"
    }
    
    /// Full version string (e.g., "1.0.0 (123)")
    static var fullVersion: String {
        "\(appVersion) (\(buildNumber))"
    }
    
    /// Bundle identifier
    static var bundleIdentifier: String {
        Bundle.main.bundleIdentifier ?? "lcc.live"
    }
    
    // MARK: - Feature Flags
    
    /// Enable metrics collection
    static var metricsEnabled: Bool {
        #if DEBUG
        // Disable metrics in debug by default to improve performance
        return ProcessInfo.processInfo.environment["METRICS_ENABLED"] == "true"
        #else
        return ProcessInfo.processInfo.environment["METRICS_ENABLED"] != "false"
        #endif
    }
    
    /// Enable debug logging
    static var debugLoggingEnabled: Bool {
        #if DEBUG
        return true
        #else
        return ProcessInfo.processInfo.environment["DEBUG_LOGGING"] == "true"
        #endif
    }
    
    // MARK: - Timeouts & Intervals
    
    /// Network request timeout (seconds)
    static var networkTimeout: TimeInterval {
        if let value = ProcessInfo.processInfo.environment["NETWORK_TIMEOUT"], 
           let timeout = TimeInterval(value) {
            return timeout
        }
        return 30.0
    }
    
    /// Image refresh interval (seconds)
    static var imageRefreshInterval: TimeInterval {
        if let value = ProcessInfo.processInfo.environment["IMAGE_REFRESH_INTERVAL"], 
           let interval = TimeInterval(value) {
            return interval
        }
        // Default to 5 seconds for live updates
        return 5.0
    }
    
    /// API version check interval (seconds)
    static var apiCheckInterval: TimeInterval {
        if let value = ProcessInfo.processInfo.environment["API_CHECK_INTERVAL"], 
           let interval = TimeInterval(value) {
            return interval
        }
        return 5.0
    }
    
    // MARK: - Debug Helpers
    
    /// Print all environment variables (debug only)
    static func printConfiguration() {
        #if DEBUG
        print("""
        ========================================
        🔧 Environment Configuration
        ========================================
        Environment: \(isProduction ? "Production" : "Development")
        Version: \(fullVersion)
        Bundle ID: \(bundleIdentifier)
        
        API Base URL: \(apiBaseURL)
        Metrics URL: \(metricsURL)
        Metrics Enabled: \(metricsEnabled)
        
        Network Timeout: \(networkTimeout)s
        Image Refresh: \(imageRefreshInterval)s
        API Check Interval: \(apiCheckInterval)s
        
        Debug Logging: \(debugLoggingEnabled)
        ========================================
        """)
        #endif
    }
}

