// swift-tools-version: 6.0
import PackageDescription

let package = Package(
    name: "YuvaKit",
    defaultLocalization: "en",
    platforms: [.iOS(.v17), .macOS(.v14)],
    products: [
        .library(name: "YuvaKit", targets: ["YuvaKit"]),
        .library(name: "YuvaAnalytics", targets: ["YuvaAnalytics"]),
    ],
    targets: [
        .target(
            name: "YuvaKit",
            path: "sdk/swift/Sources/YuvaKit",
            resources: [.process("Resources")]
        ),
        .target(
            name: "YuvaAnalytics",
            path: "sdk/swift/Sources/YuvaAnalytics",
            resources: [.copy("PrivacyInfo.xcprivacy")]
        ),
        .testTarget(
            name: "YuvaKitTests",
            dependencies: ["YuvaKit"],
            path: "sdk/swift/Tests/YuvaKitTests"
        ),
        .testTarget(
            name: "YuvaAnalyticsTests",
            dependencies: ["YuvaAnalytics"],
            path: "sdk/swift/Tests/YuvaAnalyticsTests"
        ),
    ]
)
