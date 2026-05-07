import { ArrowBack, ExitToApp, Folder, Settings } from '@mui/icons-material';
import {
  Avatar,
  Box,
  CircularProgress,
  Drawer,
  List,
  ListItem,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  Typography
} from '@mui/material';
import { useEffect, useState } from 'react';
import { Outlet, useLocation, useNavigate } from 'react-router-dom';
import { getSiteSettings, type SiteSettings } from '../../api/client';
import { getCachedMe, logoutAdmin, refreshMe, subscribeAuthState } from '../../api/authState';
import { useThemePreference } from '../../theme/ThemePreferenceProvider';
import { PublicLayoutHeader } from './PublicLayout';

const drawerWidth = 240;
const fallbackSite: SiteSettings = {
  name: 'FluffCatch',
  subtitle: '兽聚返图收集与画廊',
  logoUrl: '',
  homeMarkdown: '',
  themeMode: 'system',
  themePreset: 'blue',
  themePrimaryColor: '#2563eb',
  publicBackgroundDesktopUrl: '',
  publicBackgroundMobileUrl: '',
  footerText: `© ${new Date().getFullYear()} FluffCatch. All rights reserved.`,
  icpNumber: '',
  policeRecordNumber: '',
  policeRecordUrl: '',
  contactText: '',
  contactEmail: '',
  contactUrl: ''
};

const navItems = [
  { label: '兽聚管理', path: '/admin/events', icon: <Folder /> },
  { label: '系统设置', path: '/admin/settings', icon: <Settings /> }
];

export function AdminLayout() {
  const location = useLocation();
  const navigate = useNavigate();
  const { applySiteSettings } = useThemePreference();
  const [site, setSite] = useState<SiteSettings>(fallbackSite);
  const [checkingSession, setCheckingSession] = useState(() => !getCachedMe().authenticated);

  useEffect(() => {
    getSiteSettings()
      .then((payload) => {
        const nextSite = { ...fallbackSite, ...payload };
        setSite(nextSite);
        applySiteSettings(nextSite);
      })
      .catch(() => setSite(fallbackSite));
  }, [applySiteSettings]);

  useEffect(() => {
    let cancelled = false;

    const unsubscribe = subscribeAuthState((payload) => {
      if (!payload.authenticated) {
        navigate('/login', { replace: true });
        return;
      }
      setCheckingSession(false);
    });

    refreshMe()
      .then((payload) => {
        if (cancelled) return;
        if (!payload.authenticated) {
          navigate('/login', { replace: true });
          return;
        }
        setCheckingSession(false);
      })
      .catch(() => {
        if (!cancelled) {
          navigate('/login', { replace: true });
        }
      });

    return () => {
      cancelled = true;
      unsubscribe();
    };
  }, [navigate]);

  async function handleLogout() {
    await logoutAdmin().catch(() => undefined);
    navigate('/login', { replace: true });
  }

  const drawer = (
    <Box sx={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      <Box sx={{ borderBottom: '1px solid', borderColor: 'divider', p: 2 }}>
        <Box sx={{ alignItems: 'center', display: 'flex', gap: 1.5 }}>
          <Avatar
            src={site.logoUrl || undefined}
            sx={{ bgcolor: 'primary.main', color: 'primary.contrastText', fontWeight: 900, height: 38, width: 38 }}
            variant="rounded"
          >
            {(site.name || 'F').slice(0, 1).toUpperCase()}
          </Avatar>
          <Box sx={{ minWidth: 0 }}>
            <Typography color="primary" noWrap sx={{ fontWeight: 700 }} variant="h6">
              后台管理
            </Typography>
            <Typography color="text.secondary" noWrap variant="caption">
              {site.name || fallbackSite.name}
            </Typography>
          </Box>
        </Box>
      </Box>
      <List sx={{ flex: 1, py: 1 }}>
        {navItems.map((item) => (
          <ListItem disablePadding key={item.path} sx={{ mb: 0.5 }}>
            <ListItemButton
              onClick={() => navigate(item.path)}
              selected={location.pathname === item.path}
              sx={{
                borderRadius: 2,
                mx: 1,
                '&.Mui-selected': {
                  bgcolor: 'action.selected',
                  color: 'primary.main',
                  '& .MuiListItemIcon-root': { color: 'primary.main' }
                }
              }}
            >
              <ListItemIcon sx={{ minWidth: 40 }}>{item.icon}</ListItemIcon>
              <ListItemText primary={item.label} />
            </ListItemButton>
          </ListItem>
        ))}
      </List>
      <Box sx={{ borderTop: '1px solid', borderColor: 'divider', p: 2 }}>
        <ListItemButton onClick={() => navigate('/')} sx={{ borderRadius: 2, mb: 1 }}>
          <ListItemIcon sx={{ minWidth: 40 }}>
            <ArrowBack />
          </ListItemIcon>
          <ListItemText primary="返回主界面" />
        </ListItemButton>
        <ListItemButton onClick={() => void handleLogout()} sx={{ borderRadius: 2, color: 'error.main' }}>
          <ListItemIcon sx={{ color: 'error.main', minWidth: 40 }}>
            <ExitToApp />
          </ListItemIcon>
          <ListItemText primary="退出登录" />
        </ListItemButton>
      </Box>
    </Box>
  );

  if (checkingSession) {
    return (
      <Box sx={{ alignItems: 'center', bgcolor: 'background.default', display: 'flex', justifyContent: 'center', minHeight: '100vh' }}>
        <CircularProgress />
      </Box>
    );
  }

  return (
    <Box sx={{ bgcolor: 'background.default', display: 'flex', flexDirection: 'column', minHeight: '100vh' }}>
      <PublicLayoutHeader authenticated hideAdminEntry site={site} />
      <Box sx={{ display: 'flex', flex: 1, mt: 8 }}>
        <Drawer
          open
          sx={{
            display: { xs: 'none', md: 'block' },
            flexShrink: 0,
            whiteSpace: 'nowrap',
            width: drawerWidth,
            '& .MuiDrawer-paper': {
              bgcolor: 'background.paper',
              borderColor: 'divider',
              borderRight: '1px solid',
              boxSizing: 'border-box',
              height: 'calc(100vh - 64px)',
              top: 64,
              width: drawerWidth,
              zIndex: (theme) => theme.zIndex.drawer - 1
            }
          }}
          variant="permanent"
        >
          {drawer}
        </Drawer>
        <Box
          component="main"
          sx={{
            display: 'flex',
            flexDirection: 'column',
            flexGrow: 1,
            overflowX: 'hidden',
            width: { md: `calc(100% - ${drawerWidth}px)` }
          }}
        >
          <Box sx={{ minHeight: 'calc(100vh - 64px)', overflowX: 'hidden', p: { xs: 2, sm: 3 } }}>
            <Outlet />
          </Box>
        </Box>
      </Box>
    </Box>
  );
}
