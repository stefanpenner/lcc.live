import Foundation
import Network

/// A simple HTTP server for testing that serves fixture content
final class TestHTTPServer {
    private let listener: NWListener
    private var actualPort: UInt16 = 0
    private var isRunning = false
    
    /// Dictionary mapping paths to response data
    private var routes: [String: Data] = [:]
    
    /// Dictionary mapping paths to custom headers
    private var headers: [String: [String: String]] = [:]
    
    /// The base URL where the server is running
    var baseURL: String {
        "http://localhost:\(actualPort)"
    }
    
    /// Initialize the server on an available port
    init(port: UInt16? = nil) throws {
        let parameters = NWParameters.tcp
        parameters.allowLocalEndpointReuse = true
        
        if let port = port {
            actualPort = port
        }
        
        listener = try NWListener(using: parameters)
        listener.newConnectionHandler = { [weak self] connection in
            self?.handleConnection(connection)
        }
        
        listener.stateUpdateHandler = { [weak self] state in
            if case .ready = state {
                if let port = self?.listener.port {
                    self?.actualPort = port.rawValue
                }
            }
        }
    }
    
    /// Register a route with response data
    func registerRoute(path: String, data: Data, headers: [String: String] = [:]) {
        routes[path] = data
        self.headers[path] = headers
    }
    
    /// Register a route with a JSON string
    func registerRoute(path: String, jsonString: String, headers: [String: String] = [:]) {
        let data = jsonString.data(using: .utf8) ?? Data()
        registerRoute(path: path, data: data, headers: headers)
    }
    
    /// Start the server
    func start() async throws {
        guard !isRunning else { return }
        
        listener.start(queue: .global())
        isRunning = true
        
        // Wait for the server to be ready and get the port
        var attempts = 0
        while actualPort == 0 && attempts < 50 {
            try await Task.sleep(for: .milliseconds(100))
            if let port = listener.port {
                actualPort = port.rawValue
                break
            }
            attempts += 1
        }
        
        if actualPort == 0 {
            throw NSError(domain: "TestHTTPServer", code: -1, userInfo: [NSLocalizedDescriptionKey: "Failed to start server"])
        }
    }
    
    /// Stop the server
    func stop() {
        guard isRunning else { return }
        listener.cancel()
        isRunning = false
    }
    
    /// Handle incoming connections
    private func handleConnection(_ connection: NWConnection) {
        connection.start(queue: .global())
        
        connection.receive(minimumIncompleteLength: 1, maximumLength: 8192) { [weak self] data, _, _, error in
            guard let self = self else { return }
            
            if error != nil {
                connection.cancel()
                return
            }
            
            if let data = data, let request = String(data: data, encoding: .utf8) {
                let response = self.handleRequest(request)
                connection.send(content: response.data(using: .utf8), completion: .contentProcessed { _ in
                    connection.cancel()
                })
            }
        }
    }
    
    /// Handle HTTP request and return response
    private func handleRequest(_ request: String) -> String {
        let lines = request.components(separatedBy: "\r\n")
        guard let requestLine = lines.first else {
            return httpResponse(statusCode: 400, body: "Bad Request")
        }
        
        let components = requestLine.components(separatedBy: " ")
        guard components.count >= 2 else {
            return httpResponse(statusCode: 400, body: "Bad Request")
        }
        
        let method = components[0]
        var path = components[1]
        
        // Remove query string if present
        if let queryIndex = path.firstIndex(of: "?") {
            path = String(path[..<queryIndex])
        }
        
        // Handle HEAD requests (for version checking)
        if method == "HEAD" {
            let headers = self.headers[path] ?? [:]
            let etag = headers["ETag"] ?? "\"test-version\""
            return httpResponse(statusCode: 200, body: "", headers: ["ETag": etag], isHead: true)
        }
        
        // Handle GET requests
        if method == "GET" {
            if let data = routes[path] {
                let customHeaders = headers[path] ?? [:]
                let contentType = customHeaders["Content-Type"] ?? "application/json"
                return httpResponse(
                    statusCode: 200,
                    body: String(data: data, encoding: .utf8) ?? "",
                    headers: ["Content-Type": contentType]
                )
            } else {
                return httpResponse(statusCode: 404, body: "Not Found")
            }
        }
        
        return httpResponse(statusCode: 405, body: "Method Not Allowed")
    }
    
    /// Create HTTP response string
    private func httpResponse(
        statusCode: Int,
        body: String,
        headers: [String: String] = [:],
        isHead: Bool = false
    ) -> String {
        let statusText: String
        switch statusCode {
        case 200: statusText = "OK"
        case 404: statusText = "Not Found"
        case 405: statusText = "Method Not Allowed"
        case 400: statusText = "Bad Request"
        default: statusText = "Unknown"
        }
        
        var response = "HTTP/1.1 \(statusCode) \(statusText)\r\n"
        
        // Add custom headers
        for (key, value) in headers {
            response += "\(key): \(value)\r\n"
        }
        
        // Add content length (unless HEAD request)
        if !isHead {
            response += "Content-Length: \(body.utf8.count)\r\n"
        }
        
        response += "\r\n"
        
        // Add body (unless HEAD request)
        if !isHead {
            response += body
        }
        
        return response
    }
    
    deinit {
        stop()
    }
}

