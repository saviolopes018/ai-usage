import AppKit
import SwiftUI

struct MenuContentView: View {
    @ObservedObject var model: AppModel
    @Environment(\.colorScheme) private var colorScheme

    var body: some View {
        VStack(spacing: 0) {
            connectionHeader

            Divider()

            if let snapshot = model.snapshot {
                VStack(spacing: 0) {
                    CombinedTokensView(providers: snapshot.providers)
                    Divider()
                    ForEach(Array(snapshot.providers.enumerated()), id: \.element.id) { index, provider in
                        ProviderView(
                            provider: provider,
                            connectionState: model.state,
                            isRefreshing: model.refreshingProviders.contains(provider.provider),
                            onRefresh: { model.refresh(provider.provider) }
                        )
                        if index < snapshot.providers.count - 1 { Divider() }
                    }
                }
            } else {
                EmptyStateView(model: model)
                    .frame(minHeight: 180)
            }

            Divider()
            footer
        }
        .frame(width: 360)
        .background(Color(nsColor: .windowBackgroundColor))
        .overlay(alignment: .bottom) {
            if let message = model.transientMessage {
                Text(message)
                    .font(.caption)
                    .padding(.horizontal, 12)
                    .padding(.vertical, 7)
                    .background(.regularMaterial, in: Capsule())
                    .padding(.bottom, 52)
                    .transition(.opacity)
                    .task {
                        try? await Task.sleep(nanoseconds: 2_000_000_000)
                        if model.transientMessage == message { model.transientMessage = nil }
                    }
            }
        }
        .animation(.easeOut(duration: 0.18), value: model.transientMessage)
    }

    private var connectionHeader: some View {
        HStack(spacing: 9) {
            Image(systemName: connectionSymbol)
                .foregroundStyle(connectionColor)
                .symbolRenderingMode(.hierarchical)
                .frame(width: 18)
            VStack(alignment: .leading, spacing: 2) {
                Text(model.state.title)
                    .font(.system(size: 13, weight: .semibold))
                if let snapshot = model.snapshot {
                    Text("\(snapshot.device) · atualizado \(Formatters.observed(snapshot.updatedAt))")
                        .font(.system(size: 11))
                        .foregroundStyle(.secondary)
                        .lineLimit(1)
                } else {
                    Text(connectionDetail)
                        .font(.system(size: 11))
                        .foregroundStyle(.secondary)
                }
            }
            Spacer(minLength: 8)
            if model.state != .connected && model.state != .loading {
                Button {
                    model.connect()
                } label: {
                    Image(systemName: "arrow.clockwise")
                        .frame(width: 28, height: 28)
                }
                .buttonStyle(.borderless)
                .help("Tentar novamente")
                .accessibilityLabel("Tentar conectar novamente")
            }
        }
        .padding(.horizontal, 16)
        .padding(.vertical, 12)
        .accessibilityElement(children: .combine)
    }

    private var footer: some View {
        HStack(spacing: 4) {
            Button {
                NSApp.sendAction(Selector(("showSettingsWindow:")), to: nil, from: nil)
            } label: {
                Label("Preferências", systemImage: "gearshape")
            }
            .buttonStyle(FooterButtonStyle())

            Spacer()

            Button {
                NSApplication.shared.terminate(nil)
            } label: {
                Label("Encerrar", systemImage: "power")
            }
            .buttonStyle(FooterButtonStyle())
        }
        .padding(.horizontal, 8)
        .padding(.vertical, 6)
    }

    private var connectionColor: Color {
        switch model.state {
        case .connected: return .green
        case .loading, .reconnecting: return .orange
        default: return .red
        }
    }

    private var connectionSymbol: String {
        switch model.state {
        case .connected: return "checkmark.circle.fill"
        case .loading, .reconnecting: return "arrow.triangle.2.circlepath.circle.fill"
        default: return "exclamationmark.circle.fill"
        }
    }

    private var connectionDetail: String {
        switch model.state {
        case .configurationMissing: return "O arquivo ~/.ai-usage/config.json não existe."
        case .configurationInvalid: return "Confira token e porta no arquivo de configuração."
        case .agentStopped: return "O serviço local não está respondendo."
        case .unauthorized: return "O token local não foi aceito pelo agent."
        case .failed(let message): return message
        default: return "Consultando o serviço local…"
        }
    }
}

private struct CombinedTokensView: View {
    let providers: [ProviderUsage]
    @State private var selectedPeriod: TokenPeriod = .day

    private var total: Int64? { CombinedTokenUsage.total(for: selectedPeriod, providers: providers) }

    var body: some View {
        if CombinedTokenUsage.total(for: .day, providers: providers) != nil {
            VStack(alignment: .leading, spacing: 3) {
                Text("\(CombinedTokenUsage.providerLabel(providers: providers)) · \(selectedPeriod.description)")
                    .font(.system(size: 11, weight: .medium))
                    .foregroundStyle(.secondary)
                Text(total.map { "\($0.formatted(.number.notation(.compactName))) tokens" } ?? "Sem dados")
                    .font(.system(size: 22, weight: .semibold, design: .rounded).monospacedDigit())
                Picker("Período do consumo de tokens", selection: $selectedPeriod) {
                    ForEach(TokenPeriod.allCases) { period in
                        Text(period.buttonLabel).tag(period)
                    }
                }
                .pickerStyle(.segmented)
                .controlSize(.small)
                .padding(.top, 7)
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(.horizontal, 16)
            .padding(.vertical, 13)
            .accessibilityElement(children: .combine)
            .accessibilityLabel("\(total ?? 0) tokens gastos por \(CombinedTokenUsage.accessibilityProviderLabel(providers: providers)), \(selectedPeriod.description)")
        }
    }
}

private struct ProviderView: View {
    let provider: ProviderUsage
    let connectionState: ConnectionState
    let isRefreshing: Bool
    let onRefresh: () -> Void

    private var stale: Bool {
        connectionState != .connected || Formatters.isStale(provider.observedAt)
    }

    private var providerIcon: NSImage? {
        let url = Bundle.main.url(
            forResource: provider.provider,
            withExtension: "png",
            subdirectory: "Providers"
        ) ?? Bundle.module.url(
            forResource: provider.provider,
            withExtension: "png",
            subdirectory: "Providers"
        )
        guard let url else { return nil }
        return NSImage(contentsOf: url)
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            HStack(spacing: 10) {
                Group {
                    if let providerIcon {
                        Image(nsImage: providerIcon)
                            .resizable()
                            .scaledToFit()
                    } else {
                        Image(systemName: provider.provider == "opencode" ? "terminal.fill" : "questionmark.circle.fill")
                            .symbolRenderingMode(.hierarchical)
                            .foregroundStyle(Color.secondary)
                            .font(.system(size: 18, weight: .medium))
                    }
                }
                .frame(width: 26, height: 26)
                .accessibilityHidden(true)
                VStack(alignment: .leading, spacing: 1) {
                    Text(provider.displayName)
                        .font(.system(size: 15, weight: .semibold))
                    Text(provider.available ? (stale ? "Leitura antiga · \(Formatters.observed(provider.observedAt))" : "Leitura real · \(Formatters.observed(provider.observedAt))") : "Sem leitura recente")
                        .font(.system(size: 11))
                        .foregroundStyle(stale ? Color.orange : Color.secondary)
                }
                Spacer()
                if provider.supportsRateLimitWindows {
                    Button(action: onRefresh) {
                        if isRefreshing {
                            ProgressView().controlSize(.small).frame(width: 54)
                        } else {
                            Text("Atualizar").frame(minWidth: 54)
                        }
                    }
                    .controlSize(.small)
                    .disabled(isRefreshing || connectionState != .connected)
                    .accessibilityLabel("Atualizar limites do \(provider.displayName)")
                }
            }

            if provider.available && provider.supportsRateLimitWindows {
                UsageWindowView(label: "5 horas", window: provider.fiveHour)
                UsageWindowView(label: "Semanal", window: provider.weekly)
            } else {
                HStack(alignment: .top, spacing: 9) {
                    Image(systemName: "info.circle")
                        .foregroundStyle(.secondary)
                    Text(provider.available ? (provider.availableDetail ?? "") : provider.unavailableDetail)
                        .font(.system(size: 12))
                        .foregroundStyle(.secondary)
                        .fixedSize(horizontal: false, vertical: true)
                }
                .padding(11)
                .background(Color(nsColor: .controlBackgroundColor), in: RoundedRectangle(cornerRadius: 9))
            }
        }
        .padding(.horizontal, 16)
        .padding(.vertical, 15)
    }
}

private struct UsageWindowView: View {
    let label: String
    let window: UsageWindow?

    var body: some View {
        VStack(spacing: 7) {
            HStack(alignment: .firstTextBaseline) {
                Text(label).font(.system(size: 12, weight: .medium))
                Spacer()
                if let window {
                    Text("\(Int(window.remaining.rounded()))%")
                        .font(.system(size: 18, weight: .semibold, design: .rounded).monospacedDigit())
                    Text("livre")
                        .font(.system(size: 11, weight: .medium))
                        .foregroundStyle(.secondary)
                } else {
                    Text("Sem dados")
                        .font(.system(size: 12, weight: .medium))
                        .foregroundStyle(.secondary)
                }
            }

            if let window {
                ProgressView(value: window.used, total: 100)
                    .progressViewStyle(.linear)
                    .tint(severityColor(UsageSeverity.from(used: window.used)))
                    .accessibilityLabel("\(label), \(Int(window.used.rounded())) por cento usado")
                HStack {
                    Text(Formatters.reset(window.resetsAt))
                    Spacer()
                    Label("\(Int(window.used.rounded()))% usado", systemImage: severitySymbol(UsageSeverity.from(used: window.used)))
                        .foregroundStyle(severityColor(UsageSeverity.from(used: window.used)))
                }
                .font(.system(size: 10.5, weight: .medium))
            } else {
                ProgressView(value: 0, total: 100)
                    .progressViewStyle(.linear)
                    .tint(.clear)
                Text("Janela ainda não informada pelo provider")
                    .font(.system(size: 10.5))
                    .foregroundStyle(.secondary)
                    .frame(maxWidth: .infinity, alignment: .leading)
            }
        }
        .accessibilityElement(children: .combine)
    }

    private func severityColor(_ severity: UsageSeverity) -> Color {
        switch severity {
        case .normal: return .teal
        case .warning: return .orange
        case .critical: return .red
        }
    }

    private func severitySymbol(_ severity: UsageSeverity) -> String {
        switch severity {
        case .normal: return "checkmark.circle"
        case .warning: return "exclamationmark.triangle"
        case .critical: return "exclamationmark.octagon.fill"
        }
    }
}

private struct EmptyStateView: View {
    @ObservedObject var model: AppModel

    var body: some View {
        VStack(spacing: 12) {
            Image(systemName: model.state == .agentStopped ? "bolt.horizontal.circle" : "chart.bar.xaxis")
                .font(.system(size: 28))
                .foregroundStyle(.secondary)
            Text(emptyTitle)
                .font(.system(size: 14, weight: .semibold))
            Text(emptyDetail)
                .font(.system(size: 12))
                .foregroundStyle(.secondary)
                .multilineTextAlignment(.center)
                .frame(maxWidth: 280)

            if model.state == .agentStopped {
                if model.serviceState == .missing {
                    Button("Copiar comando de instalação") { model.copyInstallCommand() }
                } else {
                    Button("Iniciar agent") { model.startAgent() }
                        .buttonStyle(.borderedProminent)
                }
            }
        }
        .padding(24)
        .accessibilityElement(children: .contain)
    }

    private var emptyTitle: String { model.state.title }
    private var emptyDetail: String {
        switch model.state {
        case .configurationMissing: return "Execute o usage-agent uma vez para criar a configuração local."
        case .configurationInvalid: return "O arquivo local existe, mas token ou porta não são válidos."
        case .agentStopped: return model.serviceState == .missing ? "O binário esperado não foi encontrado em ~/.local/bin." : "O LaunchAgent está instalado e pode ser iniciado agora."
        case .unauthorized: return "Reinicie o app depois de corrigir a credencial local."
        default: return "Aguardando uma resposta do usage-agent."
        }
    }
}

private struct FooterButtonStyle: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .font(.system(size: 12))
            .padding(.horizontal, 8)
            .frame(minHeight: 32)
            .contentShape(Rectangle())
            .foregroundStyle(configuration.isPressed ? .primary : .secondary)
    }
}
