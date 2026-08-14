import Foundation

enum AgentClientError: LocalizedError, Equatable {
    case unauthorized
    case server(Int)
    case invalidResponse

    var errorDescription: String? {
        switch self {
        case .unauthorized: return "O agent recusou a credencial local."
        case .server(let status): return "O agent respondeu com HTTP \(status)."
        case .invalidResponse: return "O agent retornou uma resposta inválida."
        }
    }
}

struct ConfigurationLoader {
    var homeDirectory: URL = FileManager.default.homeDirectoryForCurrentUser

    var configurationURL: URL {
        homeDirectory.appendingPathComponent(".ai-usage/config.json")
    }

    func load() throws -> AgentConfiguration {
        let data = try Data(contentsOf: configurationURL)
        let config = try JSONDecoder().decode(AgentConfiguration.self, from: data)
        guard config.isValid else { throw AgentClientError.invalidResponse }
        return config
    }
}

final class AgentClient: NSObject, URLSessionWebSocketDelegate {
    private var session: URLSession!
    private var socket: URLSessionWebSocketTask?
    private let decoder: JSONDecoder
    private var continuation: AsyncThrowingStream<UsageSnapshot, Error>.Continuation?

    init(configuration: URLSessionConfiguration = .ephemeral) {
        decoder = .usageDecoder()
        super.init()
        session = URLSession(configuration: configuration, delegate: self, delegateQueue: nil)
    }

    func fetchState(configuration: AgentConfiguration) async throws -> UsageSnapshot {
        var request = URLRequest(url: configuration.baseURL.appendingPathComponent("state"))
        request.timeoutInterval = 4
        request.setValue("Bearer \(configuration.token)", forHTTPHeaderField: "Authorization")
        let (data, response) = try await session.data(for: request)
        guard let http = response as? HTTPURLResponse else { throw AgentClientError.invalidResponse }
        if http.statusCode == 401 { throw AgentClientError.unauthorized }
        guard http.statusCode == 200 else { throw AgentClientError.server(http.statusCode) }
        return try decoder.decode(UsageSnapshot.self, from: data)
    }

    func refresh(provider: String, configuration: AgentConfiguration) async throws {
        var request = URLRequest(url: configuration.baseURL.appendingPathComponent("\(provider)/refresh"))
        request.httpMethod = "POST"
        request.timeoutInterval = 18
        request.setValue("Bearer \(configuration.token)", forHTTPHeaderField: "Authorization")
        let (_, response) = try await session.data(for: request)
        guard let http = response as? HTTPURLResponse else { throw AgentClientError.invalidResponse }
        if http.statusCode == 401 { throw AgentClientError.unauthorized }
        guard (200...299).contains(http.statusCode) else { throw AgentClientError.server(http.statusCode) }
    }

    func snapshots(configuration: AgentConfiguration) -> AsyncThrowingStream<UsageSnapshot, Error> {
        disconnect()
        return AsyncThrowingStream { continuation in
            self.continuation = continuation
            var components = URLComponents(url: configuration.baseURL, resolvingAgainstBaseURL: false)!
            components.scheme = "ws"
            components.path = "/ws"
            components.queryItems = [URLQueryItem(name: "protocol", value: "1")]
            let task = self.session.webSocketTask(
                with: components.url!,
                protocols: ["ai-usage.v1", "auth.\(configuration.token)"]
            )
            self.socket = task
            continuation.onTermination = { [weak self] _ in self?.disconnect() }
            task.resume()
            self.receiveNext(task)
        }
    }

    func disconnect() {
        socket?.cancel(with: .goingAway, reason: nil)
        socket = nil
        continuation = nil
    }

    private func receiveNext(_ task: URLSessionWebSocketTask) {
        task.receive { [weak self] result in
            guard let self, self.socket === task else { return }
            switch result {
            case .success(let message):
                do {
                    let data: Data
                    switch message {
                    case .data(let value): data = value
                    case .string(let value): data = Data(value.utf8)
                    @unknown default: throw AgentClientError.invalidResponse
                    }
                    self.continuation?.yield(try self.decoder.decode(UsageSnapshot.self, from: data))
                    self.receiveNext(task)
                } catch {
                    self.continuation?.finish(throwing: error)
                }
            case .failure(let error):
                self.continuation?.finish(throwing: error)
            }
        }
    }
}
