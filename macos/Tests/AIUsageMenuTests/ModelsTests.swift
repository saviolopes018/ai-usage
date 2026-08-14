import XCTest
@testable import AIUsageMenu

final class ModelsTests: XCTestCase {
    func testDecodesCompleteAndPartialSnapshot() throws {
        let data = Data(#"{"protocolVersion":1,"agentVersion":"1.3.0","capabilities":["codex-refresh"],"device":"Mac","online":true,"updatedAt":"2026-08-14T12:00:00Z","providers":[{"provider":"codex","available":true,"observedAt":"2026-08-14T12:00:00Z","fiveHour":{"usedPercentage":30,"remainingPercentage":70,"resetsAt":"2026-08-14T17:00:00Z"}},{"provider":"claude","available":false,"observedAt":"2026-08-14T12:00:00Z"}]}"#.utf8)
        let decoder = JSONDecoder.usageDecoder()

        let snapshot = try decoder.decode(UsageSnapshot.self, from: data)

        XCTAssertEqual(snapshot.providers.count, 2)
        XCTAssertEqual(snapshot.providers[0].fiveHour?.remaining, 70)
        XCTAssertNil(snapshot.providers[0].weekly)
        XCTAssertNil(snapshot.providers[1].fiveHour)
    }

    func testDecodesGoTimestampWithFractionalSeconds() throws {
        let data = Data(#"{"protocolVersion":1,"agentVersion":"1.3.0","capabilities":[],"device":"Mac","online":true,"updatedAt":"2026-08-14T12:00:00.123456789Z","providers":[]}"#.utf8)
        XCTAssertNoThrow(try JSONDecoder.usageDecoder().decode(UsageSnapshot.self, from: data))
    }

    func testSeverityThresholds() {
        XCTAssertEqual(UsageSeverity.from(used: 74.9), .normal)
        XCTAssertEqual(UsageSeverity.from(used: 75), .warning)
        XCTAssertEqual(UsageSeverity.from(used: 90), .critical)
    }

    func testPercentageClampingAndStaleness() {
        let window = UsageWindow(usedPercentage: 125, remainingPercentage: -25, resetsAt: Date())
        XCTAssertEqual(window.used, 100)
        XCTAssertEqual(window.remaining, 0)
        XCTAssertTrue(Formatters.isStale(Date(timeIntervalSince1970: 0), now: Date(timeIntervalSince1970: 901)))
        XCTAssertFalse(Formatters.isStale(Date(timeIntervalSince1970: 1), now: Date(timeIntervalSince1970: 901)))
    }

    func testReconnectBackoffIsCapped() {
        XCTAssertEqual(ReconnectPolicy.delay(forAttempt: 1), 1)
        XCTAssertEqual(ReconnectPolicy.delay(forAttempt: 2), 2)
        XCTAssertEqual(ReconnectPolicy.delay(forAttempt: 6), 30)
        XCTAssertEqual(ReconnectPolicy.delay(forAttempt: 20), 30)
    }

    func testResetFormattingHandlesExpiredWindow() {
        let now = Date(timeIntervalSince1970: 10_000)
        XCTAssertEqual(Formatters.reset(now.addingTimeInterval(-1), now: now), "Renovação pendente")
        XCTAssertTrue(Formatters.reset(now.addingTimeInterval(3600), now: now).hasPrefix("Renova "))
    }
}
