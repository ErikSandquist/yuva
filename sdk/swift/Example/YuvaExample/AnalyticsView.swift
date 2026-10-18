import SwiftUI
import YuvaAnalytics

struct AnalyticsView: View {
    let config: ExampleConfig
    @State private var optedOut = YuvaAnalytics.isOptedOut
    @State private var consent = false
    @State private var persistent = YuvaAnalytics.persistentId != nil
    @State private var persistentId = YuvaAnalytics.persistentId
    @State private var token = ""
    @State private var log: [String] = []

    var body: some View {
        NavigationStack {
            Form {
                if config.analyticsKey == nil {
                    Section {
                        Label("Add AnalyticsKey to Config.plist to send analytics.", systemImage: "info.circle")
                            .foregroundStyle(.secondary)
                    }
                }
                Section("Events") {
                    ScreenRow("Track Purchase (9.99 EUR)", systemImage: "cart.fill", tint: .green, opensScreen: false) {
                        YuvaAnalytics.track(
                            "Purchase", props: ["plan": "pro", "seats": 3],
                            revenue: YuvaRevenue(amount: Decimal(string: "9.99")!, currency: "EUR"))
                        note("Purchase")
                    }
                    .accessibilityIdentifier("example.analytics.purchase")
                    ScreenRow("Track Signup", systemImage: "person.crop.circle.badge.plus", tint: .indigo, opensScreen: false) {
                        YuvaAnalytics.track("Signup", props: ["source": "example"])
                        note("Signup")
                    }
                    .accessibilityIdentifier("example.analytics.signup")
                    NavigationLink {
                        Text("Product Detail")
                            .font(.title2)
                            .yuvaScreen("Product Detail")
                    } label: {
                        Label {
                            Text("Open Product Detail")
                        } icon: {
                            SettingsIcon(systemImage: "shippingbox.fill", tint: .brown)
                        }
                    }
                    .accessibilityIdentifier("example.analytics.product")
                    ScreenRow("Send now", systemImage: "arrow.up.circle.fill", tint: .blue, opensScreen: false) {
                        YuvaAnalytics.flush()
                        note("Send now")
                    }
                    .accessibilityIdentifier("example.analytics.flush")
                }
                Section("Privacy") {
                    if config.analyticsRequireConsent {
                        ToggleRow(title: "Analytics consent", isOn: $consent)
                            .onChange(of: consent) { _, granted in
                                if !granted { persistent = false }
                                YuvaAnalytics.consent(granted, persistent: persistent)
                                refreshId()
                            }
                            .accessibilityIdentifier("example.analytics.consent")
                    }
                    ToggleRow(title: "Remember me (persistent)", isOn: $persistent)
                        .onChange(of: persistent) { _, keep in
                            if keep, config.analyticsRequireConsent { consent = true }
                            YuvaAnalytics.consent(keep || consent || !config.analyticsRequireConsent, persistent: keep)
                            note(keep ? "Persistent consent" : "Persistent withdrawn")
                            refreshId()
                        }
                        .accessibilityIdentifier("example.analytics.persistent")
                    if let persistentId {
                        Text(persistentId).font(.caption.monospaced()).foregroundStyle(.secondary)
                            .accessibilityIdentifier("example.analytics.persistentId")
                    }
                    TextField("Identity token", text: $token)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                        .accessibilityIdentifier("example.analytics.token")
                    Button("Identify") {
                        YuvaAnalytics.identify(token)
                        note("Identify")
                        token = ""
                    }
                    .disabled(persistentId == nil || token.isEmpty)
                    .accessibilityIdentifier("example.analytics.identify")
                    ToggleRow(title: "Opt out", isOn: $optedOut)
                        .onChange(of: optedOut) { _, out in
                            if out { YuvaAnalytics.optOut() } else { YuvaAnalytics.optIn() }
                            if out { persistent = false }
                            note(out ? "Opted out" : "Opted in")
                            refreshId()
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

    private func refreshId() {
        Task {
            try? await Task.sleep(for: .milliseconds(200))
            persistentId = YuvaAnalytics.persistentId
        }
    }

    private func note(_ text: String) {
        log.insert("\(Date().formatted(date: .omitted, time: .standard))  \(text)", at: 0)
    }
}

struct ToggleRow: View {
    let title: String
    @Binding var isOn: Bool

    var body: some View {
        Button {
            isOn.toggle()
        } label: {
            HStack {
                Text(title).foregroundStyle(.primary)
                Spacer()
                Toggle(title, isOn: $isOn).labelsHidden().allowsHitTesting(false)
            }
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .accessibilityRepresentation { Toggle(title, isOn: $isOn) }
    }
}
