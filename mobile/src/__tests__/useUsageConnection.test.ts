import { act, renderHook } from '@testing-library/react-native';
import { AppState } from 'react-native';
import { useUsageConnection } from '../useUsageConnection';

class MockSocket {
  static instances: MockSocket[] = [];
  onopen: (() => void) | null = null;
  onmessage: ((event: { data: string }) => void) | null = null;
  onerror: (() => void) | null = null;
  onclose: ((event: { code: number }) => void) | null = null;
  close = jest.fn();
  constructor(public url: string,public protocols?:string[]) { MockSocket.instances.push(this); }
}

describe('useUsageConnection', () => {
  const profile = { mode: 'local' as const, endpoint: '192.168.1.8:9876', token: 's'.repeat(43) };
  beforeEach(() => {
    jest.useFakeTimers();
    MockSocket.instances = [];
		globalThis.WebSocket = MockSocket as unknown as typeof WebSocket;
    globalThis.fetch=jest.fn().mockResolvedValue({ok:true,json:async()=>({kind:'device',migrationRequired:false,credentialId:'phone'})}) as jest.Mock;
  });
  afterEach(() => { jest.useRealTimers(); jest.restoreAllMocks(); });

  test('connects, accepts a snapshot and reconnects with cleanup', async () => {
    const remove = jest.fn();
    jest.spyOn(AppState, 'addEventListener').mockReturnValue({ remove } as never);
		const { result, unmount } = await renderHook(() => useUsageConnection(profile));
		await act(async () => jest.runOnlyPendingTimers());
    expect(MockSocket.instances).toHaveLength(1);
		expect(MockSocket.instances[0].url).not.toContain(profile.token);
		expect(MockSocket.instances[0].protocols).toEqual(['ai-usage.v1',`auth.${profile.token}`]);
		await act(async () => MockSocket.instances[0].onopen?.());
    expect(result.current.state).toBe('connected');
		expect(result.current.diagnostics.lastConnectedAt).not.toBeNull();
		await act(async () => MockSocket.instances[0].onmessage?.({ data: JSON.stringify({ protocolVersion:1, agentVersion:'1.1.0', capabilities:[], device: 'mac', online: true, updatedAt: '2026-08-08T19:00:00Z', providers: [] }) }));
    expect(result.current.snapshot?.device).toBe('mac');
		expect(result.current.diagnostics.lastSnapshotAt).not.toBeNull();
		await act(async () => MockSocket.instances[0].onclose?.({ code: 1006 }));
    expect(result.current.state).toBe('reconnecting');
		await act(async () => jest.advanceTimersByTime(1000));
    expect(MockSocket.instances).toHaveLength(2);
		await act(async () => unmount());
    expect(remove).toHaveBeenCalled();
    expect(MockSocket.instances[1].close).toHaveBeenCalled();
  });
});
