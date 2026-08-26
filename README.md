# AI Usage Monitor

<p align="center">
  <strong>Português</strong> · <a href="README.en.md">English</a>
</p>

Monitor local para acompanhar o consumo de **Codex**, **Claude Code** e **OpenCode** pelo celular, sem enviar tokens ou credenciais dos providers para serviços externos.

O projeto combina um agent em Go executado no macOS com um aplicativo Expo/React Native. O agent consulta as ferramentas locais, publica snapshots autenticados pela rede e o app transforma esses dados em uma leitura rápida de consumo, renovação das janelas, disponibilidade e integridade da conexão.

> O projeto está em desenvolvimento ativo. A integração foi projetada para uso pessoal em uma rede confiável; revise a seção [Segurança](#segurança) antes de expor o agent fora da rede local.

## Destaques

- Monitoramento de janelas de uso do Codex e Claude Code.
- Consumo acumulado de tokens do OpenCode nas últimas 24 horas, 7, 14 e 30 dias.
- Atualizações em tempo real por WebSocket.
- Descoberta automática do Mac por Bonjour/mDNS.
- Pareamento por QR code com ticket de uso único.
- Credencial individual e revogável para cada dispositivo.
- Configuração manual como alternativa à descoberta local.
- Diagnóstico separado para rede, agent, WebSocket e providers.
- Alertas locais de consumo, indisponibilidade, dados antigos e renovação.
- Persistência da última leitura válida sem transformar dados ausentes em `0%`.
- Temas claro e escuro seguindo o sistema.
- Widgets do iOS para tela inicial e tela bloqueada, mostrando o percentual usado.
- Serviço automático no macOS por `launchd`.

## Como funciona

```text
┌──────────────────────────── Mac ────────────────────────────┐
│                                                            │
│  Codex app-server ─┐                                       │
│                    ├─> usage-agent ─> HTTP + WebSocket      │
│  Claude statusLine ┤        │              │                │
│  OpenCode database┘        │              │                │
│                             ├─ cache local  └─ Bonjour/mDNS │
│                             └─ credenciais por dispositivo  │
└─────────────────────────────────────────────┬──────────────┘
                                              │ rede local
                                    ┌─────────▼─────────┐
                                    │ App iOS / Android │
                                    │ + widgets no iOS  │
                                    └───────────────────┘
```

O agent mantém um único snapshot concorrente em memória e só publica mudanças reais. Ao conectar, o cliente recebe imediatamente o estado atual; novas leituras são distribuídas aos clientes WebSocket. Ping/pong remove conexões inativas e `SIGINT`/`SIGTERM` iniciam um encerramento gracioso.

O agent não lê credenciais do Codex ou OpenCode. Para atualizar os limites do Claude, ele lê o token já existente no macOS Keychain apenas em memória e usa o `statusLine`/CLI como fallback. Do OpenCode, somente contadores agregados de tokens e timestamps são consultados no banco local; prompts e respostas não são lidos.

## Componentes

| Componente | Tecnologia | Responsabilidade |
|---|---|---|
| `usage-agent` | Go | Coletar, armazenar e publicar o uso local |
| App móvel | Expo 54, React Native 0.81 | Monitor, diagnóstico, alertas e configuração |
| Widget iOS | SwiftUI e WidgetKit | Resumo do consumo usado na tela inicial/bloqueada |
| Descoberta | Bonjour/mDNS | Encontrar o agent sem fixar o IP do Mac |
| Transporte | HTTP e WebSocket | Snapshot, refresh, diagnóstico e atualizações |

## Miniapp para macOS

O cliente nativo em `macos/` coloca o monitor na barra de menus e usa o `usage-agent` já instalado. Ele lê a configuração local em `~/.ai-usage/config.json`, conecta-se somente a `127.0.0.1` e não mantém outra cópia do token.

Para executar os testes e gerar um `.app` local:

```bash
cd macos
swift test
./scripts/build-app.sh
open "dist/AI Usage Monitor.app"
```

O app requer macOS 13 ou superior. A build local não é assinada nem notarizada; se o Gatekeeper bloquear a primeira abertura, use **Abrir** no menu de contexto do Finder.

## Requisitos

### Agent

- macOS.
- Go 1.26 ou compatível com o `go.mod`.
- Codex CLI instalado e autenticado para monitorar Codex.
- Claude Code instalado e autenticado para monitorar Claude.
- OpenCode CLI ou Desktop instalado para monitorar tokens do OpenCode.

### Aplicativo

- Node.js 24.7.0, definido em [`mobile/.nvmrc`](mobile/.nvmrc).
- npm.
- Para iOS: macOS, Xcode e CocoaPods.
- Para Android: Android Studio/SDK e um dispositivo ou emulador configurado.
- Uma build nativa Release; recursos como Bonjour, notificações e widgets não funcionam integralmente no Expo Go.

### Widget iOS

- iOS 17 ou mais recente para a extensão WidgetKit.
- Conta Apple configurada no Xcode.
- App e extensão assinados com a capability App Groups.

## Instalação rápida

### 1. Clone e compile o agent

```bash
git clone https://github.com/saviolopes/ai-usage-monitor.git
cd ai-usage-monitor
go build -o usage-agent ./agent/cmd/usage-agent
```

Inicie o agent:

```bash
./usage-agent serve
```

Na primeira execução, ele cria:

- `~/.ai-usage/config.json`, com token aleatório de 256 bits e permissão `0600`;
- `~/.ai-usage/`, com permissão `0700`;
- porta padrão `9876`.

Em outro terminal, confira o estado:

```bash
./usage-agent status
```

### 2. Configure o Claude Code

Confirme que o Claude Code CLI está autenticado:

```bash
claude auth status
```

Se `loggedIn` for `false`, autentique-o antes de continuar:

```bash
claude auth login
```

Depois de mover o binário para seu caminho definitivo:

```bash
./usage-agent configure-claude
```

Esse comando preserva as chaves existentes de `~/.claude/settings.json`, cria uma cópia de segurança em `~/.claude/settings.json.ai-usage-backup` e configura o `statusLine` usando o caminho absoluto do agent.

As leituras ativas usam automaticamente a autenticação existente do Claude Code no macOS Keychain. O token não é copiado nem persistido pelo agent. Se o Keychain ou endpoint OAuth estiver indisponível, o agent tenta o comando local do Claude Code. O endpoint OAuth de uso não é uma API pública estável e pode mudar sem aviso.

O collector do Codex não exige configuração adicional: ele inicia `codex app-server --stdio`, negocia a versão instalada e acompanha as atualizações de rate limit.

O collector do OpenCode também não exige configuração. Ele prefere `opencode db --format json` quando a CLI está disponível e usa o banco local `~/.local/share/opencode/opencode.db` em modo somente leitura como fallback para o aplicativo Desktop. O OpenCode não publica cotas ou limites de sessão próprios, como uma janela de 5 horas ou semanal. Por isso, o monitor mostra **consumo acumulado** de tokens por período (24 horas, 7, 14 e 30 dias), e não percentual restante nem horário de renovação.

### 3. Instale as dependências do app

```bash
cd mobile
nvm use
npm install
```

### 4. Pareie o celular

Mantenha o agent em execução e rode, na raiz do projeto:

```bash
./usage-agent pair
```

No aplicativo, escolha a leitura de QR e escaneie o código exibido no terminal. O QR contém um ticket de uso único, expira em cinco minutos e não carrega o token mestre.

Como alternativa, use a configuração manual com:

- endereço do Mac, por exemplo `192.168.1.20:9876`;
- token presente em `~/.ai-usage/config.json`.

O Mac e o celular devem estar na mesma rede e o sistema precisa autorizar o acesso à rede local.

## Executar o aplicativo

### iOS Simulator

```bash
cd mobile
npx expo run:ios --configuration Release
```

### iPhone físico

Conecte o iPhone ao Mac, desbloqueie-o, confirme **Confiar neste computador** e habilite o Modo de Desenvolvedor quando solicitado.

Descubra o identificador do aparelho:

```bash
xcrun xctrace list devices
```

Compile e instale uma build Release, com o JavaScript incorporado ao aplicativo:

```bash
cd mobile
npx expo run:ios --configuration Release --device SEU_UDID --no-bundler
```

Essa build funciona de forma independente depois de instalada: não exige servidor de desenvolvimento nem depende do endereço IP do Mac para carregar a interface. Na primeira build, o Xcode pode demorar porque o React Native é compilado nativamente. Builds seguintes aproveitam o cache incremental.

### Android

Com um emulador aberto ou aparelho autorizado por USB:

```bash
cd mobile
npx expo run:android --variant release
```

## Assinatura e App Group no iOS

O projeto usa os identificadores:

```text
App:       com.saviolopes.aiusagemonitor
Widget:    com.saviolopes.aiusagemonitor.widget
App Group: group.com.saviolopes.aiusagemonitor
```

Para gerar uma build assinada em outra conta Apple, substitua esses identificadores por valores sob seu controle e habilite a capability **App Groups** nos targets do app e do widget. Ambos precisam apontar para o mesmo grupo.

O app compartilha com o widget apenas o snapshot necessário para exibição. Endpoint, token e credenciais não são gravados no App Group.

Depois de instalar uma nova build:

1. Abra o app ao menos uma vez.
2. Receba uma leitura válida.
3. Abra a galeria de widgets do iOS.
4. Procure por **AI Usage**.
5. Escolha o widget compacto, detalhado ou de tela bloqueada.

Se o iOS mantiver uma versão antiga após atualizar a extensão, remova o widget e adicione-o novamente.

## Uso do agent

```text
usage-agent serve
usage-agent status
usage-agent pair
usage-agent devices
usage-agent revoke-device ID
usage-agent configure-claude
usage-agent install-service
usage-agent service-status
usage-agent uninstall-service
```

### Serviço automático no macOS

Para iniciar o agent automaticamente ao entrar no macOS:

```bash
./usage-agent install-service
./usage-agent service-status
```

O instalador:

- copia o binário para `~/.local/bin/usage-agent`;
- registra `~/Library/LaunchAgents/com.saviolopes.ai-usage-monitor.plist`;
- configura `RunAtLoad` e `KeepAlive`;
- integra o statusLine do Claude com o binário instalado;
- mantém logs rotativos em `~/.ai-usage/agent.log`.

Para remover somente o serviço e preservar configuração e caches:

```bash
./usage-agent uninstall-service
```

### Gerenciar dispositivos

```bash
./usage-agent devices
./usage-agent revoke-device ID
```

Cada pareamento recebe uma credencial própria. Depois que existe ao menos uma credencial individual, o token mestre é aceito apenas por processos locais no Mac; clientes LAN precisam usar tokens de dispositivo.

## Dados e estados

O aplicativo trata explicitamente estados que não podem ser confundidos:

- **0% usado:** leitura válida sem consumo.
- **Sem leitura:** a janela ainda não foi informada.
- **Provider indisponível:** a ferramenta local não respondeu ou não está disponível.
- **Dados antigos:** existe uma última leitura válida, mas ela ultrapassou o limite de atualização.
- **Offline:** o app não consegue alcançar o agent.

O agent persiste o último snapshot em `~/.ai-usage/snapshot.json`. A leitura do Claude também usa `~/.ai-usage/claude-usage.json`. Ambos recebem permissão `0600` e preservam o horário original da observação.

## Atualizações e alertas

O app pode solicitar leituras ativas a cada 30 segundos, 1 minuto ou 5 minutos. Pull-to-refresh e o botão de atualização fazem uma leitura completa de Codex e Claude. Os tokens do OpenCode são atualizados pelo agent a cada cinco minutos. O agent lê o token do Claude Code diretamente do macOS Keychain a cada consulta; o segredo não é gravado nos caches ou logs do agent.

Alertas locais podem ser configurados para:

- limites de consumo;
- provider indisponível;
- dados antigos;
- renovação das janelas;
- previsão de esgotamento baseada no histórico recente;
- horário silencioso.

Os resets futuros são agendados localmente pelo sistema para continuar funcionando quando o app está em background.

## API local

A porta padrão é `9876`.

| Método | Endpoint | Autenticação | Uso |
|---|---|---|---|
| `GET` | `/health` | Pública | Saúde do processo |
| `GET` | `/state` | Bearer token | Snapshot atual |
| `GET` | `/ws?protocol=1` | Subprotocolo WebSocket | Atualizações em tempo real |
| `POST` | `/codex/refresh` | Bearer token | Nova leitura do Codex |
| `POST` | `/claude/refresh` | Bearer token | Nova leitura do Claude |
| `GET` | `/auth/info` | Bearer token | Informações da credencial |
| `GET` | `/devices` | Bearer token | Dispositivos pareados |
| `DELETE` | `/devices/{id}` | Bearer token | Revogar dispositivo |

O protocolo atual é negociado explicitamente pelo cliente. Versões incompatíveis recebem HTTP `426`.

Exemplo de diagnóstico local:

```bash
TOKEN=$(sed -n 's/.*"token": "\([^"]*\)".*/\1/p' ~/.ai-usage/config.json)
curl -H "Authorization: Bearer $TOKEN" http://127.0.0.1:9876/state
```

Exemplo resumido do snapshot:

```json
{
  "protocolVersion": 1,
  "agentVersion": "1.1.0",
  "capabilities": [],
  "device": "MacBook",
  "online": true,
  "updatedAt": "2026-08-08T18:00:00Z",
  "providers": [
    {
      "provider": "codex",
      "available": true,
      "observedAt": "2026-08-08T18:00:00Z",
      "weekly": {
        "usedPercentage": 31,
        "remainingPercentage": 69,
        "resetsAt": "2026-08-15T18:00:00Z"
      }
    }
  ]
}
```

## Segurança

- Tokens de provider nunca são enviados ao app.
- O token mestre é gerado com 256 bits aleatórios.
- Arquivos sensíveis locais usam permissões restritas.
- O QR usa ticket único com expiração de cinco minutos.
- Cada celular recebe uma credencial revogável diferente.
- O anúncio mDNS não contém token.
- O endpoint interno do statusLine aceita apenas chamadas diretas de localhost.
- O widget não recebe endpoint, token ou credenciais.
- Logs não incluem o token de autenticação.

HTTP na rede local não oferece criptografia de transporte. Não encaminhe a porta `9876` diretamente para a internet. Para acesso remoto, use uma rede privada/VPN confiável e mantenha credenciais individuais.

## Desenvolvimento e qualidade

Na raiz:

```bash
go test ./...
go vet ./...
```

No aplicativo:

```bash
cd mobile
npm run lint
npm run typecheck
npm test
npx expo-doctor
```

O projeto possui testes para configuração, collectors, cache, mDNS, pareamento, servidor, WebSocket, notificações e estados visuais.

## Estrutura do repositório

```text
agent/
  cmd/usage-agent/       CLI e ciclo de vida do processo
  internal/claude/       statusLine, refresh e cache do Claude
  internal/codex/        app-server, JSON-RPC e rate limits
  internal/opencode/     tokens agregados da CLI ou banco local
  internal/config/       configuração e credenciais
  internal/domain/       modelo do snapshot
  internal/launchd/      serviço automático no macOS
  internal/logging/      logs rotativos
  internal/mdns/         anúncio Bonjour
  internal/pairing/      tickets e QR de pareamento
  internal/server/       HTTP, autenticação e WebSocket
  internal/store/        estado concorrente e subscriptions
mobile/
  src/components/        componentes visuais reutilizáveis
  src/screens/           monitor, diagnóstico, alertas e setup
  ios/AIUsageWidget/     extensão WidgetKit
  App.tsx                navegação e composição principal
docs/                    auditorias e documentação complementar
```

## Solução de problemas

### `No space left on device` durante o build do Xcode

Confirme o espaço disponível:

```bash
df -h /
du -sh ~/Library/Developer/Xcode/DerivedData 2>/dev/null
```

Feche o Xcode e remova apenas caches regeneráveis necessários, como o DerivedData deste projeto, ModuleCache, cache do CocoaPods e cache de pacotes npm. Evite apagar DeviceSupport ou dados de simuladores sem revisar seu conteúdo.

### O iPhone mostra a tela “Development Servers”

- O aparelho está com uma build de desenvolvimento instalada.
- Reinstale usando `npx expo run:ios --configuration Release --device SEU_UDID --no-bundler`.
- A build Release carrega o pacote incorporado e não procura um servidor na porta `8081`.

### O app não encontra o agent

- Confirme `./usage-agent status`.
- Teste `http://IP_DO_MAC:9876/health` pelo celular.
- Autorize Rede Local e Bonjour.
- Verifique se a porta `9876` não está bloqueada.
- Use o QR ou configuração manual para eliminar problemas de descoberta.

### Claude aparece sem leitura

Primeiro, confira a autenticação do Claude Code:

```bash
claude auth status
```

Se `loggedIn` for `false`, execute `claude auth login`. O agent obtém o token OAuth criado pelo próprio Claude Code no macOS Keychain; nenhuma credencial precisa ser adicionada ao código ou aos arquivos de configuração.

Depois, confirme a configuração do `statusLine`:

```bash
./usage-agent configure-claude
```

Reinicie o serviço com `./usage-agent install-service` se o binário foi atualizado. Em seguida, use o botão **Atualizar** no app. Se o endpoint OAuth estiver temporariamente indisponível, abra o Claude Code para permitir uma atualização pelo `statusLine`.

### Widget não atualiza

- Abra o app principal e aguarde uma leitura válida.
- Confirme que app e widget usam o mesmo App Group.
- Remova e adicione novamente o widget depois de instalar uma nova build.

### Assinatura do widget falha

No Xcode, selecione a mesma equipe para os targets do app e da extensão. Confirme que os bundle identifiers são únicos e que os dois perfis incluem o App Group configurado.

## Privacidade

Todos os snapshots, credenciais de dispositivo, preferências e históricos permanecem entre o Mac e o celular configurado. O projeto não inclui backend remoto, analytics ou sincronização em nuvem.

## Roadmap

- Distribuição simplificada de builds assinadas.
- Melhorias de empacotamento e atualização do agent.
- Cobertura adicional de QA visual em tablets e diferentes tamanhos de texto.
- Transporte remoto opcional com criptografia ponta a ponta.

Contribuições e relatos de problemas são bem-vindos por meio das Issues do GitHub. Ao relatar um problema, remova tokens, IPs privados, nomes de dispositivos e trechos sensíveis dos logs.
