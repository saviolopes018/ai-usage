import { StatusBar } from 'expo-status-bar';
import { useEffect, useMemo, useState } from 'react';
import { ActivityIndicator, NativeModules, Pressable, RefreshControl, ScrollView, StyleSheet, Text, useColorScheme, View } from 'react-native';
import { SafeAreaProvider, SafeAreaView } from 'react-native-safe-area-context';
import { ConnectionBar } from './src/components/ConnectionBar';
import { ProviderSection } from './src/components/ProviderSection';
import { PairingScanner } from './src/components/PairingScanner';
import { SetupScreen } from './src/components/SetupScreen';
import { DiagnosticsScreen } from './src/components/DiagnosticsScreen';
import { NotificationSettingsScreen } from './src/components/NotificationSettingsScreen';
import { DevicesScreen } from './src/components/DevicesScreen';
import { clearProfile, loadProfile, saveProfile } from './src/storage';
import { createTheme } from './src/theme';
import { ConnectionProfile } from './src/types';
import { useUsageConnection } from './src/useUsageConnection';
import { useDeviceDiscovery } from './src/useDeviceDiscovery';
import { useUsageNotifications } from './src/useUsageNotifications';
import { claimPairing } from './src/utils';
import { Icon } from './src/components/ui';

const INITIAL_NOW = Date.now();

function Monitor() {
  const scheme = useColorScheme();
  const theme = useMemo(() => createTheme(scheme), [scheme]);
  const [profile, setProfile] = useState<ConnectionProfile | null>(null);
  const [loaded, setLoaded] = useState(false);
  const [editing, setEditing] = useState(false);
  const [scanning, setScanning] = useState(false);
  const [managingDevices,setManagingDevices]=useState(false);
  const [activeTab, setActiveTab] = useState<'monitor' | 'diagnostics' | 'notifications'>('monitor');
  const [pairedDevice, setPairedDevice] = useState<string|null>(null);
  const [now, setNow] = useState(INITIAL_NOW);
  const { snapshot, state, message, authInfo, diagnostics: connectionDetails, refreshAll, refreshingAll, refreshClaude, refreshingClaude } = useUsageConnection(profile);
  const discovery = useDeviceDiscovery();
  const notifications=useUsageNotifications(snapshot,state);
  const networkDeviceFound=Boolean(profile?.deviceId&&discovery.devices.some(device=>device.id===profile.deviceId));

  useEffect(() => { loadProfile().then(setProfile).catch(() => setProfile(null)).finally(() => setLoaded(true)); }, []);
  useEffect(() => { const timer = setInterval(() => setNow(Date.now()), 30_000); return () => clearInterval(timer); }, []);
  useEffect(()=>{const found=profile?.deviceId?discovery.devices.find(device=>device.id===profile.deviceId):undefined;if(!profile||!found||profile.endpoint===found.endpoint)return;const next={...profile,endpoint:found.endpoint,deviceName:found.name};saveProfile(next).then(()=>setProfile(next)).catch(()=>undefined)},[discovery.devices,profile]);
  useEffect(()=>{if(!snapshot)return;NativeModules.WidgetBridge?.updateSnapshot(JSON.stringify(snapshot))},[snapshot]);

  const handleSave = async (next: ConnectionProfile) => { await saveProfile(next); setProfile(next); setEditing(false); };
  if (!loaded) return <View style={[styles.center,{backgroundColor:theme.colors.bg}]}><ActivityIndicator color={theme.colors.primary}/><Text style={[styles.loading,{color:theme.colors.muted}]}>Carregando conexão segura…</Text></View>;
  if (scanning) return <PairingScanner theme={theme} onCancel={()=>setScanning(false)} onPair={async payload=>{const next=await claimPairing(payload);setScanning(false);setPairedDevice(payload.device);setProfile(next);setEditing(true)}}/>;
  if (!profile || editing) return <SetupScreen initial={profile} pairedDevice={pairedDevice} discovery={discovery} theme={theme} onSave={async next=>{await handleSave(next);setPairedDevice(null)}} onScan={()=>setScanning(true)} onCancel={profile?()=>{setPairedDevice(null);setEditing(false)}:undefined}/>;
  if(managingDevices)return <SafeAreaView style={[styles.safe,{backgroundColor:theme.colors.bg}]} edges={['top','left','right']}><DevicesScreen profile={profile} authInfo={authInfo} theme={theme} onBack={()=>setManagingDevices(false)} onPair={()=>setScanning(true)} onSelfRevoked={async()=>{await clearProfile();setProfile(null);setManagingDevices(false)}}/></SafeAreaView>;
  return <SafeAreaView style={[styles.safe,{backgroundColor:theme.colors.bg}]} edges={['top','left','right']}>
    <StatusBar style={theme.dark?'light':'dark'}/>
    {activeTab==='monitor'&&<View style={styles.appHeader}><View style={styles.top}><View style={styles.heading}><Text style={[styles.title,{color:theme.colors.ink}]}>AI Usage</Text><View style={styles.deviceRow}><View style={[styles.deviceDot,{backgroundColor:state==='connected'?theme.colors.primary:state==='offline'?theme.colors.error:theme.colors.warning}]}/><Text style={[styles.device,{color:theme.colors.muted}]} numberOfLines={1}>{snapshot?.device??'Aguardando o Mac'}</Text></View></View><Pressable disabled={refreshingAll} onPress={()=>void refreshAll()} accessibilityRole="button" accessibilityLabel="Atualizar leituras agora" style={({pressed})=>[styles.refresh,{backgroundColor:pressed?theme.colors.surfacePressed:theme.colors.surface,opacity:refreshingAll?0.6:1}]}>{refreshingAll?<ActivityIndicator size="small" color={theme.colors.primary}/>:<Icon name="refresh" size={20} color={theme.colors.ink}/>}</Pressable></View></View>}
    <View style={styles.body}>{activeTab==='monitor'?<ScrollView contentContainerStyle={styles.content} refreshControl={<RefreshControl refreshing={refreshingAll} onRefresh={()=>void refreshAll()} tintColor={theme.colors.primary}/>}> 
        <ConnectionBar state={state} updatedAt={snapshot?.updatedAt} networkDeviceFound={networkDeviceFound} theme={theme} onPress={()=>setEditing(true)}/>
        {authInfo?.migrationRequired&&<Pressable onPress={()=>setScanning(true)} accessibilityRole="button" style={[styles.migration,{backgroundColor:theme.colors.primarySoft}]}><View style={styles.migrationCopy}><Text style={[styles.migrationTitle,{color:theme.colors.ink}]}>Proteja este iPhone</Text><Text style={[styles.noticeText,{color:theme.colors.muted}]}>O perfil usa uma credencial antiga. Toque para migrar por QR.</Text></View><Text style={[styles.migrationArrow,{color:theme.colors.primary}]}>›</Text></Pressable>}
        {message&&<View style={[styles.notice,{backgroundColor:theme.colors.surface}]}><Text style={[styles.noticeText,{color:theme.colors.muted}]}>{message}</Text></View>}
        {snapshot ? <View>{snapshot.providers.map(provider=><ProviderSection key={provider.provider} provider={provider} theme={theme} now={now} onRefresh={provider.provider==='claude'?refreshClaude:undefined} refreshing={provider.provider==='claude'&&refreshingClaude}/>)}</View> : <View style={styles.waiting}><Text style={[styles.waitingTitle,{color:theme.colors.ink}]}>Esperando o primeiro snapshot</Text><Text style={[styles.waitingBody,{color:theme.colors.muted}]}>Mantenha o Mac e o celular na mesma rede Wi‑Fi. O app tentará novamente automaticamente.</Text></View>}
        <Text style={[styles.footer,{color:theme.colors.muted}]}>Dados transmitidos diretamente pelo seu Mac.</Text>
      </ScrollView>:activeTab==='diagnostics'?<DiagnosticsScreen profile={profile} connectionState={state} snapshot={snapshot} message={message} connectionDetails={connectionDetails} discovery={discovery} theme={theme} onManageDevices={()=>setManagingDevices(true)}/>:<NotificationSettingsScreen value={notifications.preferences} supported={notifications.supported} permission={notifications.permission} theme={theme} onChange={notifications.update} onTest={notifications.test}/>}</View>
    <View accessibilityRole="tablist" style={[styles.bottomNav,{backgroundColor:theme.colors.nav,borderTopColor:theme.colors.line}]}>
      <AppTab label="Monitor" icon="speedometer-outline" selectedIcon="speedometer" selected={activeTab==='monitor'} onPress={()=>setActiveTab('monitor')} theme={theme}/>
      <AppTab label="Diagnóstico" icon="pulse-outline" selectedIcon="pulse" accessibilityLabel="Diagnóstico da conexão" selected={activeTab==='diagnostics'} onPress={()=>setActiveTab('diagnostics')} theme={theme}/>
      <AppTab label="Alertas" icon="notifications-outline" selectedIcon="notifications" accessibilityLabel="Configurar notificações" selected={activeTab==='notifications'} onPress={()=>setActiveTab('notifications')} theme={theme}/>
    </View>
  </SafeAreaView>;
}

export function AppTab({label,icon,selectedIcon,accessibilityLabel,selected,onPress,theme}:{label:string;icon:React.ComponentProps<typeof Icon>['name'];selectedIcon:React.ComponentProps<typeof Icon>['name'];accessibilityLabel?:string;selected:boolean;onPress:()=>void;theme:ReturnType<typeof createTheme>}){
  return <Pressable accessibilityRole="tab" accessibilityLabel={accessibilityLabel??label} accessibilityState={{selected}} onPress={onPress} style={({pressed})=>[styles.tab,{backgroundColor:selected?theme.colors.primarySoft:'transparent',opacity:pressed?.65:1}]}><Icon name={selected?selectedIcon:icon} size={21} color={selected?theme.colors.primary:theme.colors.muted}/><Text style={[styles.tabText,{color:selected?theme.colors.primary:theme.colors.muted,fontWeight:selected?'700':'600'}]}>{label}</Text></Pressable>;
}

export default function App(){return <SafeAreaProvider><Monitor/></SafeAreaProvider>}

const styles=StyleSheet.create({safe:{flex:1},body:{flex:1},appHeader:{paddingHorizontal:20,maxWidth:680,width:'100%',alignSelf:'center'},content:{paddingHorizontal:20,paddingTop:14,paddingBottom:32,maxWidth:680,width:'100%',alignSelf:'center'},center:{flex:1,alignItems:'center',justifyContent:'center'},loading:{fontSize:14,marginTop:12},top:{paddingTop:18,paddingBottom:12,flexDirection:'row',justifyContent:'space-between',alignItems:'center'},heading:{flex:1,paddingRight:16},title:{fontSize:28,fontWeight:'700',letterSpacing:-.6},deviceRow:{flexDirection:'row',alignItems:'center',gap:7,marginTop:4},deviceDot:{width:7,height:7,borderRadius:4},device:{fontSize:13,maxWidth:280},refresh:{width:44,height:44,borderRadius:10,alignItems:'center',justifyContent:'center'},bottomNav:{minHeight:68,flexDirection:'row',gap:8,borderTopWidth:StyleSheet.hairlineWidth,paddingHorizontal:14,paddingTop:8,paddingBottom:6},tab:{flex:1,minHeight:52,borderRadius:10,alignItems:'center',justifyContent:'center',gap:3},tabText:{fontSize:11},notice:{padding:12,borderRadius:10,marginTop:12},noticeText:{fontSize:13,lineHeight:18},migration:{padding:14,borderRadius:12,marginTop:12,flexDirection:'row',alignItems:'center',minHeight:68},migrationCopy:{flex:1,paddingRight:10},migrationTitle:{fontSize:14,fontWeight:'700',marginBottom:2},migrationArrow:{fontSize:26,fontWeight:'400'},waiting:{paddingVertical:48,maxWidth:420},waitingTitle:{fontSize:18,fontWeight:'600'},waitingBody:{fontSize:15,lineHeight:22,marginTop:8},footer:{fontSize:12,textAlign:'center',marginTop:18}});
