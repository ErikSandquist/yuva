import Foundation
import SwiftUI
#if canImport(UIKit)
import UIKit
#elseif canImport(AppKit)
import AppKit
#endif

public struct YuvaAnalyticsConfiguration: Sendable {
    public var serverURL: URL
    public var key: String
    public var appId: String
    public var requireConsent: Bool
    public var debug: Bool
    public var requests: YuvaRequestMonitoring?

    public init(
        serverURL: URL, key: String, appId: String = Bundle.main.bundleIdentifier ?? "app",
        requireConsent: Bool = false, debug: Bool = false, requests: YuvaRequestMonitoring? = nil
    ) {
        self.serverURL = serverURL
        self.key = key
        self.appId = appId
        self.requireConsent = requireConsent
        self.debug = debug
        self.requests = requests
    }
}

public struct YuvaRequestMonitoring: Sendable {
    public var key: String
    public var hosts: [String]
    public var routes: [String]
    public var route: (@Sendable (URLRequest) -> String?)?
    public var sendUnmatchedPaths: Bool
    public var apiVersion: String?
    public var environment: String?

    public init(
        key: String, hosts: [String], routes: [String] = [], route: (@Sendable (URLRequest) -> String?)? = nil,
        sendUnmatchedPaths: Bool = false, apiVersion: String? = nil, environment: String? = nil
    ) {
        self.key = key
        self.hosts = hosts
        self.routes = routes
        self.route = route
        self.sendUnmatchedPaths = sendUnmatchedPaths
        self.apiVersion = apiVersion
        self.environment = environment
    }
}

public final class YuvaRequestDelegate: NSObject, URLSessionTaskDelegate, Sendable {
    override public init() {}

    public func urlSession(_ session: URLSession, task: URLSessionTask, didFinishCollecting metrics: URLSessionTaskMetrics) {
        YuvaAnalytics.recordRequest(task, metrics: metrics)
    }
}

public enum YuvaAnalyticsValue: Sendable, Equatable, Codable, ExpressibleByStringLiteral, ExpressibleByIntegerLiteral,
    ExpressibleByFloatLiteral, ExpressibleByBooleanLiteral
{
    case string(String)
    case int(Int)
    case double(Double)
    case bool(Bool)

    public init(stringLiteral value: String) { self = .string(value) }
    public init(integerLiteral value: Int) { self = .int(value) }
    public init(floatLiteral value: Double) { self = .double(value) }
    public init(booleanLiteral value: Bool) { self = .bool(value) }

    public init(from decoder: any Decoder) throws {
        let container = try decoder.singleValueContainer()
        if let value = try? container.decode(Bool.self) {
            self = .bool(value)
        } else if let value = try? container.decode(Int.self) {
            self = .int(value)
        } else if let value = try? container.decode(Double.self) {
            self = .double(value)
        } else {
            self = .string(try container.decode(String.self))
        }
    }

    public func encode(to encoder: any Encoder) throws {
        var container = encoder.singleValueContainer()
        switch self {
        case .string(let value): try container.encode(value)
        case .int(let value): try container.encode(value)
        case .double(let value): try container.encode(value)
        case .bool(let value): try container.encode(value)
        }
    }
}

public struct YuvaRevenue: Sendable, Equatable {
    public var amount: Decimal
    public var currency: String

    public init(amount: Decimal, currency: String) {
        self.amount = amount
        self.currency = currency
    }
}

public enum YuvaAnalytics {
    static let processKey = UUID().uuidString
    static let optOutKey = "yuva_analytics_optout"
    private static let state = State()

    @MainActor
    public static func configure(_ configuration: YuvaAnalyticsConfiguration) {
        let optedOut = UserDefaults.standard.bool(forKey: optOutKey)
        let app = Device.app()
        let session = URLSession(configuration: .ephemeral)
        let collect = configuration.serverURL.appendingPathComponent("client/v1/analytics")
        let core = AnalyticsCore(
            endpoint: collect.appendingPathComponent(configuration.key),
            processKey: processKey, app: app, home: "app://\(configuration.appId)/",
            requireConsent: configuration.requireConsent, optedOut: optedOut, queueFile: queueFile(key: configuration.key),
            ids: PersistentIDStore(defaults: .standard), session: session,
            debug: configuration.debug)
        let requests = configuration.requests.map {
            RequestCore(
                endpoint: collect.appendingPathComponent($0.key).appendingPathComponent("requests"), app: app,
                requireConsent: configuration.requireConsent, optedOut: optedOut,
                queueFile: queueFile(key: $0.key + "_requests"), session: session, debug: configuration.debug)
        }
        state.install(
            core: core, requests: requests, matcher: configuration.requests.map(RequestMatcher.init),
            appId: configuration.appId)
        Device.observe()
    }

    public static func screen(_ name: String) {
        guard let url = state.screen(name) else { return }
        state.send(.record(QueuedEvent(n: "screen", u: url, t: timestamp())))
    }

    public static func track(
        _ name: String, props: [String: YuvaAnalyticsValue] = [:], revenue: YuvaRevenue? = nil, interactive: Bool = true
    ) {
        guard let url = state.currentScreen else { return }
        state.send(
            .record(
                QueuedEvent(
                    n: name, u: url, t: timestamp(), p: props.isEmpty ? nil : props,
                    revenue: revenue.map { RevenueBody(a: "\($0.amount)", c: $0.currency.uppercased()) },
                    i: interactive ? nil : false)))
    }

    public static func consent(_ granted: Bool, persistent: Bool = false) {
        state.send(.consent(granted, persistent: persistent))
    }

    public static func identify(_ token: String) {
        state.send(.identify(token))
    }

    public static var persistentId: String? {
        guard let value = PersistentIDStore(defaults: .standard).load(), !value.expired(at: Date()) else { return nil }
        return value.id
    }

    public static func optOut() {
        UserDefaults.standard.set(true, forKey: optOutKey)
        state.send(.optOut)
    }

    public static func optIn() {
        UserDefaults.standard.removeObject(forKey: optOutKey)
        state.send(.optIn)
    }

    public static var isOptedOut: Bool {
        UserDefaults.standard.bool(forKey: optOutKey)
    }

    public static func flush() {
        state.send(.flush(nil))
    }

    public static func recordRequest(_ task: URLSessionTask, metrics: URLSessionTaskMetrics) {
        guard let record = state.matcher?.record(task, metrics: metrics) else { return }
        state.send(.request(record))
    }

    public static func queuedRequests() async -> Int {
        await withCheckedContinuation { continuation in
            if !state.send(.queuedRequests { continuation.resume(returning: $0) }, requests: true) {
                continuation.resume(returning: 0)
            }
        }
    }

    static func screenURL(appId: String, name: String) -> String {
        let path = name.trimmingCharacters(in: CharacterSet(charactersIn: "/"))
        var allowed = CharacterSet.urlPathAllowed
        allowed.remove(charactersIn: "?#%")
        return "app://\(appId)/" + (path.addingPercentEncoding(withAllowedCharacters: allowed) ?? "")
    }

    private static func timestamp() -> Int64 {
        Int64(Date().timeIntervalSince1970 * 1000)
    }

    private static func queueFile(key: String) -> URL? {
        guard let support = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask).first else {
            return nil
        }
        var directory = support.appendingPathComponent("YuvaAnalytics", isDirectory: true)
        try? FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        var values = URLResourceValues()
        values.isExcludedFromBackup = true
        try? directory.setResourceValues(values)
        let name = String(key.unicodeScalars.map { CharacterSet.alphanumerics.contains($0) || $0 == "_" ? Character($0) : "-" })
        return directory.appendingPathComponent(name + ".json")
    }

    private final class State: @unchecked Sendable {
        private let lock = NSLock()
        private var installed: AnalyticsCore?
        private var requests: RequestCore?
        private var requestMatcher: RequestMatcher?
        private var continuation: AsyncStream<AnalyticsCommand>.Continuation?
        private var appId = ""
        private var current: String?

        var currentScreen: String? {
            lock.withLock {
                guard installed != nil else { return nil }
                return current ?? "app://\(appId)/"
            }
        }

        var matcher: RequestMatcher? {
            lock.withLock { requestMatcher }
        }

        func install(core: AnalyticsCore, requests: RequestCore?, matcher: RequestMatcher?, appId: String) {
            let (stream, continuation) = AsyncStream<AnalyticsCommand>.makeStream()
            let previous = lock.withLock {
                let previous = self.continuation
                installed = core
                self.requests = requests
                requestMatcher = matcher
                self.continuation = continuation
                self.appId = appId
                return previous
            }
            previous?.finish()
            Task {
                await core.start()
                await requests?.start()
                for await command in stream {
                    if case .flush(let done?) = command, let requests {
                        await requests.handle(.flush(nil))
                        await core.handle(
                            .flush {
                                Task {
                                    await requests.idle()
                                    done()
                                }
                            })
                    } else {
                        await core.handle(command)
                        await requests?.handle(command)
                    }
                }
            }
        }

        func screen(_ name: String) -> String? {
            lock.withLock {
                guard installed != nil else { return nil }
                let url = YuvaAnalytics.screenURL(appId: appId, name: name)
                current = url
                return url
            }
        }

        @discardableResult
        func send(_ command: AnalyticsCommand, requests needed: Bool = false) -> Bool {
            let target = lock.withLock { needed && requests == nil ? nil : continuation }
            guard let target else { return false }
            if case .enqueued = target.yield(command) { return true }
            return false
        }
    }

    @MainActor
    private enum Device {
        static var observing = false

        static func app() -> AppInfo {
            let os = ProcessInfo.processInfo.operatingSystemVersion
            var osVersion = "\(os.majorVersion).\(os.minorVersion)"
            if os.patchVersion > 0 { osVersion += ".\(os.patchVersion)" }
            return AppInfo(
                platform: "ios",
                version: Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String,
                osVersion: osVersion, device: kind(), width: width(),
                language: Bundle.main.preferredLocalizations.first)
        }

        static func kind() -> String {
            #if canImport(UIKit)
            if ProcessInfo.processInfo.isiOSAppOnMac || ProcessInfo.processInfo.isMacCatalystApp { return "desktop" }
            switch UIDevice.current.userInterfaceIdiom {
            case .pad: return "tablet"
            case .mac: return "desktop"
            default: return "mobile"
            }
            #else
            return "desktop"
            #endif
        }

        static func width() -> Int? {
            #if canImport(UIKit)
            let scene = UIApplication.shared.connectedScenes.compactMap { $0 as? UIWindowScene }.first
            return scene.map { Int($0.screen.bounds.width) }
            #elseif canImport(AppKit)
            return NSScreen.main.map { Int($0.frame.width) }
            #else
            return nil
            #endif
        }

        static func observe() {
            #if canImport(UIKit)
            guard !observing else { return }
            observing = true
            let center = NotificationCenter.default
            center.addObserver(forName: UIScene.didActivateNotification, object: nil, queue: .main) { _ in
                MainActor.assumeIsolated {
                    if let width = width() { YuvaAnalytics.state.send(.width(width)) }
                    YuvaAnalytics.flush()
                }
            }
            center.addObserver(forName: UIApplication.didEnterBackgroundNotification, object: nil, queue: .main) { _ in
                MainActor.assumeIsolated { sendInBackground() }
            }
            #endif
        }

        #if canImport(UIKit)
        static func sendInBackground() {
            let task = BackgroundTask()
            task.id = UIApplication.shared.beginBackgroundTask(withName: "YuvaAnalytics") {
                MainActor.assumeIsolated { task.end() }
            }
            YuvaAnalytics.state.send(.flush { Task { @MainActor in task.end() } })
        }
        #endif
    }
}

#if canImport(UIKit)
@MainActor
private final class BackgroundTask {
    var id = UIBackgroundTaskIdentifier.invalid

    func end() {
        guard id != .invalid else { return }
        UIApplication.shared.endBackgroundTask(id)
        id = .invalid
    }
}
#endif

extension View {
    public func yuvaScreen(_ name: String) -> some View {
        onAppear { YuvaAnalytics.screen(name) }
    }
}
