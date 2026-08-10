import { useEffect, useRef, useState } from 'react';
import { AccessibilityInfo, ActivityIndicator, Animated, Pressable, StyleSheet, Text, View } from 'react-native';
import { ProviderUsage, UsageWindow } from '../types';
import { Theme } from '../theme';
import { formatRelative, formatReset } from '../utils';

function WindowRow({ label, window, theme, now }: { label:string; window?:UsageWindow; theme:Theme; now:number }) {
  if (!window) return <View style={styles.window}><View style={styles.windowHead}><Text style={[styles.windowLabel,{color:theme.colors.ink}]}>{label}</Text><Text style={[styles.unknown,{color:theme.colors.muted}]}>Sem dados</Text></View><View style={[styles.emptyTrack,{backgroundColor:theme.colors.surface}]} /><Text style={[styles.reset,{color:theme.colors.muted}]}>Janela ainda não informada pelo provider</Text></View>;
  const used=Math.max(0,Math.min(100,window.usedPercentage));
  const remaining=Math.max(0,Math.min(100,window.remainingPercentage));
  const meterColor=used>=90?theme.colors.error:used>=75?theme.colors.warning:theme.colors.primary;
  return <View style={styles.window} accessible accessibilityLabel={`${label}: ${used} por cento usado, ${window.remainingPercentage} por cento restante. ${formatReset(window.resetsAt,now)}`}>
    <View style={styles.windowHead}><Text style={[styles.windowLabel,{color:theme.colors.ink}]}>{label}</Text><View style={styles.metric}><Text style={[styles.remaining,{color:theme.colors.ink}]}>{Math.round(remaining)}%</Text><Text style={[styles.remainingLabel,{color:theme.colors.muted}]}> livre</Text></View></View>
    <View style={[styles.track,{backgroundColor:theme.colors.surfaceRaised}]}><MeterFill used={used} color={meterColor}/></View>
    <View style={styles.windowFoot}><Text style={[styles.reset,{color:theme.colors.muted}]}>{formatReset(window.resetsAt,now)}</Text><Text style={[styles.used,{color:meterColor}]}>{Math.round(used)}% usado</Text></View>
  </View>;
}

function MeterFill({used,color}:{used:number;color:string}) {
  const progress=useRef(new Animated.Value(used)).current;
  const [reduceMotion,setReduceMotion]=useState(false);
  useEffect(()=>{AccessibilityInfo.isReduceMotionEnabled().then(setReduceMotion);const subscription=AccessibilityInfo.addEventListener('reduceMotionChanged',setReduceMotion);return()=>subscription.remove()},[]);
  useEffect(()=>{if(reduceMotion){progress.setValue(used);return}Animated.timing(progress,{toValue:used,duration:200,useNativeDriver:false}).start()},[progress,reduceMotion,used]);
  const width=progress.interpolate({inputRange:[0,100],outputRange:['0%','100%']});
  return <Animated.View style={[styles.fill,{backgroundColor:color,width}]}/>;
}

export function ProviderSection({ provider, theme, now, onRefresh, refreshing=false }: {provider:ProviderUsage;theme:Theme;now:number;onRefresh?:()=>void;refreshing?:boolean}) {
  const name=provider.provider==='codex'?'Codex':provider.provider==='claude'?'Claude':provider.provider;
  const mark=provider.provider==='codex'?'Cₓ':provider.provider==='claude'?'Cl':name.slice(0,2);
  const providerColor=provider.provider==='claude'?theme.colors.claude:theme.colors.codex;
  const providerSoft=provider.provider==='claude'?theme.colors.claudeSoft:theme.colors.codexSoft;
  const stale=provider.available&&now-new Date(provider.observedAt).getTime()>15*60_000;
  return <View style={[styles.section,{borderBottomColor:theme.colors.line}]}>
    <View style={styles.heading}><View style={styles.identity}><View style={[styles.mark,{backgroundColor:providerSoft}]}><Text style={[styles.markText,{color:providerColor}]}>{mark}</Text></View><View><Text style={[styles.name,{color:theme.colors.ink}]}>{name}</Text><Text style={[styles.observed,{color:stale?theme.colors.warning:theme.colors.muted}]}>{provider.available?(stale?`Leitura antiga · ${formatRelative(provider.observedAt,now)}`:`Leitura real · ${formatRelative(provider.observedAt,now)}`):'Sem leitura recente'}</Text></View></View>{onRefresh&&<Pressable disabled={refreshing} onPress={onRefresh} accessibilityRole="button" accessibilityLabel="Atualizar limites do Claude" style={({pressed})=>[styles.refresh,{backgroundColor:pressed?theme.colors.surfacePressed:theme.colors.surface,opacity:refreshing?.6:1}]}>{refreshing?<ActivityIndicator size="small" color={providerColor}/>:<Text style={[styles.refreshLabel,{color:providerColor}]}>Atualizar</Text>}</Pressable>}</View>
    {!provider.available ? <View style={[styles.unavailable,{backgroundColor:theme.colors.surface}]}><Text style={[styles.unavailableTitle,{color:theme.colors.ink}]}>Provider indisponível</Text><Text style={[styles.unavailableBody,{color:theme.colors.muted}]}>{provider.provider==='claude'?'Abra o Claude Code CLI e envie uma mensagem para receber a primeira leitura.':`Não foi possível consultar o ${name} neste Mac.`}</Text></View> : <><WindowRow label="5 horas" window={provider.fiveHour} theme={theme} now={now}/><WindowRow label="Semanal" window={provider.weekly} theme={theme} now={now}/></>}
  </View>;
}
const styles=StyleSheet.create({section:{paddingVertical:26,borderBottomWidth:StyleSheet.hairlineWidth},heading:{flexDirection:'row',alignItems:'center',justifyContent:'space-between',gap:12,marginBottom:24},identity:{flexDirection:'row',alignItems:'center',gap:12,flex:1},mark:{width:40,height:40,borderRadius:10,alignItems:'center',justifyContent:'center'},markText:{fontSize:14,fontWeight:'800',letterSpacing:-.2},name:{fontSize:20,fontWeight:'700',letterSpacing:-.3},observed:{fontSize:12,marginTop:2},refresh:{minWidth:88,minHeight:44,borderRadius:10,alignItems:'center',justifyContent:'center',paddingHorizontal:12},refreshLabel:{fontSize:14,fontWeight:'600'},window:{marginBottom:26},windowHead:{flexDirection:'row',justifyContent:'space-between',alignItems:'baseline',gap:12},windowLabel:{fontSize:15,fontWeight:'600'},metric:{flexDirection:'row',alignItems:'baseline'},remaining:{fontSize:22,fontWeight:'700',letterSpacing:-.5,fontVariant:['tabular-nums']},remainingLabel:{fontSize:13,fontWeight:'600'},track:{height:7,borderRadius:4,overflow:'hidden',marginTop:10},emptyTrack:{height:7,borderRadius:4,marginTop:10},fill:{height:'100%',borderRadius:4},windowFoot:{flexDirection:'row',justifyContent:'space-between',alignItems:'center',gap:12,marginTop:8},reset:{fontSize:12,fontVariant:['tabular-nums'],flexShrink:1},used:{fontSize:12,fontWeight:'600',fontVariant:['tabular-nums']},unknown:{fontSize:13,fontWeight:'600'},unavailable:{padding:16,borderRadius:12},unavailableTitle:{fontSize:15,fontWeight:'600'},unavailableBody:{fontSize:14,lineHeight:20,marginTop:4}});
