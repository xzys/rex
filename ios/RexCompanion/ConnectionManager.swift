import Foundation
import Observation

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
    var ipAddress: String = "192.168.1.100"

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

        connectionState = .connecting
        let urlString = "ws://\(ipAddress):\(port)/"
        print("[rex:ws] connecting to \(urlString)")
        guard let url = URL(string: urlString) else {
            print("[rex:ws] invalid URL: \(urlString)")
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
                    print("[rex:ws] ping failed: \(error.localizedDescription)")
                    self.handleDisconnect()
                } else {
                    print("[rex:ws] connected")
                    self.connectionState = .connected
                }
            }
        }
    }

    func disconnect() {
        print("[rex:ws] disconnecting")
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
            print("[rex:ws] encode error: \(error.localizedDescription)")
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
                case .failure:
                    self.handleDisconnect()
                }
            }
        }
    }

    private func handleMessage(_ text: String) {
        guard let data = text.data(using: .utf8),
              let message = try? JSONDecoder().decode(VoiceMessage.self, from: data) else {
            print("[rex:ws] failed to decode: \(text)")
            return
        }

        print("[rex:ws] received: \(message.type)")
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
        print("[rex:ws] disconnected, will reconnect")
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
