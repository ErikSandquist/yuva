import SwiftUI
import YuvaAnalytics

struct APICallsSection: View {
    let config: ExampleConfig
    @State private var calls: [String] = []
    @State private var queued: Int?
    @State private var running = false
    private static let session = URLSession(configuration: .ephemeral, delegate: YuvaRequestDelegate(), delegateQueue: nil)

    var body: some View {
        Section {
            if config.apiKey == nil {
                Text("Add APIKey to Config.plist to time API calls.").foregroundStyle(.secondary)
            }
            Button("Call the API") { Task { await run() } }
                .disabled(running || config.apiKey == nil)
                .accessibilityIdentifier("example.requests.call")
            ForEach(calls, id: \.self) { line in
                Text(line).font(.caption.monospaced())
            }
            if let queued {
                LabeledContent("Queued", value: "\(queued)")
                    .accessibilityIdentifier("example.requests.queued")
            }
        } header: {
            Text("API calls")
        } footer: {
            Text("Timed by YuvaRequestDelegate; calls with no matching route are skipped.")
        }
    }

    private func run() async {
        running = true
        defer { running = false }
        calls = []
        let paths = ["/healthz", "/readyz", "/client/v1/conversations/\(UUID().uuidString.lowercased())", "/not-mapped/42"]
        for path in paths {
            let url = config.serverURL.appending(path: path)
            do {
                let (_, response) = try await Self.session.data(from: url)
                calls.append("GET \(path.prefix(32))  \((response as? HTTPURLResponse)?.statusCode ?? 0)")
            } catch {
                calls.append("GET \(path.prefix(32))  \(error.localizedDescription)")
            }
        }
        try? await Task.sleep(for: .milliseconds(300))
        queued = await YuvaAnalytics.queuedRequests()
    }
}
