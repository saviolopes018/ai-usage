import ServiceManagement
import SwiftUI

@MainActor
final class LoginItemModel: ObservableObject {
    @Published var enabled = SMAppService.mainApp.status == .enabled
    @Published var errorMessage: String?

    func setEnabled(_ value: Bool) {
        do {
            if value {
                try SMAppService.mainApp.register()
            } else {
                try SMAppService.mainApp.unregister()
            }
            enabled = SMAppService.mainApp.status == .enabled
            errorMessage = nil
        } catch {
            enabled = SMAppService.mainApp.status == .enabled
            errorMessage = "Não foi possível alterar o início automático."
        }
    }
}

struct SettingsView: View {
    @StateObject private var loginItem = LoginItemModel()

    var body: some View {
        Form {
            Toggle("Abrir ao iniciar a sessão", isOn: Binding(
                get: { loginItem.enabled },
                set: { loginItem.setEnabled($0) }
            ))
            Text("O app se conecta somente ao usage-agent local em 127.0.0.1.")
                .font(.caption)
                .foregroundStyle(.secondary)
            if let error = loginItem.errorMessage {
                Label(error, systemImage: "exclamationmark.triangle")
                    .font(.caption)
                    .foregroundStyle(.red)
            }
        }
        .padding(20)
        .frame(width: 390)
    }
}
