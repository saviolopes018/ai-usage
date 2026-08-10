import { BarcodeScanningResult, CameraView, useCameraPermissions } from 'expo-camera';
import { useRef, useState } from 'react';
import { ActivityIndicator, Pressable, StyleSheet, Text, View } from 'react-native';
import { PairingPayload } from '../types';
import { Theme } from '../theme';
import { parsePairingPayload } from '../utils';

export function PairingScanner({ theme, onPair, onCancel }: {theme:Theme;onPair:(payload:PairingPayload)=>Promise<void>;onCancel:()=>void}) {
  const [permission, requestPermission] = useCameraPermissions();
  const [error, setError] = useState<string|null>(null);
  const scanned = useRef(false);
  const [claiming,setClaiming]=useState(false);
  const handleScan = ({data}: BarcodeScanningResult) => {
    if (scanned.current) return;
    const payload = parsePairingPayload(data);
    if (!payload) { setError('Este QR não pertence ao AI Usage Monitor.'); return; }
    scanned.current = true;
    setClaiming(true);
    void onPair(payload).catch(reason=>{setError(reason instanceof Error?reason.message:'Falha no pareamento.');scanned.current=false}).finally(()=>setClaiming(false));
  };
  if (!permission) return <View style={[styles.center,{backgroundColor:theme.colors.bg}]}><ActivityIndicator color={theme.colors.primary}/></View>;
  if (!permission.granted) return <View style={[styles.permission,{backgroundColor:theme.colors.bg}]}><Text style={[styles.title,{color:theme.colors.ink}]}>Câmera necessária</Text><Text style={[styles.body,{color:theme.colors.muted}]}>A câmera é usada somente para ler o QR exibido pelo agent no seu Mac.</Text><Pressable onPress={requestPermission} accessibilityRole="button" style={[styles.primary,{backgroundColor:theme.colors.primary}]}><Text style={styles.primaryText}>Permitir câmera</Text></Pressable><Pressable onPress={onCancel} accessibilityRole="button" style={styles.cancel}><Text style={[styles.cancelText,{color:theme.colors.muted}]}>Voltar</Text></Pressable></View>;
  return <View style={styles.root}><CameraView style={StyleSheet.absoluteFill} facing="back" barcodeScannerSettings={{barcodeTypes:['qr']}} onBarcodeScanned={claiming?undefined:handleScan}/><View style={styles.overlay}><View style={styles.top}><Text style={styles.scanTitle}>Escaneie o QR do agent</Text><Text style={styles.scanBody}>No Mac, execute: ./usage-agent pair</Text></View><View style={[styles.frame,{borderColor:error?theme.colors.error:'#FFFFFF'}]}>{claiming&&<ActivityIndicator color="#FFF" size="large"/>}</View><View style={styles.bottom}>{error&&<Text accessibilityRole="alert" style={styles.scanError}>{error}</Text>}<Pressable onPress={onCancel} accessibilityRole="button" style={styles.close}><Text style={styles.closeText}>Cancelar</Text></Pressable></View></View></View>;
}
const styles=StyleSheet.create({root:{flex:1,backgroundColor:'#000'},center:{flex:1,alignItems:'center',justifyContent:'center'},permission:{flex:1,paddingHorizontal:28,justifyContent:'center'},title:{fontSize:26,fontWeight:'700'},body:{fontSize:16,lineHeight:23,marginTop:10,marginBottom:28},primary:{height:52,borderRadius:10,alignItems:'center',justifyContent:'center'},primaryText:{color:'#FFF',fontSize:16,fontWeight:'700'},cancel:{minHeight:48,alignItems:'center',justifyContent:'center',marginTop:8},cancelText:{fontSize:15,fontWeight:'600'},overlay:{flex:1,backgroundColor:'rgba(0,0,0,.34)',alignItems:'center',justifyContent:'space-between',paddingTop:80,paddingBottom:50,paddingHorizontal:24},top:{alignItems:'center'},scanTitle:{color:'#FFF',fontSize:22,fontWeight:'700'},scanBody:{color:'#E4E9E7',fontSize:14,marginTop:8},frame:{width:260,height:260,borderWidth:2,borderRadius:16,backgroundColor:'transparent',alignItems:'center',justifyContent:'center'},bottom:{minHeight:90,alignItems:'center'},scanError:{color:'#FFF',fontSize:14,backgroundColor:'rgba(160,35,24,.9)',paddingHorizontal:12,paddingVertical:8,borderRadius:8,marginBottom:12},close:{minWidth:120,minHeight:48,borderRadius:10,backgroundColor:'rgba(0,0,0,.72)',alignItems:'center',justifyContent:'center'},closeText:{color:'#FFF',fontSize:16,fontWeight:'600'}});
