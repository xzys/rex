import Foundation
import Speech
import AVFoundation
import Observation

private let log = Log(category: "speech")

@Observable
@MainActor
final class SpeechManager {
    private var audioEngine: AVAudioEngine?
    private var recognitionRequest: SFSpeechAudioBufferRecognitionRequest?
    private var recognitionTask: SFSpeechRecognitionTask?
    private var speechRecognizer: SFSpeechRecognizer?
    private(set) var isRecording = false
    private(set) var isAuthorized = false

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

    func startRecording() {
        guard !isRecording else { return }
        guard let speechRecognizer, speechRecognizer.isAvailable else {
            log.error("recognizer unavailable")
            return
        }

        log.info("starting recording")
        // Set up audio session
        let audioSession = AVAudioSession.sharedInstance()
        do {
            try audioSession.setCategory(.record, mode: .measurement, options: .duckOthers)
            try audioSession.setActive(true, options: .notifyOthersOnDeactivation)
        } catch {
            log.error("audio session error: \(error.localizedDescription)")
            return
        }

        // Cancel any existing task
        recognitionTask?.cancel()
        recognitionTask = nil

        let request = SFSpeechAudioBufferRecognitionRequest()
        request.shouldReportPartialResults = true
        request.addsPunctuation = true
        recognitionRequest = request

        let audioEngine = AVAudioEngine()
        self.audioEngine = audioEngine

        let inputNode = audioEngine.inputNode
        let recordingFormat = inputNode.outputFormat(forBus: 0)

        inputNode.installTap(onBus: 0, bufferSize: 1024, format: recordingFormat) { buffer, _ in
            request.append(buffer)
        }

        audioEngine.prepare()
        do {
            try audioEngine.start()
        } catch {
            log.error("audio engine start failed: \(error.localizedDescription)")
            stopRecording()
            return
        }

        recognitionTask = speechRecognizer.recognitionTask(with: request) { [weak self] result, error in
            Task { @MainActor [weak self] in
                guard let self else { return }
                if let result {
                    let text = result.bestTranscription.formattedString
                    if result.isFinal {
                        log.info("final: \(text)")
                        self.onFinalResult?(text)
                    } else {
                        log.debug("partial: \(text)")
                        self.onPartialResult?(text)
                    }
                }
                if let error {
                    log.error("recognition error: \(error.localizedDescription)")
                    self.stopRecording()
                }
            }
        }

        isRecording = true
    }

    func stopRecording() {
        log.info("stopping recording")
        audioEngine?.stop()
        audioEngine?.inputNode.removeTap(onBus: 0)
        audioEngine = nil

        recognitionRequest?.endAudio()
        recognitionRequest = nil

        recognitionTask?.cancel()
        recognitionTask = nil

        isRecording = false
    }
}
