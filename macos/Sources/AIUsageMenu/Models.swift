import Foundation

extension JSONDecoder {
    static func usageDecoder() -> JSONDecoder {
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .custom { decoder in
            let value = try decoder.singleValueContainer().decode(String.self)
            let fractional = ISO8601DateFormatter()
            fractional.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
            if let date = fractional.date(from: value) { return date }
            let standard = ISO8601DateFormatter()
            standard.formatOptions = [.withInternetDateTime]
            if let date = standard.date(from: value) { return date }
            throw DecodingError.dataCorruptedError(in: try decoder.singleValueContainer(), debugDescription: "Data RFC3339 inválida: \(value)")
        }
        return decoder
    }
}

struct AgentConfiguration: Codable, Equatable, Sendable {
    let token: String
    let port: Int

    var isValid: Bool { !token.isEmpty && (1...65_535).contains(port) }
    var baseURL: URL { URL(string: "http://127.0.0.1:\(port)")! }
}

struct UsageSnapshot: Codable, Equatable, Sendable {
    let protocolVersion: Int
    let agentVersion: String
    let capabilities: [String]
    let device: String
    let online: Bool
    let updatedAt: Date
    let providers: [ProviderUsage]
}

struct ProviderUsage: Codable, Equatable, Identifiable, Sendable {
    let provider: String
    let available: Bool
    let observedAt: Date
    let fiveHour: UsageWindow?
    let weekly: UsageWindow?
    let tokens: TokenUsage?

    var id: String { provider }
    var displayName: String {
        switch provider {
        case "codex": return "Codex"
        case "claude": return "Claude"
        default: return provider.capitalized
        }
    }
}

struct TokenUsage: Codable, Equatable, Sendable {
    let inputTokens: Int64
    let outputTokens: Int64
    let cachedInputTokens: Int64
    let totalTokens: Int64
}

struct UsageWindow: Codable, Equatable, Sendable {
    let usedPercentage: Double
    let remainingPercentage: Double
    let resetsAt: Date

    var used: Double { min(100, max(0, usedPercentage)) }
    var remaining: Double { min(100, max(0, remainingPercentage)) }
}

enum UsageSeverity: Equatable {
    case normal, warning, critical

    static func from(used: Double) -> Self {
        if used >= 90 { return .critical }
        if used >= 75 { return .warning }
        return .normal
    }
}

enum ConnectionState: Equatable {
    case loading
    case connected
    case reconnecting(attempt: Int)
    case configurationMissing
    case configurationInvalid
    case agentStopped
    case unauthorized
    case failed(String)

    var title: String {
        switch self {
        case .loading: return "Conectando…"
        case .connected: return "Conectado"
        case .reconnecting: return "Reconectando…"
        case .configurationMissing: return "Configuração não encontrada"
        case .configurationInvalid: return "Configuração inválida"
        case .agentStopped: return "Agent parado"
        case .unauthorized: return "Acesso recusado"
        case .failed: return "Falha de conexão"
        }
    }
}

enum Formatters {
    static func reset(_ date: Date, now: Date = Date()) -> String {
        if date <= now { return "Renovação pendente" }
        let interval = date.timeIntervalSince(now)
        if interval < 60 * 60 * 24 {
            let formatter = RelativeDateTimeFormatter()
            formatter.unitsStyle = .full
            return "Renova \(formatter.localizedString(for: date, relativeTo: now))"
        }
        let formatter = DateFormatter()
        formatter.locale = Locale(identifier: "pt_BR")
        formatter.setLocalizedDateFormatFromTemplate("d MMM, HH:mm")
        return "Renova em \(formatter.string(from: date))"
    }

    static func observed(_ date: Date, now: Date = Date()) -> String {
        let formatter = RelativeDateTimeFormatter()
        formatter.unitsStyle = .short
        return formatter.localizedString(for: date, relativeTo: now)
    }

    static func isStale(_ date: Date, now: Date = Date()) -> Bool {
        now.timeIntervalSince(date) > 15 * 60
    }
}

enum ReconnectPolicy {
    static func delay(forAttempt attempt: Int) -> TimeInterval {
        min(30, pow(2, Double(max(0, attempt - 1))))
    }
}
