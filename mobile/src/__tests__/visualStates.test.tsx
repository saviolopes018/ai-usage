import { fireEvent, render } from '@testing-library/react-native';
import { AppTab, CombinedTokens } from '../../App';
import { ProviderSection } from '../components/ProviderSection';
import { createTheme } from '../theme';

const theme=createTheme('light');

describe('visual states',()=>{
  it('exposes selected navigation state and handles presses',async()=>{
    const onPress=jest.fn();
    const screen=await render(<AppTab label="Monitor" icon="speedometer-outline" selectedIcon="speedometer" selected onPress={onPress} theme={theme}/>);
    const tab=screen.getByRole('tab',{name:'Monitor'});
    expect(tab.props.accessibilityState).toEqual({selected:true});
    fireEvent.press(tab);
    expect(onPress).toHaveBeenCalledTimes(1);
  });

  it('keeps unknown windows distinct from zero usage',async()=>{
    const screen=await render(<ProviderSection provider={{provider:'codex',available:true,observedAt:new Date().toISOString()}} theme={theme} now={Date.now()}/>);
    expect(screen.getAllByText('Sem dados')).toHaveLength(2);
    expect(screen.queryByText('0% usado')).toBeNull();
  });

  it('announces usage and remaining quota',async()=>{
    const now=Date.now();
    const screen=await render(<ProviderSection provider={{provider:'claude',available:true,observedAt:new Date(now).toISOString(),fiveHour:{usedPercentage:90,remainingPercentage:10,resetsAt:new Date(now+60_000).toISOString()}}} theme={theme} now={now}/>);
    expect(screen.getByLabelText(/5 horas: 90 por cento usado, 10 por cento restante/)).toBeTruthy();
    expect(screen.getByText('90% usado')).toBeTruthy();
  });

  it('shows stale and unavailable provider states in text',async()=>{
    const now=Date.now();
    const stale=await render(<ProviderSection provider={{provider:'codex',available:true,observedAt:new Date(now-16*60_000).toISOString()}} theme={theme} now={now}/>);
    expect(stale.getByText(/Leitura antiga/)).toBeTruthy();
    const unavailable=await render(<ProviderSection provider={{provider:'claude',available:false,observedAt:new Date(now).toISOString()}} theme={theme} now={now}/>);
    expect(unavailable.getByText('Provider indisponível')).toBeTruthy();
  });

  it('shows OpenCode token usage without quota window placeholders',async()=>{
    const now=Date.now();
    const provider={provider:'opencode',available:true,observedAt:new Date(now).toISOString(),tokens:{inputTokens:20,outputTokens:5,cachedInputTokens:4,totalTokens:25,periods:{'24h':{inputTokens:20,outputTokens:5,cachedInputTokens:4,totalTokens:25},'7d':{inputTokens:70,outputTokens:10,cachedInputTokens:8,totalTokens:80},'14d':{inputTokens:120,outputTokens:20,cachedInputTokens:12,totalTokens:140},'30d':{inputTokens:200,outputTokens:30,cachedInputTokens:20,totalTokens:230}}}};
    const screen=await render(<ProviderSection provider={provider} theme={theme} now={now}/>);
    expect(screen.getByText('OpenCode')).toBeTruthy();
    expect(screen.getByText('Consumo acumulado')).toBeTruthy();
    expect(screen.getByText('O OpenCode informa uso de tokens, não limites de sessão.')).toBeTruthy();
    expect(screen.getByLabelText('24 horas: 25 tokens')).toBeTruthy();
    expect(screen.getByLabelText('7 dias: 80 tokens')).toBeTruthy();
    expect(screen.getByLabelText('14 dias: 140 tokens')).toBeTruthy();
    expect(screen.getByLabelText('30 dias: 230 tokens')).toBeTruthy();
    expect(screen.queryByText('Janela ainda não informada pelo provider')).toBeNull();
  });

  it('does not show token period rows for quota providers',async()=>{
    const now=Date.now();
    const provider={provider:'codex',available:true,observedAt:new Date(now).toISOString(),tokens:{inputTokens:20,outputTokens:5,cachedInputTokens:4,totalTokens:25,periods:{'7d':{inputTokens:70,outputTokens:10,cachedInputTokens:8,totalTokens:80}}}};
    const screen=await render(<ProviderSection provider={provider} theme={theme} now={now}/>);
    expect(screen.queryByLabelText('7 dias: 80 tokens')).toBeNull();
  });

  it('announces missing OpenCode periods as unavailable instead of zero',async()=>{
    const now=Date.now();
    const provider={provider:'opencode',available:true,observedAt:new Date(now).toISOString(),tokens:{inputTokens:20,outputTokens:5,cachedInputTokens:4,totalTokens:25}};
    const screen=await render(<ProviderSection provider={provider} theme={theme} now={now}/>);
    expect(screen.getByLabelText('7 dias: sem dados')).toBeTruthy();
  });

  it('includes OpenCode in the combined token summary',async()=>{
    const providers=[
      {provider:'codex',available:true,observedAt:new Date().toISOString(),tokens:{inputTokens:10,outputTokens:2,cachedInputTokens:0,totalTokens:12}},
      {provider:'opencode',available:true,observedAt:new Date().toISOString(),tokens:{inputTokens:20,outputTokens:3,cachedInputTokens:0,totalTokens:23}},
    ];
    const screen=await render(<CombinedTokens providers={providers} theme={theme}/>);
    expect(screen.getByText('Codex + OpenCode · últimas 24h')).toBeTruthy();
    expect(screen.getByLabelText(/35 tokens gastos por Codex e OpenCode/)).toBeTruthy();
  });
});
