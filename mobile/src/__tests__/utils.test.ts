import { authenticatedHeaders, formatReset, normalizeAgentURL, normalizeClaudeRefreshURL, normalizeProviderRefreshURL, normalizeWebSocketURL, parsePairingPayload, parseSnapshot, validateProfile, webSocketProtocols } from '../utils';

describe('connection utilities',()=>{
  test('normalizes local and remote endpoints',()=>{
    expect(normalizeWebSocketURL({mode:'local',endpoint:'192.168.1.8:9876',token:'a b'})).toBe('ws://192.168.1.8:9876/ws?protocol=1');
    expect(normalizeWebSocketURL({mode:'remote',endpoint:'https://demo.ngrok.app/',token:'secret'})).toBe('wss://demo.ngrok.app/ws?protocol=1');
  });
  test('keeps credentials out of URLs',()=>{const token='a'.repeat(43);const profile={mode:'local' as const,endpoint:'192.168.1.8:9876',token};expect(normalizeClaudeRefreshURL(profile)).toBe('http://192.168.1.8:9876/claude/refresh');expect(normalizeProviderRefreshURL(profile,'codex')).toBe('http://192.168.1.8:9876/codex/refresh');expect(authenticatedHeaders(profile).Authorization).toBe(`Bearer ${token}`);expect(webSocketProtocols(profile)).toEqual(['ai-usage.v1',`auth.${token}`])});
  test('builds an agent probe URL without leaking the token',()=>{expect(normalizeAgentURL({mode:'local',endpoint:'192.168.1.8:9876',token:'secret'},'/health')).toBe('http://192.168.1.8:9876/health');});
  test('accepts only versioned local pairing payloads',()=>{
    const ticket='x'.repeat(43);const deviceId='abcdef0123456789';const valid=JSON.stringify({type:'ai-usage-monitor-pairing',version:2,endpoint:'192.168.1.8:9876',ticket,device:'Mac',deviceId,expiresAt:'2099-01-01T00:00:00Z'});
    expect(parsePairingPayload(valid)?.endpoint).toBe('192.168.1.8:9876');
    expect(parsePairingPayload(JSON.stringify({type:'ai-usage-monitor-pairing',version:1,endpoint:'192.168.1.8:9876',ticket,device:'Mac',deviceId,expiresAt:'2099-01-01T00:00:00Z'}))).toBeNull();
    expect(parsePairingPayload(JSON.stringify({type:'ai-usage-monitor-pairing',version:2,endpoint:'example.com:9876',ticket,device:'Mac',deviceId,expiresAt:'2099-01-01T00:00:00Z'}))).toBeNull();
  });
  test('validates required fields',()=>{expect(validateProfile({mode:'local',endpoint:'',token:''})).toMatch(/endereço/i);expect(validateProfile({mode:'local',endpoint:'host:9876',token:'x'.repeat(43)})).toBeNull()});
  test('unknown snapshot is rejected',()=>{expect(parseSnapshot({providers:[]})).toBeNull();expect(parseSnapshot({protocolVersion:1,agentVersion:'1.1.0',capabilities:[],device:'mac',updatedAt:'2026-01-01T00:00:00Z',online:true,providers:[]})).not.toBeNull();expect(parseSnapshot({protocolVersion:2,agentVersion:'2',capabilities:[],device:'mac',updatedAt:'2026-01-01T00:00:00Z',providers:[]})).toBeNull()});
  test('reset countdown does not turn unknown into zero',()=>{expect(formatReset('',0)).toBe('Reset não informado');expect(formatReset('2026-01-01T01:00:00Z',Date.parse('2026-01-01T00:00:00Z'))).toContain('1h')});
});
