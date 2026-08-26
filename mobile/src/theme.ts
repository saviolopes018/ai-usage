import { ColorSchemeName } from 'react-native';

export type Theme = ReturnType<typeof createTheme>;

export function createTheme(scheme: ColorSchemeName) {
  const dark = scheme === 'dark';
  return {
    dark,
    colors: dark
      ? { bg: '#111312', surface: '#1A1E1D', surfaceRaised: '#252B29', surfacePressed: '#303735', ink: '#F2F6F4', muted: '#A8B2AE', subtle: '#78827E', line: '#343B38', primary: '#5CC6B7', primarySoft: '#173B36', primaryInk: '#081D1A', codex: '#5CC6B7', codexSoft: '#173B36', claude: '#F08A72', claudeSoft: '#43231F', opencode: '#9DA7FF', opencodeSoft: '#282B50', warning: '#E2AA51', warningSoft: '#3C2C13', error: '#F17C70', errorSoft: '#401F1C', success: '#68C79A', input: '#171A19', nav: '#171A19', overlay: 'rgba(0,0,0,.58)', onAccent: '#071B18' }
      : { bg: '#FBFCFC', surface: '#F0F4F2', surfaceRaised: '#E5ECE9', surfacePressed: '#D9E3DF', ink: '#17201D', muted: '#5E6C67', subtle: '#7B8984', line: '#D2DCD8', primary: '#287F74', primarySoft: '#D9EFEB', primaryInk: '#12554D', codex: '#287F74', codexSoft: '#D9EFEB', claude: '#C65F49', claudeSoft: '#F7E2DD', opencode: '#5965B8', opencodeSoft: '#E5E7FA', warning: '#96620E', warningSoft: '#FAEDCF', error: '#B74438', errorSoft: '#F8DFDC', success: '#277C58', input: '#FFFFFF', nav: '#FFFFFF', overlay: 'rgba(7,20,16,.48)', onAccent: '#FFFFFF' },
    space: { xs: 4, sm: 8, md: 12, lg: 20, xl: 28, xxl: 40 },
    radius: { sm: 8, md: 12, pill: 999 },
    type: { caption: 12, body: 14, control: 15, section: 17, title: 28 },
    touch: { min: 44, control: 52 },
    motion: { fast: 180, normal: 220 },
  };
}
