import { createTheme } from '@mui/material/styles';

export const theme = createTheme({
  palette: {
    mode: 'light',
    primary: {
      main: '#2563eb',
      light: '#3b82f6',
      dark: '#1d4ed8'
    },
    secondary: {
      main: '#6b7280',
      light: '#9ca3af',
      dark: '#4b5563'
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
      default: '#f9fafb',
      paper: '#ffffff'
    },
    text: {
      primary: '#1f2937',
      secondary: '#6b7280'
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
        contained: {
          boxShadow: '0 2px 4px 0 rgba(0, 0, 0, 0.1)',
          '&:hover': { boxShadow: '0 4px 8px 0 rgba(0, 0, 0, 0.12)' }
        }
      }
    },
    MuiCard: {
      styleOverrides: {
        root: {
          border: '1px solid #e5e7eb',
          boxShadow: '0 2px 8px 0 rgba(0, 0, 0, 0.08), 0 1px 3px 0 rgba(0, 0, 0, 0.04)'
        }
      }
    },
    MuiPaper: {
      styleOverrides: {
        root: {
          backgroundImage: 'none'
        }
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
        root: { '&:hover': { backgroundColor: 'rgba(0, 0, 0, 0.04)' } }
      }
    },
    MuiDrawer: {
      styleOverrides: {
        paper: {
          borderRight: '1px solid #e5e7eb',
          boxShadow: 'none'
        }
      }
    },
    MuiAppBar: {
      styleOverrides: {
        root: {
          backgroundColor: '#ffffff',
          boxShadow: '0 1px 3px 0 rgba(0, 0, 0, 0.08)',
          color: '#1f2937'
        }
      }
    },
    MuiTableCell: {
      styleOverrides: {
        head: {
          backgroundColor: '#f9fafb',
          color: '#374151',
          fontWeight: 600
        }
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
