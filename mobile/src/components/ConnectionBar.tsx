import { Pressable, StyleSheet, Text, View } from 'react-native';
import { ConnectionState } from '../types';
import { Theme } from '../theme';
import { formatRelative } from '../utils';
import { Icon } from './ui';

const labels: Record<ConnectionState, string> = { idle: 'Não configurado', connecting: 'Conectando', connected: 'Conectado', reconnecting: 'Reconectando', offline: 'Offline' };

export function ConnectionBar({ state, updatedAt, networkDeviceFound, theme, onPress }: { state: ConnectionState; updatedAt?: string; networkDeviceFound?: boolean; theme: Theme; onPress: () => void }) {
  const ok = state === 'connected';
  const color=ok?theme.colors.success:state==='offline'?theme.colors.error:theme.colors.warning;
  return <Pressable onPress={onPress} accessibilityRole="button" accessibilityLabel={`${labels[state]}. Abrir configuração`} style={({ pressed }) => [styles.row, { backgroundColor: pressed?theme.colors.surfacePressed:theme.colors.surface }]}> 
    <View style={[styles.icon,{backgroundColor:ok?theme.colors.primarySoft:state==='offline'?theme.colors.errorSoft:theme.colors.warningSoft}]}><Icon name={ok?'wifi':'wifi-outline'} size={19} color={color}/></View>
    <View style={styles.copy}><Text style={[styles.label, { color: theme.colors.ink }]}>{labels[state]}</Text><Text style={[styles.detail, { color: theme.colors.muted }]}>{networkDeviceFound?`Mac encontrado · ${updatedAt?`atualizado ${formatRelative(updatedAt)}`:'aguardando dados'}`:updatedAt?`Atualizado ${formatRelative(updatedAt)}`:'Toque para configurar'}</Text></View>
    <Icon name="chevron-forward" size={18} color={theme.colors.subtle}/>
  </Pressable>;
}
const styles = StyleSheet.create({ row:{minHeight:66,borderRadius:12,paddingHorizontal:12,flexDirection:'row',alignItems:'center',gap:12},icon:{width:40,height:40,borderRadius:10,alignItems:'center',justifyContent:'center'},copy:{flex:1},label:{fontSize:15,fontWeight:'700'},detail:{fontSize:12,marginTop:2} });
