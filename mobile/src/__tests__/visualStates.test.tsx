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
    const provider={provider:'opencode',available:true,observedAt:new Date(now).toISOString(),tokens:{inputTokens:20,outputTokens:5,cachedInputTokens:4,totalTokens:25}};
    const screen=await render(<ProviderSection provider={provider} theme={theme} now={now}/>);
    expect(screen.getByText('OpenCode')).toBeTruthy();
    expect(screen.getByText('Consumo disponível no resumo de tokens.')).toBeTruthy();
    expect(screen.queryByText('Janela ainda não informada pelo provider')).toBeNull();
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
