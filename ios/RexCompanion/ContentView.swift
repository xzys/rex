import SwiftUI

private let log = Log(category: "app")

struct ContentView: View {
    @State private var manager = ConnectionManager()
    @State private var speech: SpeechManager?
    @FocusState private var isIPFieldFocused: Bool

    var body: some View {
        ZStack {
            Color.clear
                .contentShape(Rectangle())
                .onTapGesture {
                    isIPFieldFocused = false
                }

            VStack(alignment: .leading, spacing: 20) {
                HStack(spacing: 12) {
                    Toggle("", isOn: $manager.isEnabled)
                        .labelsHidden()
                        .accessibilityIdentifier("enabledToggle")
                        .onChange(of: manager.isEnabled) {
                            manager.enabledChanged()
                        }

                    Text(manager.connectionState.rawValue)
                        .accessibilityIdentifier("connectionStateLabel")
                    Spacer()
                }

                HStack {
                    Text("IP Address")
                    Spacer()
                    TextField("192.168.1.100", text: $manager.ipAddress)
                        .textFieldStyle(.roundedBorder)
                        .keyboardType(.decimalPad)
                        .autocorrectionDisabled()
                        .multilineTextAlignment(.trailing)
                        .font(.system(.body, design: .monospaced))
                        .frame(width: ipFieldWidth)
                        .focused($isIPFieldFocused)
                        .disabled(manager.isEnabled)
                        .accessibilityIdentifier("ipAddressField")
                        .onChange(of: manager.ipAddress) {
                            manager.ipChanged()
                        }
                }

                Spacer()
            }
            .padding()
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
        .onAppear {
            log.info("launched ip=\(manager.ipAddress) enabled=\(manager.isEnabled)")
            setupCallbacks()
            if manager.isEnabled {
                ensureSpeechActive()
                manager.connect()
            }
        }
        .onChange(of: manager.isEnabled) {
            // Keep the audio session / engine lifecycle aligned with the
            // toggle. When enabled, an always-on silent playback keeps the app
            // alive in background (UIBackgroundModes=audio). When disabled we
            // release the session so the mic-in-use state clears.
            if manager.isEnabled {
                ensureSpeechActive()
            } else {
                speech?.deactivate()
            }
        }
    }

    private func ensureSpeechActive() {
        if speech == nil {
            initSpeech()
        }
        speech?.activate()
    }

    private var ipFieldWidth: CGFloat {
        let characterCount = max(manager.ipAddress.count, 13)
        return max(160, CGFloat(characterCount) * 18)
    }

    private func setupCallbacks() {
        // When server sends "start", begin recording
        manager.onStart = {
            if speech == nil {
                initSpeech()
            }
            speech?.startRecording()
        }
        // When server sends "stop", stop recording
        manager.onStop = {
            speech?.stopRecording()
        }
    }

    private func initSpeech() {
        let sm = SpeechManager()
        sm.requestPermissions()
        sm.onPartialResult = { text in
            manager.send(VoiceMessage(type: "partial", text: text))
        }
        sm.onFinalResult = { text in
            manager.send(VoiceMessage(type: "final", text: text))
        }
        speech = sm
    }

}
