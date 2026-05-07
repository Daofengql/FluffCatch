import { CssBaseline } from '@mui/material';
import { ThemeProvider, useColorScheme } from '@mui/material/styles';
import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import { getSiteSettings, type SiteSettings } from '../api/client';
import { appPalettes, createAppTheme, normalizePaletteKey, normalizeThemeColor, type AppPaletteKey } from './theme';

export type AppThemeMode = 'system' | 'light' | 'dark';

type SiteThemeSettings = {
  mode: AppThemeMode;
  paletteKey: AppPaletteKey;
  primaryColor: string;
};

type ThemePreferenceContextValue = SiteThemeSettings & {
  applySiteSettings: (site: Partial<SiteSettings>) => void;
  reloadSiteTheme: () => Promise<void>;
  palettes: typeof appPalettes;
};

const defaultSiteTheme: SiteThemeSettings = {
  mode: 'system',
  paletteKey: 'blue',
  primaryColor: '#2563eb'
};

const ThemePreferenceContext = createContext<ThemePreferenceContextValue | null>(null);

export function ThemePreferenceProvider({ children }: { children: ReactNode }) {
  const [siteTheme, setSiteTheme] = useState<SiteThemeSettings>(defaultSiteTheme);
  const theme = useMemo(() => createAppTheme(siteTheme.paletteKey, siteTheme.primaryColor), [siteTheme.paletteKey, siteTheme.primaryColor]);

  const applySiteSettings = useCallback((site: Partial<SiteSettings>) => {
    const nextTheme = resolveSiteTheme(site);
    setSiteTheme((prev) => (
      prev.mode === nextTheme.mode &&
      prev.paletteKey === nextTheme.paletteKey &&
      prev.primaryColor === nextTheme.primaryColor
        ? prev
        : nextTheme
    ));
  }, []);

  const reloadSiteTheme = useCallback(async () => {
    const site = await getSiteSettings();
    applySiteSettings(site);
  }, [applySiteSettings]);

  useEffect(() => {
    reloadSiteTheme().catch(() => undefined);
  }, [reloadSiteTheme]);

  const contextValue = useMemo(
    () => ({ ...siteTheme, applySiteSettings, palettes: appPalettes, reloadSiteTheme }),
    [applySiteSettings, reloadSiteTheme, siteTheme]
  );

  return (
    <ThemePreferenceContext.Provider value={contextValue}>
      <ThemeProvider defaultMode={siteTheme.mode} disableTransitionOnChange storageManager={null} theme={theme}>
        <ThemeModeSync mode={siteTheme.mode} />
        <CssBaseline enableColorScheme />
        {children}
      </ThemeProvider>
    </ThemePreferenceContext.Provider>
  );
}

function ThemeModeSync({ mode }: { mode: AppThemeMode }) {
  const { setMode } = useColorScheme();

  useEffect(() => {
    setMode(mode);
  }, [mode, setMode]);

  return null;
}

function resolveSiteTheme(site: Partial<SiteSettings>): SiteThemeSettings {
  return {
    mode: normalizeThemeMode(site.themeMode),
    paletteKey: normalizePaletteKey(site.themePreset),
    primaryColor: normalizeThemeColor(site.themePrimaryColor)
  };
}

function normalizeThemeMode(mode: string | null | undefined): AppThemeMode {
  return mode === 'light' || mode === 'dark' || mode === 'system' ? mode : 'system';
}

export function useThemePreference() {
  const context = useContext(ThemePreferenceContext);
  if (!context) {
    throw new Error('useThemePreference must be used inside ThemePreferenceProvider');
  }
  return context;
}
