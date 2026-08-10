import { AppState, AppStateStatus } from 'react-native';
import { useCallback, useEffect, useRef, useState } from 'react';
import { AuthInfo, ConnectionProfile, ConnectionState, UsageSnapshot } from './types';
import { authenticatedHeaders, normalizeAgentURL, normalizeProviderRefreshURL, normalizeWebSocketURL, parseSnapshot, webSocketProtocols } from './utils';

const MAX_BACKOFF = 30_000;

export function useUsageConnection(profile: ConnectionProfile | null) {
  const [snapshot, setSnapshot] = useState<UsageSnapshot | null>(null);
  const [state, setState] = useState<ConnectionState>(profile ? 'connecting' : 'idle');
  const [message, setMessage] = useState<string | null>(null);
	const [refreshingClaude, setRefreshingClaude] = useState(false);
	const [refreshingAll, setRefreshingAll] = useState(false);
	const [lastError, setLastError] = useState<string | null>(null);
	const [lastConnectedAt, setLastConnectedAt] = useState<string | null>(null);
	const [lastSnapshotAt, setLastSnapshotAt] = useState<string | null>(null);
  const [authInfo,setAuthInfo]=useState<AuthInfo|null>(null);
  const socket = useRef<WebSocket | null>(null);
  const retryTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const attempts = useRef(0);
  const active = useRef(true);
	const reconnectRef = useRef<() => void>(() => undefined);
	const refreshAllRef = useRef<(announce?: boolean) => Promise<void>>(async () => undefined);
	const refreshingAllRef = useRef(false);

  const disconnect = useCallback(() => {
    if (retryTimer.current) clearTimeout(retryTimer.current);
    retryTimer.current = null;
    socket.current?.close();
    socket.current = null;
  }, []);

  const connect = useCallback(() => {
    if (!profile || !active.current) return;
    disconnect();
    setState(attempts.current ? 'reconnecting' : 'connecting');
    setMessage(null);
    let ws: WebSocket;
	try {
		ws = new WebSocket(normalizeWebSocketURL(profile),webSocketProtocols(profile));
	} catch {
		setState('offline');
		setMessage('Endereço inválido.');
		setLastError('Endereço inválido.');
		return;
	}
    socket.current = ws;
    ws.onopen = () => { attempts.current = 0; setState('connected'); setMessage(null); setLastConnectedAt(new Date().toISOString()); void fetch(normalizeAgentURL(profile,'/auth/info'),{headers:authenticatedHeaders(profile)}).then(async response=>{if(response.ok)setAuthInfo(await response.json() as AuthInfo)}).catch(()=>undefined); };
    ws.onmessage = event => {
		try {
			const parsed = parseSnapshot(JSON.parse(String(event.data)));
			if (parsed) { setSnapshot(parsed); setLastSnapshotAt(new Date().toISOString()); }
			else { setMessage('O agent enviou um formato desconhecido.'); setLastError('O agent enviou um formato desconhecido.'); }
		} catch {
			setMessage('Não foi possível ler a atualização do agent.');
			setLastError('Não foi possível ler a atualização do agent.');
		}
    };
    ws.onerror = () => { setMessage('Não foi possível alcançar o Mac.'); setLastError('Não foi possível alcançar o Mac.'); };
    ws.onclose = event => {
		if (socket.current !== ws) return;
		socket.current = null;
      if (!active.current) return;
      if (event.code === 1008 || event.code === 4001) { setState('offline'); setMessage('Token recusado pelo agent.'); setLastError('Token recusado pelo agent.'); return; }
      const delay = Math.min(1000 * 2 ** attempts.current, MAX_BACKOFF);
      attempts.current += 1;
      setState('reconnecting');
		retryTimer.current = setTimeout(() => reconnectRef.current(), delay);
    };
  }, [disconnect, profile]);

  useEffect(() => {
    active.current = true;
    attempts.current = 0;
		reconnectRef.current = connect;
		const initialConnect = setTimeout(() => {
			if (profile) connect();
			else {
				disconnect();
				setState('idle');
				setSnapshot(null);
			}
		}, 0);
    const onAppState = (next: AppStateStatus) => {
      active.current = next === 'active';
      if (next === 'active') {
		attempts.current = 0;
		connect();
		void refreshAllRef.current(false);
	  } else disconnect();
    };
    const subscription = AppState.addEventListener('change', onAppState);
		return () => {
			active.current = false;
			clearTimeout(initialConnect);
			subscription.remove();
			disconnect();
		};
  }, [connect, disconnect, profile]);

	const refreshProvider = useCallback(async (provider: 'codex' | 'claude') => {
		if (!profile) throw new Error('missing profile');
		const response = await fetch(normalizeProviderRefreshURL(profile, provider), { method: 'POST', headers:authenticatedHeaders(profile) });
		if (!response.ok) throw new Error(String(response.status));
	}, [profile]);

	const refreshClaude = useCallback(async () => {
		if (!profile || refreshingClaude) return;
		setRefreshingClaude(true);
		setMessage(null);
		try {
			await refreshProvider('claude');
			setMessage('Claude atualizado sem consumo de tokens.');
		} catch {
			setMessage('Não foi possível atualizar o Claude agora.');
			setLastError('Não foi possível atualizar o Claude agora.');
		} finally {
			setRefreshingClaude(false);
		}
	}, [profile, refreshProvider, refreshingClaude]);

	const refreshAll = useCallback(async (announce = true) => {
		if (!profile || refreshingAllRef.current) return;
		refreshingAllRef.current = true;
		setRefreshingAll(true);
		if (announce) setMessage(null);
		const results = await Promise.allSettled([refreshProvider('codex'), refreshProvider('claude')]);
		const failures = results.filter(result => result.status === 'rejected').length;
		if (announce) {
			const resultMessage = failures === 0 ? 'Leituras atualizadas.' : failures === 2 ? 'Não foi possível atualizar os providers.' : 'Um provider não pôde ser atualizado.';
			setMessage(resultMessage);
			if (failures > 0) setLastError(resultMessage);
		}
		refreshingAllRef.current = false;
		setRefreshingAll(false);
	}, [profile, refreshProvider]);
	refreshAllRef.current = refreshAll;

	useEffect(() => {
		if (!profile) return;
		const interval = profile.refreshIntervalMs ?? 60_000;
		const timer = setInterval(() => {
			if (active.current) void refreshAllRef.current(false);
		}, interval);
		return () => clearInterval(timer);
	}, [profile]);

  return { snapshot, state, message, authInfo, diagnostics: { lastError, lastConnectedAt, lastSnapshotAt }, reconnect: connect, refreshAll, refreshingAll, refreshClaude, refreshingClaude };
}
