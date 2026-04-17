import Foundation
import Speech
import AVFoundation
import Observation

private let log = Log(category: "speech")

@Observable
@MainActor
final class SpeechManager {
    private var audioEngine: AVAudioEngine?
    private var silencePlayer: AVAudioPlayerNode?
    private var silenceBuffer: AVAudioPCMBuffer?
    private var recognitionRequest: SFSpeechAudioBufferRecognitionRequest?
    private var recognitionTask: SFSpeechRecognitionTask?
    private var speechRecognizer: SFSpeechRecognizer?
    private var lastPartial: String = ""

    private(set) var isRecording = false
    private(set) var isAuthorized = false
    private(set) var isActive = false

    var onPartialResult: ((String) -> Void)?
    var onFinalResult: ((String) -> Void)?

    init() {
        speechRecognizer = SFSpeechRecognizer()
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

    // Activate the AVAudioSession and start a silent player. With
    // UIBackgroundModes=audio in Info.plist, an app that is actively routing
    // audio stays running when backgrounded. The silent output keeps the
    // engine "in use" without lighting the mic indicator — the indicator only
    // appears when we actually install an input tap during startRecording().
    func activate() {
        guard !isActive else { return }

        let session = AVAudioSession.sharedInstance()
        do {
            try session.setCategory(
                .playAndRecord,
                mode: .measurement,
                options: [.mixWithOthers, .defaultToSpeaker, .allowBluetooth]
            )
            try session.setActive(true, options: .notifyOthersOnDeactivation)
        } catch {
            log.error("audio session activate failed: \(error.localizedDescription)")
            return
        }

        let engine = AVAudioEngine()
        let player = AVAudioPlayerNode()
        engine.attach(player)

        let format = engine.mainMixerNode.outputFormat(forBus: 0)
        engine.connect(player, to: engine.mainMixerNode, format: format)

        // One second of silence, looped. Zero-initialized buffer = silence.
        let frameCount = AVAudioFrameCount(format.sampleRate)
        guard let buffer = AVAudioPCMBuffer(pcmFormat: format, frameCapacity: frameCount) else {
            log.error("failed to allocate silence buffer")
            return
        }
        buffer.frameLength = frameCount

        engine.prepare()
        do {
            try engine.start()
        } catch {
            log.error("engine start failed: \(error.localizedDescription)")
            return
        }

        player.scheduleBuffer(buffer, at: nil, options: .loops)
        player.play()

        self.audioEngine = engine
        self.silencePlayer = player
        self.silenceBuffer = buffer
        self.isActive = true
        log.info("activated (silent keeper running)")
    }

    func deactivate() {
        guard isActive else { return }
        if isRecording { stopRecording() }

        silencePlayer?.stop()
        audioEngine?.stop()
        silencePlayer = nil
        silenceBuffer = nil
        audioEngine = nil
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

        // Ensure the shared engine is running. If activate() was never called
        // (e.g. toggle was already on at launch and we raced), bring it up now.
        if !isActive {
            activate()
        }
        guard let engine = audioEngine else {
            log.error("no audio engine after activate")
            return
        }

        log.info("starting recording")
        recognitionTask?.cancel()
        recognitionTask = nil
        lastPartial = ""

        let request = SFSpeechAudioBufferRecognitionRequest()
        request.shouldReportPartialResults = true
        request.addsPunctuation = true
        recognitionRequest = request

        let inputNode = engine.inputNode
        let recordingFormat = inputNode.outputFormat(forBus: 0)
        inputNode.installTap(onBus: 0, bufferSize: 1024, format: recordingFormat) { buffer, _ in
            request.append(buffer)
        }

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
        // Remove the input tap so the mic indicator goes off between presses,
        // but leave the engine + silent player running so the app stays alive
        // in the background.
        audioEngine?.inputNode.removeTap(onBus: 0)
        recognitionRequest?.endAudio()
        recognitionRequest = nil
        isRecording = false
    }
}
