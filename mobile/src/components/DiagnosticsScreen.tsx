import { useCallback, useEffect, useMemo, useState } from 'react';
import { AppState, NativeModules, ScrollView, StyleSheet, Text, View } from 'react-native';
import { Theme } from '../theme';
import { ConnectionProfile, ConnectionState, DiscoveredDevice, UsageSnapshot } from '../types';
import { DiscoveryState } from '../useDeviceDiscovery';
import { formatRelative, normalizeAgentURL } from '../utils';
import { ActionButton, ScreenHeading, StatusPill } from './ui';

type ProbeState = 'checking' | 'ok' | 'error' | 'unavailable';

type Props = {
  profile: ConnectionProfile;
  connectionState: ConnectionState;
  snapshot: UsageSnapshot | null;
  message: string | null;
  connectionDetails: { lastError: string | null; lastConnectedAt: string | null; lastSnapshotAt: string | null };
  discovery: { devices: DiscoveredDevice[]; state: DiscoveryState; restart: () => void };
  theme: Theme;
  onManageDevices:()=>void;
};

export function DiagnosticsScreen({ profile, connectionState, snapshot, message, connectionDetails, discovery, theme, onManageDevices }: Props) {
  const [agentProbe, setAgentProbe] = useState<ProbeState>('checking');
  const [metroProbe, setMetroProbe] = useState<ProbeState>('checking');
  const [checkedAt, setCheckedAt] = useState<string | null>(null);
  const metroURL = useMemo(getMetroURL, []);
  const restartDiscovery = discovery.restart;
  const discovered = profile.deviceId ? discovery.devices.find(device => device.id === profile.deviceId) : undefined;

  const runChecks = useCallback(async () => {
	restartDiscovery();
    setAgentProbe('checking');
    setMetroProbe(metroURL ? 'checking' : 'unavailable');
    const agent = fetch(normalizeAgentURL(profile, '/health'), { method: 'GET' })
      .then(response => setAgentProbe(response.ok ? 'ok' : 'error'))
      .catch(() => setAgentProbe('error'));
    const metro = metroURL
      ? fetch(`${metroURL}/status`, { method: 'GET' }).then(async response => {
          const body = await response.text();
          setMetroProbe(response.ok && body.includes('packager-status:running') ? 'ok' : 'error');
        }).catch(() => setMetroProbe('error'))
      : Promise.resolve();
    await Promise.all([agent, metro]);
    setCheckedAt(new Date().toISOString());
  }, [metroURL, profile, restartDiscovery]);

  useEffect(() => {
    void runChecks();
    const subscription = AppState.addEventListener('change', state => {
      if (state === 'active') void runChecks();
    });
    return () => subscription.remove();
  }, [runChecks]);

  const providers = snapshot?.providers ?? [];
  return <View style={[styles.root, { backgroundColor: theme.colors.bg }]}>
    <ScrollView contentContainerStyle={styles.content}>
      <ScreenHeading title="Diagnóstico" description="Saúde da conexão entre este aparelho, a rede local e o agent no Mac." theme={theme} action={<StatusPill label={connectionLabel(connectionState)} tone={connectionState==='connected'?'success':connectionState==='offline'?'error':'warning'} theme={theme}/>}/>

      <Section title="Aplicativo" theme={theme}>
        <DiagnosticRow label="Runtime" value={__DEV__ ? 'Development build' : 'Release build'} state="ok" theme={theme}/>
        <DiagnosticRow label="Metro" value={probeLabel(metroProbe)} detail={metroURL ?? 'Não é necessário em builds release'} state={metroProbe} theme={theme}/>
      </Section>

      <Section title="Rede local" theme={theme}>
        <DiagnosticRow label="Descoberta mDNS" value={discoveryLabel(discovery.state)} state={discoveryState(discovery.state)} theme={theme}/>
        <DiagnosticRow label="Mac pareado" value={discovered ? 'Encontrado na rede' : profile.deviceId ? 'Não encontrado agora' : 'Sem identidade mDNS'} detail={discovered?.name ?? profile.deviceName} state={discovered ? 'ok' : 'unavailable'} theme={theme}/>
        <DiagnosticRow label="Endpoint" value={profile.endpoint} detail={discovered && discovered.endpoint !== profile.endpoint ? `Descoberto: ${discovered.endpoint}` : undefined} state="ok" theme={theme}/>
      </Section>

      <Section title="Agent" theme={theme}>
        <DiagnosticRow label="HTTP /health" value={probeLabel(agentProbe)} state={agentProbe} theme={theme}/>
        <DiagnosticRow label="Versão" value={snapshot?.agentVersion ?? 'Aguardando snapshot'} detail={snapshot ? `Protocolo ${snapshot.protocolVersion}` : undefined} state={snapshot ? 'ok' : 'unavailable'} theme={theme}/>
        <DiagnosticRow label="Capacidades" value={snapshot?.capabilities.length ? snapshot.capabilities.join(', ') : 'Não informadas'} state={snapshot?.capabilities.length ? 'ok' : 'unavailable'} theme={theme}/>
        <DiagnosticRow label="WebSocket" value={connectionLabel(connectionState)} detail={connectionState === 'connected' ? `Token aceito${connectionDetails.lastConnectedAt ? ` · ${formatTimestamp(connectionDetails.lastConnectedAt)}` : ''}` : undefined} state={connectionState === 'connected' ? 'ok' : connectionState === 'offline' ? 'error' : 'checking'} theme={theme}/>
        <DiagnosticRow label="Último snapshot" value={snapshot ? formatRelative(snapshot.updatedAt) : 'Ainda não recebido'} detail={connectionDetails.lastSnapshotAt ? `Recebido no iPhone em ${formatTimestamp(connectionDetails.lastSnapshotAt)}` : snapshot?.updatedAt ? formatTimestamp(snapshot.updatedAt) : undefined} state={snapshot ? 'ok' : 'unavailable'} theme={theme}/>
        <DiagnosticRow label="Atualização ativa" value={intervalLabel(profile.refreshIntervalMs ?? 60_000)} state="ok" theme={theme}/>
      </Section>

      <View style={styles.action}><ActionButton label="Gerenciar dispositivos" icon="phone-portrait-outline" theme={theme} onPress={onManageDevices}/></View>

      <Section title="Providers" theme={theme}>
        {['codex', 'claude'].map(name => {
          const provider = providers.find(item => item.provider === name);
          return <DiagnosticRow key={name} label={name === 'codex' ? 'Codex' : 'Claude'} value={provider?.available ? 'Disponível' : 'Indisponível'} detail={provider?.observedAt ? `Leitura real ${formatRelative(provider.observedAt)} · ${formatTimestamp(provider.observedAt)}` : 'Nenhuma leitura real'} state={provider?.available ? 'ok' : 'unavailable'} theme={theme}/>;
        })}
      </Section>

      {(connectionDetails.lastError || message) && <View style={[styles.errorBox, { backgroundColor: theme.colors.surface }]}><Text style={[styles.errorLabel, { color: theme.colors.ink }]}>{connectionDetails.lastError ? 'Último erro' : 'Último evento'}</Text><Text style={[styles.errorText, { color: theme.colors.muted }]}>{connectionDetails.lastError ?? message}</Text></View>}
      <View style={styles.action}><ActionButton label="Testar novamente" icon="refresh-outline" theme={theme} disabled={agentProbe === 'checking' || metroProbe === 'checking'} loading={agentProbe === 'checking' || metroProbe === 'checking'} onPress={()=>void runChecks()}/></View>
      <Text style={[styles.checked, { color: theme.colors.muted }]}>{checkedAt ? `Verificado ${formatRelative(checkedAt)}` : 'Executando verificações…'}</Text>
    </ScrollView>
  </View>;
}

function Section({ title, theme, children }: { title: string; theme: Theme; children: React.ReactNode }) {
  return <View style={styles.section}><Text style={[styles.sectionTitle, { color: theme.colors.ink }]}>{title}</Text><View style={[styles.rows, { borderColor: theme.colors.line }]}>{children}</View></View>;
}

function DiagnosticRow({ label, value, detail, state, theme }: { label: string; value: string; detail?: string; state: ProbeState; theme: Theme }) {
  const color = state === 'ok' ? theme.colors.primary : state === 'error' ? theme.colors.error : state === 'checking' ? theme.colors.warning : theme.colors.muted;
  return <View style={[styles.row, { borderBottomColor: theme.colors.line }]} accessibilityLabel={`${label}: ${value}${detail ? `. ${detail}` : ''}`}>
    <View style={[styles.dot, { backgroundColor: color }]}/><Text style={[styles.label, { color: theme.colors.ink }]}>{label}</Text>
    <View style={styles.valueColumn}><Text selectable style={[styles.value, { color: theme.colors.ink }]}>{value}</Text>{detail ? <Text selectable style={[styles.detail, { color: theme.colors.muted }]}>{detail}</Text> : null}</View>
  </View>;
}

function getMetroURL(): string | null {
  if (!__DEV__) return null;
  const scriptURL = NativeModules.SourceCode?.scriptURL;
  if (typeof scriptURL !== 'string' || !/^https?:\/\//.test(scriptURL)) return null;
  try { return new URL(scriptURL).origin; } catch { return null; }
}
function probeLabel(state: ProbeState) { return state === 'ok' ? 'Acessível' : state === 'error' ? 'Inacessível' : state === 'checking' ? 'Verificando…' : 'Não aplicável'; }
function discoveryLabel(state: DiscoveryState) { return state === 'ready' ? 'Ativa' : state === 'scanning' ? 'Procurando…' : state === 'error' ? 'Falhou' : 'Não suportada'; }
function discoveryState(state: DiscoveryState): ProbeState { return state === 'ready' ? 'ok' : state === 'scanning' ? 'checking' : state === 'error' ? 'error' : 'unavailable'; }
function connectionLabel(state: ConnectionState) { return ({ idle: 'Não configurado', connecting: 'Conectando…', connected: 'Conectado e autenticado', reconnecting: 'Reconectando…', offline: 'Desconectado' } as const)[state]; }
function intervalLabel(ms: number) { return ms === 30_000 ? 'A cada 30 segundos' : ms === 300_000 ? 'A cada 5 minutos' : 'A cada 1 minuto'; }
function formatTimestamp(iso: string) { const date = new Date(iso); return Number.isFinite(date.getTime()) ? date.toLocaleString('pt-BR') : iso; }

const styles = StyleSheet.create({
  root:{flex:1},content:{paddingHorizontal:20,paddingTop:22,paddingBottom:40,maxWidth:680,width:'100%',alignSelf:'center'},section:{marginTop:24},sectionTitle:{fontSize:15,fontWeight:'700',marginBottom:9},rows:{backgroundColor:'transparent'},row:{minHeight:62,flexDirection:'row',alignItems:'center',gap:10,borderBottomWidth:StyleSheet.hairlineWidth,paddingVertical:10},dot:{width:8,height:8,borderRadius:4},label:{fontSize:14,fontWeight:'600',width:112},valueColumn:{flex:1,alignItems:'flex-end'},value:{fontSize:14,textAlign:'right',fontVariant:['tabular-nums']},detail:{fontSize:12,lineHeight:17,textAlign:'right',marginTop:2},errorBox:{padding:14,borderRadius:10,marginTop:24},errorLabel:{fontSize:13,fontWeight:'700'},errorText:{fontSize:13,lineHeight:19,marginTop:3},action:{marginTop:20},checked:{fontSize:12,textAlign:'center',marginTop:10}
});
