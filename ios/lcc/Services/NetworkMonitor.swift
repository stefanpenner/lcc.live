import Foundation
import Network
import Observation

/// Monitors network connectivity and provides real-time status updates
@Observable
class NetworkMonitor {

    // MARK: - Observed Properties

    var isConnected: Bool = true
    var connectionType: ConnectionType = .unknown
    var lastConnectionChange: Date = Date()

    enum ConnectionType {
        case wifi
        case cellular
        case ethernet
        case unknown
        case none

        var description: String {
            switch self {
            case .wifi: return "Wi-Fi"
            case .cellular: return "Cellular"
            case .ethernet: return "Ethernet"
            case .unknown: return "Unknown"
            case .none: return "No Connection"
            }
        }

        var icon: String {
            switch self {
            case .wifi: return "wifi"
            case .cellular: return "antenna.radiowaves.left.and.right"
            case .ethernet: return "cable.connector"
            case .unknown: return "network"
            case .none: return "wifi.slash"
            }
        }
    }

    // MARK: - Private Properties

    @ObservationIgnored private let monitor = NWPathMonitor()
    @ObservationIgnored private let queue = DispatchQueue(label: "NetworkMonitor")
    @ObservationIgnored private let logger = Logger(category: .networking)

    // MARK: - Initialization

    init() {
        startMonitoring()
    }

    deinit {
        stopMonitoring()
    }

    // MARK: - Public Methods

    func startMonitoring() {
        monitor.pathUpdateHandler = { [weak self] path in
            guard let self = self else { return }

            let wasConnected = self.isConnected
            let newConnectionStatus = path.status == .satisfied
            let newConnectionType = self.determineConnectionType(path: path)

            DispatchQueue.main.async {
                self.isConnected = newConnectionStatus
                self.connectionType = newConnectionType

                if wasConnected != newConnectionStatus {
                    self.lastConnectionChange = Date()
                    self.logConnectionChange(wasConnected: wasConnected, isConnected: newConnectionStatus)

                    // Track metrics
                    if newConnectionStatus {
                        MetricsService.shared.track(event: .appForeground, tags: ["reason": "network_connected"])
                    } else {
                        MetricsService.shared.track(event: .appBackground, tags: ["reason": "network_disconnected"])
                    }
                }
            }
        }

        monitor.start(queue: queue)
        logger.info("Network monitoring started")
    }

    func stopMonitoring() {
        monitor.cancel()
        logger.info("Network monitoring stopped")
    }

    // MARK: - Private Methods

    private func determineConnectionType(path: NWPath) -> ConnectionType {
        if path.status != .satisfied {
            return .none
        }

        if path.usesInterfaceType(.wifi) {
            return .wifi
        } else if path.usesInterfaceType(.cellular) {
            return .cellular
        } else if path.usesInterfaceType(.wiredEthernet) {
            return .ethernet
        } else {
            return .unknown
        }
    }

    private func logConnectionChange(wasConnected: Bool, isConnected: Bool) {
        if isConnected {
            logger.info("Network connected (\(connectionType.description))")
        } else {
            logger.warning("Network disconnected")
        }
    }

    // MARK: - Static Shared Instance

    static let shared = NetworkMonitor()
}
