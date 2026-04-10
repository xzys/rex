import SwiftUI

public struct ContentView: View {
    @State private var manager = WebSocketManager()
    @State private var speech: SpeechManager?

    public var body: some View {
        VStack(alignment: .leading, spacing: 20) {
            HStack(spacing: 12) {
                Toggle("", isOn: $manager.isEnabled)
                    .labelsHidden()
                    .onChange(of: manager.isEnabled) {
                        manager.enabledChanged()
                    }

                Text(manager.connectionState.rawValue)
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
                    .disabled(manager.isEnabled)
                    .onChange(of: manager.ipAddress) {
                        manager.ipChanged()
                    }
            }

            Spacer()
        }
        .padding()
        .onAppear {
            setupCallbacks()
            if manager.isEnabled {
                manager.connect()
            }
        }
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

    public init() {}
}
