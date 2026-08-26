import { computeUsageAlerts, isQuietHour } from '../useUsageNotifications';
import { NotificationPreferences, UsageSnapshot } from '../types';

const prefs:NotificationPreferences={enabled:true,thresholds:[50,75,90],providerUnavailable:true,staleData:true,windowReset:true,fiveHourAlerts:true,weeklyAlerts:true,predictiveAlerts:true,quietHours:false,quietStartHour:22,quietEndHour:8};
const snapshot=(used:number,available=true,observedAt='2026-08-08T20:00:00Z'):UsageSnapshot=>({protocolVersion:1,agentVersion:'1.1.0',capabilities:[],device:'mac',online:true,updatedAt:observedAt,providers:[{provider:'codex',available,observedAt,weekly:{usedPercentage:used,remainingPercentage:100-used,resetsAt:'2026-08-15T20:00:00Z'}}]});

describe('usage alerts',()=>{
 test('does not notify on the initial snapshot',()=>expect(computeUsageAlerts(null,snapshot(90),prefs,Date.parse('2026-08-08T20:01:00Z'),new Set())).toEqual([]));
 test('notifies only thresholds that were crossed',()=>{const alerts=computeUsageAlerts(snapshot(49),snapshot(76),prefs,Date.parse('2026-08-08T20:01:00Z'),new Set());expect(alerts.map(a=>a.title)).toEqual(['Codex atingiu 50%','Codex atingiu 75%'])});
 test('deduplicates unavailable alerts',()=>{const emitted=new Set<string>();expect(computeUsageAlerts(snapshot(10),snapshot(10,false),prefs,Date.now(),emitted)).toHaveLength(1);expect(computeUsageAlerts(snapshot(10),snapshot(10,false),prefs,Date.now(),emitted)).toHaveLength(0)});
 test('predicts depletion only from multiple recent samples',()=>{const first=snapshot(10,true,'2026-08-08T19:00:00Z');const second=snapshot(20,true,'2026-08-08T20:00:00Z');const current=snapshot(30,true,'2026-08-08T21:00:00Z');const alerts=computeUsageAlerts(second,current,{...prefs,thresholds:[]},Date.parse('2026-08-08T21:00:00Z'),new Set(),[first,second]);expect(alerts[0]?.title).toContain('esgotar');expect(computeUsageAlerts(second,current,{...prefs,thresholds:[]},Date.parse('2026-08-08T21:00:00Z'),new Set(),[second])).toEqual([])});
 test('respects overnight quiet hours',()=>{expect(isQuietHour(23,22,8)).toBe(true);expect(isQuietHour(7,22,8)).toBe(true);expect(isQuietHour(12,22,8)).toBe(false);expect(computeUsageAlerts(snapshot(49),snapshot(76),{...prefs,quietHours:true},new Date(2026,7,8,23).getTime(),new Set())).toEqual([])});
 test('uses the OpenCode display name in provider alerts',()=>{const before={...snapshot(0),providers:[{provider:'opencode',available:true,observedAt:'2026-08-08T20:00:00Z'}]};const current={...before,providers:[{...before.providers[0],available:false}]};expect(computeUsageAlerts(before,current,prefs,Date.parse('2026-08-08T20:01:00Z'),new Set())[0]?.title).toBe('OpenCode indisponível')});
});
