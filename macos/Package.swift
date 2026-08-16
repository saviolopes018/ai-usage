// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "AIUsageMenu",
    platforms: [.macOS(.v13)],
    products: [.executable(name: "AIUsageMenu", targets: ["AIUsageMenu"])],
    targets: [
        .executableTarget(
            name: "AIUsageMenu",
            resources: [.process("Resources")]
        ),
        .testTarget(name: "AIUsageMenuTests", dependencies: ["AIUsageMenu"]),
    ]
)
