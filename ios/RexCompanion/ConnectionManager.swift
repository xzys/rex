import Foundation
import Observation

private let log = Log(category: "connection")

enum ConnectionState: String {
    case disconnected = "Disconnected"
    case connecting = "Connecting..."
    case connected = "Connected"
}

struct VoiceMessage: Codable {
    let type: String
    var text: String?
}

@Observable
@MainActor
final class ConnectionManager {
    var connectionState: ConnectionState = .disconnected
    var isEnabled: Bool = false
    var ipAddress: String = ""

    private var webSocketTask: URLSessionWebSocketTask?
    private var reconnectTask: Task<Void, Never>?
    private let port = 9876

    // Callback when start/stop messages received from server
    var onStart: (() -> Void)?
    var onStop: (() -> Void)?

    init() {
        // Restore persisted state
        if let savedIP = UserDefaults.standard.string(forKey: "ws_ip"), !savedIP.isEmpty {
            ipAddress = savedIP
        }
        isEnabled = UserDefaults.standard.bool(forKey: "ws_enabled")

        #if DEBUG
        // Test hooks: environment overrides applied after persisted state so
        // `xcodebuildmcp device build-and-run --env REX_IP=... REX_ENABLED=1` can
        // drive the app without UI input. Not persisted to UserDefaults.
        let env = ProcessInfo.processInfo.environment
        if let ip = env["REX_IP"]?.trimmingCharacters(in: .whitespacesAndNewlines), !ip.isEmpty {
            ipAddress = ip
            log.info("REX_IP override: \(ip)")
        }
        if let raw = env["REX_ENABLED"] {
            let v = raw.lowercased()
            isEnabled = (v == "1" || v == "true" || v == "yes")
            log.info("REX_ENABLED override: \(self.isEnabled)")
        }
        if env["REX_RESET"] != nil {
            UserDefaults.standard.removeObject(forKey: "ws_ip")
            UserDefaults.standard.removeObject(forKey: "ws_enabled")
            log.info("REX_RESET: cleared persisted ws_ip/ws_enabled")
        }
        #endif
    }

    func enabledChanged() {
        UserDefaults.standard.set(isEnabled, forKey: "ws_enabled")
        if isEnabled {
            connect()
        } else {
            disconnect()
        }
    }

    func ipChanged() {
        UserDefaults.standard.set(ipAddress, forKey: "ws_ip")
    }

    func connect() {
        guard isEnabled else { return }
        disconnect()

        let host = ipAddress.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !host.isEmpty else {
            log.error("connect requested with empty ip address")
            connectionState = .disconnected
            return
        }

        connectionState = .connecting
        let urlString = "ws://\(host):\(port)/"
        log.info("connecting to \(urlString)")
        guard let url = URL(string: urlString) else {
            log.error("invalid URL: \(urlString)")
            connectionState = .disconnected
            return
        }

        let session = URLSession(configuration: .default)
        let task = session.webSocketTask(with: url)
        webSocketTask = task
        task.resume()

        // Start receiving messages
        receiveMessage()

        // Check connection by sending a ping
        task.sendPing { [weak self] error in
            Task { @MainActor in
                guard let self else { return }
                if let error {
                    log.error("ping failed: \(error.localizedDescription)")
                    self.handleDisconnect()
                } else {
                    log.info("connected")
                    self.connectionState = .connected
                }
            }
        }
    }

    func disconnect() {
        log.info("disconnecting")
        reconnectTask?.cancel()
        reconnectTask = nil
        webSocketTask?.cancel(with: .normalClosure, reason: nil)
        webSocketTask = nil
        connectionState = .disconnected
    }

    func send(_ message: VoiceMessage) {
        guard let task = webSocketTask else { return }
        do {
            let data = try JSONEncoder().encode(message)
            guard let string = String(data: data, encoding: .utf8) else { return }
            task.send(.string(string)) { [weak self] error in
                if error != nil {
                    Task { @MainActor in
                        self?.handleDisconnect()
                    }
                }
            }
        } catch {
            log.error("encode error: \(error.localizedDescription)")
        }
    }

    private func receiveMessage() {
        webSocketTask?.receive { [weak self] result in
            Task { @MainActor in
                guard let self else { return }
                switch result {
                case .success(let message):
                    switch message {
                    case .string(let text):
                        self.handleMessage(text)
                    case .data(let data):
                        if let text = String(data: data, encoding: .utf8) {
                            self.handleMessage(text)
                        }
                    @unknown default:
                        break
                    }
                    // Continue receiving
                    self.receiveMessage()
                case .failure(let error):
                    log.error("receive failed: \(error.localizedDescription)")
                    self.handleDisconnect()
                }
            }
        }
    }

    private func handleMessage(_ text: String) {
        guard let data = text.data(using: .utf8),
              let message = try? JSONDecoder().decode(VoiceMessage.self, from: data) else {
            log.error("failed to decode: \(text)")
            return
        }

        log.info("received: \(message.type)")
        switch message.type {
        case "start":
            onStart?()
        case "stop":
            onStop?()
        default:
            break
        }
    }

    private func handleDisconnect() {
        log.info("disconnected, will reconnect")
        webSocketTask = nil
        connectionState = .disconnected
        scheduleReconnect()
    }

    private func scheduleReconnect() {
        guard isEnabled else { return }
        reconnectTask?.cancel()
        reconnectTask = Task {
            try? await Task.sleep(for: .seconds(3))
            guard !Task.isCancelled, isEnabled else { return }
            connect()
        }
    }
}
