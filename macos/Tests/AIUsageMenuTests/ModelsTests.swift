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

    func testCombinedTokenTotalUsesSelectedPeriod() throws {
        let data = Data(#"{"protocolVersion":1,"agentVersion":"1.5.0","capabilities":["token-usage-periods"],"device":"Mac","online":true,"updatedAt":"2026-08-25T12:00:00Z","providers":[{"provider":"codex","available":true,"observedAt":"2026-08-25T12:00:00Z","tokens":{"inputTokens":10,"outputTokens":2,"cachedInputTokens":0,"totalTokens":12,"periods":{"24h":{"inputTokens":10,"outputTokens":2,"cachedInputTokens":0,"totalTokens":12},"7d":{"inputTokens":30,"outputTokens":5,"cachedInputTokens":0,"totalTokens":35},"14d":{"inputTokens":50,"outputTokens":8,"cachedInputTokens":0,"totalTokens":58},"30d":{"inputTokens":70,"outputTokens":10,"cachedInputTokens":0,"totalTokens":80}}}},{"provider":"claude","available":true,"observedAt":"2026-08-25T12:00:00Z","tokens":{"inputTokens":20,"outputTokens":3,"cachedInputTokens":0,"totalTokens":23,"periods":{"24h":{"inputTokens":20,"outputTokens":3,"cachedInputTokens":0,"totalTokens":23},"7d":{"inputTokens":40,"outputTokens":7,"cachedInputTokens":0,"totalTokens":47},"14d":{"inputTokens":60,"outputTokens":9,"cachedInputTokens":0,"totalTokens":69},"30d":{"inputTokens":90,"outputTokens":12,"cachedInputTokens":0,"totalTokens":102}}}}]}"#.utf8)
        let snapshot = try JSONDecoder.usageDecoder().decode(UsageSnapshot.self, from: data)

        XCTAssertEqual(CombinedTokenUsage.total(for: .day, providers: snapshot.providers), 35)
        XCTAssertEqual(CombinedTokenUsage.total(for: .sevenDays, providers: snapshot.providers), 82)
        XCTAssertEqual(CombinedTokenUsage.total(for: .fourteenDays, providers: snapshot.providers), 127)
        XCTAssertEqual(CombinedTokenUsage.total(for: .thirtyDays, providers: snapshot.providers), 182)
    }

    func testOpenCodeUsesTokenOnlyPresentation() throws {
        let data = Data(#"{"protocolVersion":1,"agentVersion":"1.6.0","capabilities":[],"device":"Mac","online":true,"updatedAt":"2026-08-26T12:00:00Z","providers":[{"provider":"codex","available":true,"observedAt":"2026-08-26T12:00:00Z","tokens":{"inputTokens":10,"outputTokens":2,"cachedInputTokens":0,"totalTokens":12,"periods":{"7d":{"inputTokens":30,"outputTokens":5,"cachedInputTokens":0,"totalTokens":35}}}},{"provider":"opencode","available":true,"observedAt":"2026-08-26T12:00:00Z","tokens":{"inputTokens":20,"outputTokens":3,"cachedInputTokens":0,"totalTokens":23,"periods":{"24h":{"inputTokens":20,"outputTokens":3,"cachedInputTokens":0,"totalTokens":23},"7d":{"inputTokens":40,"outputTokens":7,"cachedInputTokens":0,"totalTokens":47},"14d":{"inputTokens":60,"outputTokens":9,"cachedInputTokens":0,"totalTokens":69},"30d":{"inputTokens":90,"outputTokens":12,"cachedInputTokens":0,"totalTokens":102}}}}]}"#.utf8)
        let snapshot = try JSONDecoder.usageDecoder().decode(UsageSnapshot.self, from: data)
        let openCode = try XCTUnwrap(snapshot.providers.last)

        XCTAssertEqual(openCode.displayName, "OpenCode")
        XCTAssertFalse(openCode.supportsRateLimitWindows)
        XCTAssertEqual(openCode.tokenConsumptionTitle, "Consumo acumulado")
        XCTAssertEqual(openCode.tokenConsumptionDetail, "O OpenCode informa uso de tokens, não limites de sessão.")
        XCTAssertEqual(openCode.tokenConsumptionRows, [
            TokenConsumptionRow(label: "24 horas", totalTokens: 23),
            TokenConsumptionRow(label: "7 dias", totalTokens: 47),
            TokenConsumptionRow(label: "14 dias", totalTokens: 69),
            TokenConsumptionRow(label: "30 dias", totalTokens: 102),
        ])
        XCTAssertEqual(snapshot.providers.first?.tokenConsumptionRows, [])
        XCTAssertNil(snapshot.providers.first?.tokenConsumptionTitle)
        XCTAssertNil(snapshot.providers.first?.tokenConsumptionDetail)
        XCTAssertEqual(CombinedTokenUsage.providerLabel(providers: snapshot.providers), "Codex + OpenCode")
        XCTAssertEqual(CombinedTokenUsage.accessibilityProviderLabel(providers: snapshot.providers), "Codex e OpenCode")
    }
}
