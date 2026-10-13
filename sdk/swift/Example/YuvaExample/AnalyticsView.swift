import SwiftUI
import YuvaAnalytics

struct AnalyticsView: View {
    let config: ExampleConfig
    @State private var optedOut = YuvaAnalytics.isOptedOut
    @State private var consent = false
    @State private var log: [String] = []

    var body: some View {
        NavigationStack {
            Form {
                if config.analyticsKey == nil {
                    Section {
                        Text("Add AnalyticsKey to Config.plist to send analytics.").foregroundStyle(.secondary)
                    }
                }
                Section("Events") {
                    Button("Track Purchase (9.99 EUR)") {
                        YuvaAnalytics.track(
                            "Purchase", props: ["plan": "pro", "seats": 3],
                            revenue: YuvaRevenue(amount: Decimal(string: "9.99")!, currency: "EUR"))
                        note("Purchase")
                    }
                    .accessibilityIdentifier("example.analytics.purchase")
                    Button("Track Signup") {
                        YuvaAnalytics.track("Signup", props: ["source": "example"])
                        note("Signup")
                    }
                    .accessibilityIdentifier("example.analytics.signup")
                    NavigationLink("Open Product Detail") {
                        Text("Product Detail")
                            .font(.title2)
                            .yuvaScreen("Product Detail")
                    }
                    .accessibilityIdentifier("example.analytics.product")
                    Button("Send now") {
                        YuvaAnalytics.flush()
                        note("Send now")
                    }
                    .accessibilityIdentifier("example.analytics.flush")
                }
                Section("Privacy") {
                    if config.analyticsRequireConsent {
                        Toggle("Analytics consent", isOn: $consent)
                            .onChange(of: consent) { _, granted in YuvaAnalytics.consent(granted) }
                            .accessibilityIdentifier("example.analytics.consent")
                    }
                    Toggle("Opt out", isOn: $optedOut)
                        .onChange(of: optedOut) { _, out in
                            if out { YuvaAnalytics.optOut() } else { YuvaAnalytics.optIn() }
                            note(out ? "Opted out" : "Opted in")
                        }
                        .accessibilityIdentifier("example.analytics.optout")
                }
                if !log.isEmpty {
                    Section("Recorded") {
                        ForEach(Array(log.enumerated()), id: \.offset) { _, line in
                            Text(line).font(.caption.monospaced())
                        }
                    }
                }
            }
            .navigationTitle("Analytics")
        }
        .yuvaScreen("Analytics")
    }

    private func note(_ text: String) {
        log.insert("\(Date().formatted(date: .omitted, time: .standard))  \(text)", at: 0)
    }
}
