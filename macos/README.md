# AI Usage Monitor para macOS

Cliente SwiftUI de barra de menus para o `usage-agent`. Requer macOS 13 ou superior e um agent configurado em `~/.ai-usage/config.json`.

## Desenvolvimento

```bash
swift run AIUsageMenu
swift test
```

## Gerar o aplicativo

```bash
./scripts/build-app.sh
open "dist/AI Usage Monitor.app"
```

O script compila em Release, cria um bundle local em `dist/` e aplica somente uma assinatura ad-hoc para que o macOS valide a estrutura do app. Também é possível informar outro diretório de saída como primeiro argumento. A build não possui assinatura Developer ID nem notarização.

O miniapp não aparece no Dock. Use o ícone na barra de menus para consultar limites dos providers que os expõem, acompanhar o consumo acumulado do OpenCode, solicitar atualização, iniciar um LaunchAgent já instalado ou abrir as preferências de início automático. O OpenCode informa uso de tokens por período, não limites de sessão ou horários de renovação.
