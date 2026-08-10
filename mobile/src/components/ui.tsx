import { Ionicons } from '@expo/vector-icons';
import { ActivityIndicator, Pressable, StyleSheet, Text, View } from 'react-native';
import { Theme } from '../theme';

type IconName = React.ComponentProps<typeof Ionicons>['name'];

export function Icon({name,size=20,color}:{name:IconName;size?:number;color:string}) {
  return <Ionicons name={name} size={size} color={color}/>;
}

export function ScreenHeading({title,description,theme,action}:{title:string;description?:string;theme:Theme;action?:React.ReactNode}) {
  return <View style={styles.heading}><View style={styles.headingCopy}><Text style={[styles.title,{color:theme.colors.ink}]}>{title}</Text>{description&&<Text style={[styles.description,{color:theme.colors.muted}]}>{description}</Text>}</View>{action}</View>;
}

export function SectionHeading({children,theme}:{children:React.ReactNode;theme:Theme}) {
  return <Text style={[styles.sectionTitle,{color:theme.colors.ink}]}>{children}</Text>;
}

export function ActionButton({label,icon,theme,onPress,variant='secondary',disabled=false,loading=false}:{label:string;icon?:IconName;theme:Theme;onPress:()=>void;variant?:'primary'|'secondary'|'danger'|'quiet';disabled?:boolean;loading?:boolean}) {
  const primary=variant==='primary'; const danger=variant==='danger';
  const bg=primary?theme.colors.primary:danger?theme.colors.error:variant==='quiet'?'transparent':theme.colors.surface;
  const color=primary||danger?theme.colors.onAccent:theme.colors.ink;
  return <Pressable disabled={disabled||loading} onPress={onPress} accessibilityRole="button" accessibilityState={{disabled:disabled||loading,busy:loading}} style={({pressed})=>[styles.button,{backgroundColor:pressed&&!primary&&!danger?theme.colors.surfacePressed:bg,opacity:disabled?.45:pressed?.72:1}]}>{loading?<ActivityIndicator color={color}/>:<>{icon&&<Icon name={icon} size={18} color={color}/>}<Text style={[styles.buttonText,{color}]}>{label}</Text></>}</Pressable>;
}

export function StatusPill({label,tone,theme}:{label:string;tone:'success'|'warning'|'error'|'neutral';theme:Theme}) {
  const color=tone==='success'?theme.colors.success:tone==='warning'?theme.colors.warning:tone==='error'?theme.colors.error:theme.colors.muted;
  const backgroundColor=tone==='success'?theme.colors.primarySoft:tone==='warning'?theme.colors.warningSoft:tone==='error'?theme.colors.errorSoft:theme.colors.surface;
  return <View style={[styles.pill,{backgroundColor}]}><View style={[styles.pillDot,{backgroundColor:color}]}/><Text style={[styles.pillText,{color}]}>{label}</Text></View>;
}

export function InfoBanner({title,body,tone='neutral',theme}:{title?:string;body:string;tone?:'neutral'|'warning'|'error'|'accent';theme:Theme}) {
  const backgroundColor=tone==='warning'?theme.colors.warningSoft:tone==='error'?theme.colors.errorSoft:tone==='accent'?theme.colors.primarySoft:theme.colors.surface;
  const iconColor=tone==='warning'?theme.colors.warning:tone==='error'?theme.colors.error:tone==='accent'?theme.colors.primary:theme.colors.muted;
  const icon:IconName=tone==='error'?'alert-circle-outline':tone==='warning'?'warning-outline':tone==='accent'?'shield-checkmark-outline':'information-circle-outline';
  return <View style={[styles.banner,{backgroundColor}]}><Icon name={icon} size={20} color={iconColor}/><View style={styles.bannerCopy}>{title&&<Text style={[styles.bannerTitle,{color:theme.colors.ink}]}>{title}</Text>}<Text style={[styles.bannerBody,{color:theme.colors.muted}]}>{body}</Text></View></View>;
}

const styles=StyleSheet.create({heading:{flexDirection:'row',alignItems:'flex-start',gap:16},headingCopy:{flex:1},title:{fontSize:28,fontWeight:'700',letterSpacing:-.6},description:{fontSize:14,lineHeight:20,marginTop:6,maxWidth:540},sectionTitle:{fontSize:17,fontWeight:'700',letterSpacing:-.2,marginTop:26,marginBottom:8},button:{minHeight:48,borderRadius:10,paddingHorizontal:16,flexDirection:'row',gap:8,alignItems:'center',justifyContent:'center'},buttonText:{fontSize:15,fontWeight:'700'},pill:{minHeight:28,borderRadius:999,paddingHorizontal:10,flexDirection:'row',alignItems:'center',gap:6,alignSelf:'flex-start'},pillDot:{width:6,height:6,borderRadius:3},pillText:{fontSize:12,fontWeight:'700'},banner:{padding:14,borderRadius:12,flexDirection:'row',alignItems:'flex-start',gap:10},bannerCopy:{flex:1},bannerTitle:{fontSize:14,fontWeight:'700',marginBottom:2},bannerBody:{fontSize:13,lineHeight:19}});
