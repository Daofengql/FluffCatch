import { createTheme } from '@mui/material/styles';

export type AppPaletteKey = 'blue' | 'emerald' | 'rose' | 'amber' | 'violet' | 'custom';

type PalettePreset = {
  key: AppPaletteKey;
  label: string;
  swatch: string;
  light: {
    primary: string;
    primaryLight: string;
    primaryDark: string;
    selected: string;
    hover: string;
    focus: string;
  };
  dark: {
    primary: string;
    primaryLight: string;
    primaryDark: string;
    selected: string;
    focus: string;
  };
};

export const appPalettes: PalettePreset[] = [
  {
    key: 'blue',
    label: '晴蓝',
    swatch: '#2563eb',
    light: {
      primary: '#2563eb',
      primaryLight: '#3b82f6',
      primaryDark: '#1d4ed8',
      hover: 'rgba(37, 99, 235, 0.07)',
      selected: 'rgba(37, 99, 235, 0.13)',
      focus: 'rgba(37, 99, 235, 0.2)'
    },
    dark: {
      primary: '#60a5fa',
      primaryLight: '#93c5fd',
      primaryDark: '#3b82f6',
      selected: 'rgba(96, 165, 250, 0.22)',
      focus: 'rgba(96, 165, 250, 0.3)'
    }
  },
  {
    key: 'emerald',
    label: '松绿',
    swatch: '#059669',
    light: {
      primary: '#059669',
      primaryLight: '#10b981',
      primaryDark: '#047857',
      hover: 'rgba(5, 150, 105, 0.08)',
      selected: 'rgba(5, 150, 105, 0.15)',
      focus: 'rgba(5, 150, 105, 0.22)'
    },
    dark: {
      primary: '#34d399',
      primaryLight: '#6ee7b7',
      primaryDark: '#10b981',
      selected: 'rgba(52, 211, 153, 0.22)',
      focus: 'rgba(52, 211, 153, 0.3)'
    }
  },
  {
    key: 'rose',
    label: '绯红',
    swatch: '#e11d48',
    light: {
      primary: '#e11d48',
      primaryLight: '#f43f5e',
      primaryDark: '#be123c',
      hover: 'rgba(225, 29, 72, 0.08)',
      selected: 'rgba(225, 29, 72, 0.15)',
      focus: 'rgba(225, 29, 72, 0.22)'
    },
    dark: {
      primary: '#fb7185',
      primaryLight: '#fda4af',
      primaryDark: '#f43f5e',
      selected: 'rgba(251, 113, 133, 0.24)',
      focus: 'rgba(251, 113, 133, 0.32)'
    }
  },
  {
    key: 'amber',
    label: '琥珀',
    swatch: '#d97706',
    light: {
      primary: '#d97706',
      primaryLight: '#f59e0b',
      primaryDark: '#b45309',
      hover: 'rgba(217, 119, 6, 0.08)',
      selected: 'rgba(217, 119, 6, 0.16)',
      focus: 'rgba(217, 119, 6, 0.24)'
    },
    dark: {
      primary: '#fbbf24',
      primaryLight: '#fde68a',
      primaryDark: '#f59e0b',
      selected: 'rgba(251, 191, 36, 0.23)',
      focus: 'rgba(251, 191, 36, 0.32)'
    }
  },
  {
    key: 'violet',
    label: '鸢紫',
    swatch: '#7c3aed',
    light: {
      primary: '#7c3aed',
      primaryLight: '#8b5cf6',
      primaryDark: '#6d28d9',
      hover: 'rgba(124, 58, 237, 0.08)',
      selected: 'rgba(124, 58, 237, 0.15)',
      focus: 'rgba(124, 58, 237, 0.22)'
    },
    dark: {
      primary: '#a78bfa',
      primaryLight: '#c4b5fd',
      primaryDark: '#8b5cf6',
      selected: 'rgba(167, 139, 250, 0.24)',
      focus: 'rgba(167, 139, 250, 0.32)'
    }
  }
];

export function normalizePaletteKey(value: string | null | undefined): AppPaletteKey {
  return value === 'custom' || appPalettes.some((item) => item.key === value) ? (value as AppPaletteKey) : 'blue';
}

export function normalizeThemeColor(value: string | null | undefined) {
  const color = (value || '').trim().toLowerCase();
  return /^#[0-9a-f]{6}$/.test(color) ? color : '#2563eb';
}

function customPalette(primary: string): PalettePreset {
  return {
    key: 'custom',
    label: '自定义',
    swatch: primary,
    light: {
      primary,
      primaryLight: mixColor(primary, '#ffffff', 0.18),
      primaryDark: mixColor(primary, '#000000', 0.18),
      hover: withAlpha(primary, 0.08),
      selected: withAlpha(primary, 0.15),
      focus: withAlpha(primary, 0.22)
    },
    dark: {
      primary: mixColor(primary, '#ffffff', 0.38),
      primaryLight: mixColor(primary, '#ffffff', 0.55),
      primaryDark: primary,
      selected: withAlpha(mixColor(primary, '#ffffff', 0.38), 0.24),
      focus: withAlpha(mixColor(primary, '#ffffff', 0.38), 0.32)
    }
  };
}

function withAlpha(hex: string, alpha: number) {
  const rgb = hexToRgb(hex);
  return `rgba(${rgb.r}, ${rgb.g}, ${rgb.b}, ${alpha})`;
}

function mixColor(hex: string, targetHex: string, amount: number) {
  const color = hexToRgb(hex);
  const target = hexToRgb(targetHex);
  const mixed = {
    r: Math.round(color.r + (target.r - color.r) * amount),
    g: Math.round(color.g + (target.g - color.g) * amount),
    b: Math.round(color.b + (target.b - color.b) * amount)
  };
  return `#${[mixed.r, mixed.g, mixed.b].map((channel) => channel.toString(16).padStart(2, '0')).join('')}`;
}

function hexToRgb(hex: string) {
  const normalized = normalizeThemeColor(hex).slice(1);
  return {
    r: parseInt(normalized.slice(0, 2), 16),
    g: parseInt(normalized.slice(2, 4), 16),
    b: parseInt(normalized.slice(4, 6), 16)
  };
}

export function createAppTheme(paletteKey: AppPaletteKey = 'blue', customPrimaryColor = '#2563eb') {
  const preset = paletteKey === 'custom' ? customPalette(normalizeThemeColor(customPrimaryColor)) : appPalettes.find((item) => item.key === paletteKey) ?? appPalettes[0];

  return createTheme({
    cssVariables: {
      colorSchemeSelector: 'data'
    },
    colorSchemes: {
      light: {
        palette: {
          mode: 'light',
          primary: {
            main: preset.light.primary,
            light: preset.light.primaryLight,
            dark: preset.light.primaryDark
          },
          secondary: {
            main: '#475569',
            light: '#64748b',
            dark: '#334155'
          },
          error: {
            main: '#dc2626'
          },
          warning: {
            main: '#ca8a04'
          },
          success: {
            main: '#16a34a'
          },
          info: {
            main: '#0284c7'
          },
          background: {
            default: '#f8fafc',
            paper: '#ffffff'
          },
          text: {
            primary: '#111827',
            secondary: '#475569'
          },
          divider: '#d8dee8',
          action: {
            hover: preset.light.hover,
            selected: preset.light.selected,
            focus: preset.light.focus
          }
        }
      },
      dark: {
        palette: {
          mode: 'dark',
          primary: {
            main: preset.dark.primary,
            light: preset.dark.primaryLight,
            dark: preset.dark.primaryDark,
            contrastText: '#07111f'
          },
          secondary: {
            main: '#cbd5e1',
            light: '#e2e8f0',
            dark: '#94a3b8'
          },
          error: {
            main: '#f87171'
          },
          warning: {
            main: '#facc15'
          },
          success: {
            main: '#4ade80'
          },
          info: {
            main: '#38bdf8'
          },
          background: {
            default: '#070b14',
            paper: '#101826'
          },
          text: {
            primary: '#f8fafc',
            secondary: '#d5deeb'
          },
          divider: 'rgba(203, 213, 225, 0.22)',
          action: {
            hover: 'rgba(226, 232, 240, 0.1)',
            selected: preset.dark.selected,
            focus: preset.dark.focus
          }
        }
      }
    },
    shape: {
      borderRadius: 8
    },
    typography: {
      fontFamily: [
        'Inter',
        '-apple-system',
        'BlinkMacSystemFont',
        '"Segoe UI"',
        'Roboto',
        '"Helvetica Neue"',
        'Arial',
        'sans-serif',
        '"Apple Color Emoji"',
        '"Segoe UI Emoji"',
        '"Segoe UI Symbol"'
      ].join(','),
      h4: {
        fontWeight: 600
      },
      h5: {
        fontWeight: 600
      },
      h6: {
        fontWeight: 600
      },
      button: {
        fontWeight: 500,
        textTransform: 'none'
      }
    },
    components: {
      MuiButton: {
        styleOverrides: {
          root: { boxShadow: 'none' },
          contained: ({ theme }) => ({
            boxShadow: theme.palette.mode === 'dark' ? '0 10px 22px rgba(0, 0, 0, 0.35)' : '0 2px 4px 0 rgba(0, 0, 0, 0.1)',
            '&:hover': {
              boxShadow: theme.palette.mode === 'dark' ? '0 14px 28px rgba(0, 0, 0, 0.42)' : '0 4px 8px 0 rgba(0, 0, 0, 0.12)'
            }
          })
        }
      },
      MuiCard: {
        styleOverrides: {
          root: ({ theme }) => ({
            border: '1px solid',
            borderColor: theme.palette.divider,
            boxShadow:
              theme.palette.mode === 'dark'
                ? '0 18px 42px rgba(0, 0, 0, 0.38), 0 0 0 1px rgba(255, 255, 255, 0.02)'
                : '0 2px 8px 0 rgba(0, 0, 0, 0.08), 0 1px 3px 0 rgba(0, 0, 0, 0.04)'
          })
        }
      },
      MuiPaper: {
        styleOverrides: {
          root: ({ theme }) => ({
            backgroundImage: 'none',
            borderColor: theme.palette.divider
          })
        }
      },
      MuiTextField: {
        defaultProps: { size: 'small', variant: 'outlined' }
      },
      MuiSelect: {
        defaultProps: { size: 'small', variant: 'outlined' }
      },
      MuiIconButton: {
        styleOverrides: {
          root: ({ theme }) => ({ '&:hover': { backgroundColor: theme.palette.action.hover } })
        }
      },
      MuiDrawer: {
        styleOverrides: {
          paper: ({ theme }) => ({
            borderRight: '1px solid',
            borderColor: theme.palette.divider,
            boxShadow: 'none'
          })
        }
      },
      MuiAppBar: {
        styleOverrides: {
          root: ({ theme }) => ({
            backgroundColor: theme.palette.background.paper,
            boxShadow: theme.palette.mode === 'dark' ? '0 1px 0 rgba(255, 255, 255, 0.08)' : '0 1px 3px 0 rgba(15, 23, 42, 0.08)',
            color: theme.palette.text.primary
          })
        }
      },
      MuiTableCell: {
        styleOverrides: {
          head: ({ theme }) => ({
            backgroundColor: theme.palette.action.hover,
            color: theme.palette.text.primary,
            fontWeight: 600
          })
        }
      },
      MuiDialog: {
        styleOverrides: {
          paper: {
            borderRadius: 12
          }
        }
      }
    }
  });
}

export const theme = createAppTheme();
