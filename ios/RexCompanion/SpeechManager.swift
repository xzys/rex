import Foundation
import Speech
import AVFoundation
import Observation

private let log = Log(category: "speech")

@Observable
@MainActor
final class SpeechManager {
    private let audioEngine = AVAudioEngine()
    private var recognitionRequest: SFSpeechAudioBufferRecognitionRequest?
    private var recognitionTask: SFSpeechRecognitionTask?
    private var speechRecognizer: SFSpeechRecognizer?
    private var lastPartial: String = ""
    private var tapInstalled = false

    private(set) var isRecording = false
    private(set) var isAuthorized = false
    private(set) var isActive = false

    var onPartialResult: ((String) -> Void)?
    var onFinalResult: ((String) -> Void)?

    init() {
        speechRecognizer = SFSpeechRecognizer()
        registerNotifications()
    }

    func requestPermissions() {
        SFSpeechRecognizer.requestAuthorization { [weak self] status in
            Task { @MainActor [weak self] in
                guard let self else { return }
                log.info("authorization: \(String(describing: status))")
                if status == .authorized {
                    self.isAuthorized = true
                }
            }
        }
    }

    // Bring the audio session up and start the engine once. With
    // UIBackgroundModes=audio + an active session, iOS keeps the process
    // running when backgrounded. The engine runs continuously with no tap
    // installed; startRecording() adds the tap on demand.
    func activate() {
        guard !isActive else { return }

        let session = AVAudioSession.sharedInstance()
        do {
            try session.setCategory(
                .playAndRecord,
                mode: .measurement,
                options: [.mixWithOthers, .defaultToSpeaker, .allowBluetooth]
            )
            try session.setActive(true, options: [])
        } catch {
            log.error("audio session activate failed: \(error.localizedDescription)")
            return
        }

        audioEngine.prepare()
        do {
            try audioEngine.start()
        } catch {
            log.error("engine start failed: \(error.localizedDescription)")
            return
        }

        isActive = true
        log.info("activated (session active, engine running)")
    }

    func deactivate() {
        guard isActive else { return }
        if isRecording { stopRecording() }
        if audioEngine.isRunning {
            audioEngine.stop()
        }
        try? AVAudioSession.sharedInstance().setActive(false, options: .notifyOthersOnDeactivation)
        isActive = false
        log.info("deactivated")
    }

    func startRecording() {
        guard !isRecording else { return }
        guard let speechRecognizer, speechRecognizer.isAvailable else {
            log.error("recognizer unavailable")
            return
        }
        if !isActive { activate() }
        guard isActive else {
            log.error("startRecording: session not active")
            return
        }
        if !audioEngine.isRunning {
            log.info("engine not running at startRecording; restarting")
            audioEngine.reset()
            audioEngine.prepare()
            do { try audioEngine.start() } catch {
                log.error("engine restart failed: \(error.localizedDescription)")
                return
            }
        }

        log.info("starting recording")
        recognitionTask?.cancel()
        recognitionTask = nil
        lastPartial = ""

        let request = SFSpeechAudioBufferRecognitionRequest()
        request.shouldReportPartialResults = true
        request.addsPunctuation = true
        request.requiresOnDeviceRecognition = true
        recognitionRequest = request

        installInputTap(for: request)

        recognitionTask = speechRecognizer.recognitionTask(with: request) { [weak self] result, error in
            Task { @MainActor [weak self] in
                guard let self else { return }
                if let result {
                    let text = result.bestTranscription.formattedString
                    if result.isFinal {
                        log.info("final: \(text)")
                        let emitted = text.isEmpty ? self.lastPartial : text
                        self.onFinalResult?(emitted)
                        self.lastPartial = ""
                        self.recognitionTask = nil
                    } else {
                        log.debug("partial: \(text)")
                        self.lastPartial = text
                        self.onPartialResult?(text)
                    }
                }
                if let error {
                    log.error("recognition error: \(error.localizedDescription)")
                    if self.recognitionTask != nil {
                        self.onFinalResult?(self.lastPartial)
                        self.lastPartial = ""
                        self.recognitionTask = nil
                    }
                }
            }
        }

        isRecording = true
    }

    func stopRecording() {
        log.info("stopping recording")
        removeInputTap()
        recognitionRequest?.endAudio()
        recognitionRequest = nil
        isRecording = false
    }

    private func installInputTap(for request: SFSpeechAudioBufferRecognitionRequest) {
        guard !tapInstalled else { return }
        let input = audioEngine.inputNode
        let format = input.outputFormat(forBus: 0)
        input.installTap(onBus: 0, bufferSize: 1024, format: format) { buffer, _ in
            request.append(buffer)
        }
        tapInstalled = true
    }

    private func removeInputTap() {
        guard tapInstalled else { return }
        audioEngine.inputNode.removeTap(onBus: 0)
        tapInstalled = false
    }

    // MARK: - System notifications

    private func registerNotifications() {
        let nc = NotificationCenter.default
        nc.addObserver(
            self,
            selector: #selector(handleInterruption(_:)),
            name: AVAudioSession.interruptionNotification,
            object: AVAudioSession.sharedInstance()
        )
        nc.addObserver(
            self,
            selector: #selector(handleRouteChange(_:)),
            name: AVAudioSession.routeChangeNotification,
            object: AVAudioSession.sharedInstance()
        )
        nc.addObserver(
            self,
            selector: #selector(handleMediaServicesReset(_:)),
            name: AVAudioSession.mediaServicesWereResetNotification,
            object: AVAudioSession.sharedInstance()
        )
    }

    @objc private func handleInterruption(_ notification: Notification) {
        guard let raw = notification.userInfo?[AVAudioSessionInterruptionTypeKey] as? UInt,
              let type = AVAudioSession.InterruptionType(rawValue: raw) else { return }
        Task { @MainActor [weak self] in
            guard let self else { return }
            switch type {
            case .began:
                log.info("interruption began")
                // The system deactivates the session for us. Drop the tap and
                // let the engine stop; don't call setActive(false) ourselves.
                self.removeInputTap()
                if self.recognitionTask != nil {
                    self.onFinalResult?(self.lastPartial)
                    self.lastPartial = ""
                    self.recognitionTask?.cancel()
                    self.recognitionTask = nil
                }
                self.isRecording = false
            case .ended:
                log.info("interruption ended; reactivating")
                do {
                    try AVAudioSession.sharedInstance().setActive(true, options: [])
                } catch {
                    log.error("reactivate failed: \(error.localizedDescription)")
                    return
                }
                if !self.audioEngine.isRunning {
                    self.audioEngine.reset()
                    self.audioEngine.prepare()
                    do { try self.audioEngine.start() } catch {
                        log.error("engine restart after interruption failed: \(error.localizedDescription)")
                    }
                }
            @unknown default:
                break
            }
        }
    }

    @objc private func handleRouteChange(_ notification: Notification) {
        Task { @MainActor [weak self] in
            guard let self else { return }
            if self.isActive && !self.audioEngine.isRunning {
                log.info("route change left engine stopped; restarting")
                self.audioEngine.reset()
                self.audioEngine.prepare()
                do { try self.audioEngine.start() } catch {
                    log.error("engine restart after route change failed: \(error.localizedDescription)")
                }
            }
        }
    }

    @objc private func handleMediaServicesReset(_ notification: Notification) {
        Task { @MainActor [weak self] in
            guard let self else { return }
            log.info("media services reset")
            self.isActive = false
            self.activate()
        }
    }
}
