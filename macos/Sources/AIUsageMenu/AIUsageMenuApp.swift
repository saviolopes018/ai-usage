import SwiftUI

@main
struct AIUsageMenuApp: App {
    @StateObject private var model = AppModel()

    var body: some Scene {
        MenuBarExtra {
            MenuContentView(model: model)
        } label: {
            Image(systemName: menuBarSymbol)
                .accessibilityLabel(menuBarLabel)
        }
        .menuBarExtraStyle(.window)

        Settings {
            SettingsView()
        }
    }

    private var menuBarSymbol: String {
        if model.state != .connected { return "chart.bar.xaxis" }
        return model.hasCriticalUsage ? "exclamationmark.circle.fill" : "chart.bar.fill"
    }

    private var menuBarLabel: String {
        if model.state != .connected { return "AI Usage, desconectado" }
        return model.hasCriticalUsage ? "AI Usage, limite crítico" : "AI Usage"
    }
}
