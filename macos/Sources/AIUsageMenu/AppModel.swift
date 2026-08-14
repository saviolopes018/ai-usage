import AppKit
import Foundation

@MainActor
final class AppModel: ObservableObject {
    @Published private(set) var state: ConnectionState = .loading
    @Published private(set) var snapshot: UsageSnapshot?
    @Published private(set) var refreshingProviders: Set<String> = []
    @Published private(set) var serviceState: LaunchAgentState = .missing
    @Published var transientMessage: String?

    private let loader: ConfigurationLoader
    private let client: AgentClient
    private let launchAgent: LaunchAgentManager
    private var configuration: AgentConfiguration?
    private var connectionTask: Task<Void, Never>?
    private var wakeObserver: NSObjectProtocol?
    private var generation = 0

    init(
        loader: ConfigurationLoader = ConfigurationLoader(),
        client: AgentClient = AgentClient(),
        launchAgent: LaunchAgentManager = LaunchAgentManager()
    ) {
        self.loader = loader
        self.client = client
        self.launchAgent = launchAgent
        wakeObserver = NSWorkspace.shared.notificationCenter.addObserver(
            forName: NSWorkspace.didWakeNotification,
            object: nil,
            queue: .main
        ) { [weak self] _ in Task { @MainActor in self?.connect() } }
        Task { [weak self] in self?.start() }
    }

    deinit {
        if let wakeObserver { NSWorkspace.shared.notificationCenter.removeObserver(wakeObserver) }
    }

    func start() { connect() }

    func connect() {
        connectionTask?.cancel()
        client.disconnect()
        generation += 1
        let currentGeneration = generation
        state = snapshot == nil ? .loading : .reconnecting(attempt: 1)
        serviceState = launchAgent.status()

        do {
            configuration = try loader.load()
        } catch CocoaError.fileReadNoSuchFile {
            state = .configurationMissing
            return
        } catch AgentClientError.invalidResponse {
            state = .configurationInvalid
            return
        } catch {
            state = .configurationInvalid
            return
        }

        connectionTask = Task { [weak self] in
            await self?.connectionLoop(generation: currentGeneration)
        }
    }

    private func connectionLoop(generation currentGeneration: Int) async {
        guard let configuration else { return }
        var attempt = 0
        while !Task.isCancelled, generation == currentGeneration {
            do {
                let initial = try await client.fetchState(configuration: configuration)
                guard generation == currentGeneration else { return }
                snapshot = initial
                state = .connected
                serviceState = .running
                attempt = 0
                for try await update in client.snapshots(configuration: configuration) {
                    guard generation == currentGeneration else { return }
                    snapshot = update
                    state = .connected
                }
            } catch AgentClientError.unauthorized {
                state = .unauthorized
                return
            } catch is CancellationError {
                return
            } catch {
                attempt += 1
                serviceState = launchAgent.status()
                state = serviceState == .running ? .reconnecting(attempt: attempt) : .agentStopped
                let delay = ReconnectPolicy.delay(forAttempt: attempt)
                try? await Task.sleep(nanoseconds: UInt64(delay * 1_000_000_000))
            }
        }
    }

    func refresh(_ provider: String) {
        guard let configuration, !refreshingProviders.contains(provider) else { return }
        refreshingProviders.insert(provider)
        Task {
            defer { refreshingProviders.remove(provider) }
            do {
                try await client.refresh(provider: provider, configuration: configuration)
                transientMessage = "Atualização solicitada"
            } catch AgentClientError.unauthorized {
                state = .unauthorized
            } catch {
                transientMessage = "Não foi possível atualizar \(provider == "codex" ? "o Codex" : "o Claude")"
            }
        }
    }

    func startAgent() {
        do {
            try launchAgent.start()
            transientMessage = "Agent iniciado"
            connect()
        } catch {
            serviceState = launchAgent.status()
            transientMessage = serviceState == .missing ? "Instale o serviço pelo Terminal" : "Não foi possível iniciar o agent"
        }
    }

    func copyInstallCommand() {
        NSPasteboard.general.clearContents()
        NSPasteboard.general.setString(launchAgent.installCommand, forType: .string)
        transientMessage = "Comando copiado"
    }

    var hasCriticalUsage: Bool {
        snapshot?.providers.contains { provider in
            [provider.fiveHour, provider.weekly].compactMap { $0 }.contains { $0.used >= 90 }
        } ?? false
    }
}
