import { ConnectionProfile, PairingPayload, UsageSnapshot } from './types';

export function normalizeWebSocketURL(profile: ConnectionProfile): string {
  const raw = profile.endpoint.trim().replace(/\/+$/, '');
  const withScheme = /^[a-z]+:\/\//i.test(raw) ? raw : `${profile.mode === 'remote' ? 'https' : 'http'}://${raw}`;
  const url = new URL(withScheme);
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
  url.pathname = '/ws';
  url.search = 'protocol=1';
  return url.toString();
}

export function normalizeProviderRefreshURL(profile: ConnectionProfile, provider: 'codex' | 'claude'): string {
  const raw = profile.endpoint.trim().replace(/\/+$/, '');
  const withScheme = /^[a-z]+:\/\//i.test(raw) ? raw : `${profile.mode === 'remote' ? 'https' : 'http'}://${raw}`;
  const url = new URL(withScheme);
  url.pathname = `/${provider}/refresh`;
  url.search = '';
  return url.toString();
}

export function webSocketProtocols(profile:ConnectionProfile):string[]{return ['ai-usage.v1',`auth.${profile.token.trim()}`]}
export function authenticatedHeaders(profile:ConnectionProfile):Record<string,string>{return {Authorization:`Bearer ${profile.token.trim()}`}}

export function normalizeAgentURL(profile: ConnectionProfile, path: string): string {
  const raw = profile.endpoint.trim().replace(/\/+$/, '');
  const withScheme = /^[a-z]+:\/\//i.test(raw) ? raw : `${profile.mode === 'remote' ? 'https' : 'http'}://${raw}`;
  const url = new URL(withScheme);
  url.pathname = path.startsWith('/') ? path : `/${path}`;
  url.search = '';
  return url.toString();
}

export function normalizeClaudeRefreshURL(profile: ConnectionProfile): string {
  return normalizeProviderRefreshURL(profile, 'claude');
}

export function validateProfile(profile: ConnectionProfile): string | null {
  if (!profile.endpoint.trim()) return 'Informe o endereço do Mac.';
  if (!profile.token.trim()) return 'Informe o token do agent.';
  if (!/^[A-Za-z0-9_-]{32,128}$/.test(profile.token.trim())) return 'O token do agent é inválido.';
  try { normalizeWebSocketURL(profile); } catch { return 'Use um IP, hostname ou URL válido.'; }
  return null;
}

export function parsePairingPayload(raw: string): PairingPayload | null {
  try {
    const value = JSON.parse(raw) as Partial<PairingPayload>;
    if (value.type !== 'ai-usage-monitor-pairing' || value.version !== 2) return null;
    if (typeof value.endpoint !== 'string' || typeof value.ticket !== 'string' || typeof value.device !== 'string' || typeof value.deviceId !== 'string' || typeof value.expiresAt !== 'string') return null;
    if (value.ticket.length < 32 || value.ticket.length > 128 || value.device.trim().length > 120 || Date.parse(value.expiresAt) <= Date.now()) return null;
    if (!/^[a-f0-9]{16}$/.test(value.deviceId)) return null;
    if (value.device.trim() === '') return null;
    const url = new URL(`http://${value.endpoint}`);
    if (!url.hostname || !url.port || url.username || url.password || url.pathname !== '/') return null;
    if (!isPrivateIPv4(url.hostname) && !url.hostname.endsWith('.local')) return null;
    return value as PairingPayload;
  } catch {
    return null;
  }
}

export async function claimPairing(payload: PairingPayload): Promise<ConnectionProfile> {
  const url = new URL(`http://${payload.endpoint}/pair/claim`);
  const response = await fetch(url.toString(), {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({ticket:payload.ticket,name:'iPhone'})});
  if (!response.ok) throw new Error(response.status===410?'Este QR expirou ou já foi usado.':'O agent recusou o pareamento.');
  const result = await response.json() as {token?:unknown;credentialId?:unknown};
  if (typeof result.token!=='string'||result.token.length<32||typeof result.credentialId!=='string') throw new Error('Resposta de pareamento inválida.');
  return {mode:'local',endpoint:payload.endpoint,token:result.token,deviceId:payload.deviceId,deviceName:payload.device};
}

function isPrivateIPv4(host: string): boolean {
  const parts = host.split('.').map(Number);
  if (parts.length !== 4 || parts.some(part=>!Number.isInteger(part)||part<0||part>255)) return false;
  return parts[0]===10 || (parts[0]===172&&parts[1]>=16&&parts[1]<=31) || (parts[0]===192&&parts[1]===168);
}

export function parseSnapshot(value: unknown): UsageSnapshot | null {
  if (!value || typeof value !== 'object') return null;
  const data = value as Partial<UsageSnapshot>;
  if (data.protocolVersion !== 1 || typeof data.agentVersion !== 'string' || !Array.isArray(data.capabilities)) return null;
  if (typeof data.device !== 'string' || typeof data.updatedAt !== 'string' || !Array.isArray(data.providers)) return null;
  return data as UsageSnapshot;
}

export function formatRelative(iso: string, now = Date.now()): string {
  const elapsed = Math.max(0, now - new Date(iso).getTime());
  if (!Number.isFinite(elapsed)) return 'horário desconhecido';
  if (elapsed < 60_000) return 'agora';
  const minutes = Math.floor(elapsed / 60_000);
  if (minutes < 60) return `há ${minutes} min`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `há ${hours} h`;
  return `há ${Math.floor(hours / 24)} d`;
}

export function formatReset(iso: string, now = Date.now()): string {
  const target = new Date(iso).getTime();
  if (!Number.isFinite(target) || target <= 0) return 'Reset não informado';
  const diff = target - now;
  if (diff <= 0) return 'Aguardando atualização';
  const totalMinutes = Math.ceil(diff / 60_000);
  const days = Math.floor(totalMinutes / 1440);
  const hours = Math.floor((totalMinutes % 1440) / 60);
  const minutes = totalMinutes % 60;
  const countdown = days > 0 ? `${days}d ${hours}h` : hours > 0 ? `${hours}h ${minutes}min` : `${minutes}min`;
  const local = new Date(target).toLocaleString('pt-BR', { day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit' });
  return `${countdown} · ${local}`;
}
