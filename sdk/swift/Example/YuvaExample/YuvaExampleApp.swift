import SwiftUI
import YuvaAnalytics
import YuvaKit

struct ExampleConfig {
    let serverURL: URL
    let channelKey: String
    let identityTokenURL: URL?
    let analyticsKey: String?
    let analyticsRequireConsent: Bool
    let apiKey: String?

    static func load() -> ExampleConfig? {
        guard let url = Bundle.main.url(forResource: "Config", withExtension: "plist"),
              let data = try? Data(contentsOf: url),
              let values = try? PropertyListSerialization.propertyList(from: data, format: nil) as? [String: String],
              let server = values["ServerURL"].flatMap(URL.init(string:)),
              let channel = values["ChannelKey"]
        else { return nil }
        return ExampleConfig(
            serverURL: server, channelKey: channel, identityTokenURL: values["IdentityTokenURL"].flatMap(URL.init(string:)),
            analyticsKey: values["AnalyticsKey"].flatMap { $0.isEmpty ? nil : $0 },
            analyticsRequireConsent: values["AnalyticsConsent"] == "required",
            apiKey: values["APIKey"].flatMap { $0.isEmpty ? nil : $0 })
    }
}

@main
struct YuvaExampleApp: App {
    private let config = ExampleConfig.load()
    @State private var tab = UserDefaults.standard.string(forKey: "YuvaTab") ?? "support"

    init() {
        if let config, let key = config.analyticsKey {
            YuvaAnalytics.configure(
                YuvaAnalyticsConfiguration(
                    serverURL: config.serverURL, key: key, requireConsent: config.analyticsRequireConsent, debug: true,
                    requests: config.apiKey.map {
                        YuvaRequestMonitoring(
                            key: $0, hosts: [config.serverURL.host ?? ""],
                            routes: ["/healthz", "/readyz", "/client/v1/conversations/{id}"], environment: "development")
                    }))
        }
    }

    var body: some Scene {
        WindowGroup {
            if let config {
                TabView(selection: $tab) {
                    ContentView(config: config)
                        .yuvaScreen("Support")
                        .tabItem { Label("Support", systemImage: "bubble.left.and.bubble.right") }
                        .tag("support")
                    AnalyticsView(config: config)
                        .tabItem { Label("Analytics", systemImage: "chart.bar") }
                        .tag("analytics")
                }
            } else {
                ContentUnavailableView(
                    "Missing Config.plist", systemImage: "gearshape",
                    description: Text("Copy Config.example.plist to Config.plist and fill it in."))
            }
        }
    }
}
