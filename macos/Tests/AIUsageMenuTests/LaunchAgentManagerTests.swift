import Foundation
import XCTest
@testable import AIUsageMenu

private final class MockExecutor: ProcessExecuting {
    var results: [ProcessResult]
    var calls: [(String, [String])] = []

    init(_ results: [ProcessResult]) { self.results = results }
    func run(_ executable: String, arguments: [String]) throws -> ProcessResult {
        calls.append((executable, arguments))
        return results.removeFirst()
    }
}

final class LaunchAgentManagerTests: XCTestCase {
    private var temporaryHome: URL!

    override func setUpWithError() throws {
        temporaryHome = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try FileManager.default.createDirectory(at: temporaryHome.appendingPathComponent(".local/bin"), withIntermediateDirectories: true)
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: temporaryHome)
    }

    func testMissingWhenBinaryIsAbsent() {
        let executor = MockExecutor([])
        let manager = LaunchAgentManager(executor: executor, userID: 501, homeDirectory: temporaryHome)
        XCTAssertEqual(manager.status(), .missing)
        XCTAssertTrue(executor.calls.isEmpty)
    }

    func testDetectsRunningService() throws {
        FileManager.default.createFile(atPath: temporaryHome.appendingPathComponent(".local/bin/usage-agent").path, contents: Data())
        let executor = MockExecutor([ProcessResult(status: 0, output: "state = running")])
        let manager = LaunchAgentManager(executor: executor, userID: 501, homeDirectory: temporaryHome)
        XCTAssertEqual(manager.status(), .running)
    }

    func testStartsInstalledService() throws {
        FileManager.default.createFile(atPath: temporaryHome.appendingPathComponent(".local/bin/usage-agent").path, contents: Data())
        let executor = MockExecutor([
            ProcessResult(status: 0, output: "state = stopped"),
            ProcessResult(status: 0, output: ""),
        ])
        let manager = LaunchAgentManager(executor: executor, userID: 501, homeDirectory: temporaryHome)
        try manager.start()
        XCTAssertEqual(executor.calls.last?.1, ["kickstart", "-k", "gui/501/com.saviolopes.ai-usage-monitor"])
    }

    func testSurfacesKickstartFailure() throws {
        FileManager.default.createFile(atPath: temporaryHome.appendingPathComponent(".local/bin/usage-agent").path, contents: Data())
        let executor = MockExecutor([
            ProcessResult(status: 0, output: "state = stopped"),
            ProcessResult(status: 5, output: "service failed"),
        ])
        let manager = LaunchAgentManager(executor: executor, userID: 501, homeDirectory: temporaryHome)
        XCTAssertThrowsError(try manager.start())
    }
}
