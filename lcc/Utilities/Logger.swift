import Foundation
import OSLog

/// 📝 Structured Logging
///
/// **Usage:**
/// ```swift
/// // Static loggers (recommended)
/// Logger.app.info("App launched")
/// Logger.networking.error("Network failed")
///
/// // Or create instances
/// let logger = Logger(category: .networking)
/// logger.info("API call succeeded")
/// ```
///
/// **Benefits:**
/// - Viewable in Console.app (filter by subsystem)
/// - Debug logs auto-disabled in production
/// - Proper log levels (debug, info, warning, error, fault)
struct Logger {
    
    /// Log categories for different subsystems
    enum Category: String {
        case app = "app"
        case networking = "networking"
        case ui = "ui"
        case performance = "performance"
        case imageLoading = "imageLoading"
        case metrics = "metrics"
    }
    
    private let logger: os.Logger
    
    init(category: Category) {
        self.logger = os.Logger(subsystem: AppEnvironment.bundleIdentifier, category: category.rawValue)
    }
    
    // MARK: - Logging Methods
    
    func debug(_ message: String, file: String = #file, function: String = #function, line: Int = #line) {
        guard AppEnvironment.debugLoggingEnabled else { return }
        let filename = URL(fileURLWithPath: file).lastPathComponent
        logger.debug("[\(filename):\(line)] \(message)")
    }
    
    func info(_ message: String, file: String = #file, function: String = #function, line: Int = #line) {
        let filename = URL(fileURLWithPath: file).lastPathComponent
        logger.info("[\(filename):\(line)] \(message)")
    }
    
    func warning(_ message: String, file: String = #file, function: String = #function, line: Int = #line) {
        let filename = URL(fileURLWithPath: file).lastPathComponent
        logger.warning("[\(filename):\(line)] ⚠️ \(message)")
    }
    
    func error(_ message: String, error: Error? = nil, file: String = #file, function: String = #function, line: Int = #line) {
        let filename = URL(fileURLWithPath: file).lastPathComponent
        if let error = error {
            logger.error("[\(filename):\(line)] ❌ \(message) - Error: \(error.localizedDescription)")
        } else {
            logger.error("[\(filename):\(line)] ❌ \(message)")
        }
    }
    
    func fault(_ message: String, file: String = #file, function: String = #function, line: Int = #line) {
        let filename = URL(fileURLWithPath: file).lastPathComponent
        logger.fault("[\(filename):\(line)] 🔥 \(message)")
    }
    
    // MARK: - Static Convenience Loggers
    
    static let app = Logger(category: .app)
    static let networking = Logger(category: .networking)
    static let ui = Logger(category: .ui)
    static let performance = Logger(category: .performance)
    static let imageLoading = Logger(category: .imageLoading)
    static let metrics = Logger(category: .metrics)
}
