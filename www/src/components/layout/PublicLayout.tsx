import { AdminPanelSettings, CloudUpload, Home, Login, Menu as MenuIcon } from '@mui/icons-material';
import { AppBar, Box, Button, Container, IconButton, Link, Menu, MenuItem, Stack, Toolbar, Typography } from '@mui/material';
import { useEffect, useMemo, useState } from 'react';
import { Outlet, useLocation, useNavigate } from 'react-router-dom';
import { getSiteSettings, type SiteSettings } from '../../api/client';
import { getCachedMe, refreshMe, subscribeAuthState } from '../../api/authState';
import { useThemePreference } from '../../theme/ThemePreferenceProvider';

const defaultSite: SiteSettings = {
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

type PublicLayoutHeaderProps = {
  authenticated: boolean;
  hideAdminEntry?: boolean;
  site: SiteSettings;
};

export function PublicLayout() {
  const location = useLocation();
  const { applySiteSettings } = useThemePreference();
  const [site, setSite] = useState<SiteSettings>(defaultSite);
  const [authenticated, setAuthenticated] = useState(() => getCachedMe().authenticated);

  useEffect(() => {
    getSiteSettings()
      .then((payload) => {
        const nextSite = { ...defaultSite, ...payload };
        setSite(nextSite);
        applySiteSettings(nextSite);
      })
      .catch(() => setSite(defaultSite));
  }, [applySiteSettings]);

  useEffect(() => {
    const unsubscribe = subscribeAuthState((payload) => setAuthenticated(payload.authenticated));
    refreshMe().catch(() => undefined);
    return unsubscribe;
  }, []);

  useEffect(() => {
    document.title = site.name || defaultSite.name;
  }, [site.name]);

  const centered = useMemo(() => location.pathname === '/login', [location.pathname]);

  return (
    <Box sx={{ bgcolor: 'background.default', display: 'flex', flexDirection: 'column', minHeight: '100vh' }}>
      <PublicLayoutHeader authenticated={authenticated} site={site} />
      <Box
        component="main"
        sx={(theme) => ({
          alignItems: centered ? 'center' : 'flex-start',
          ...publicBackgroundSx(site, theme),
          display: 'flex',
          flex: 1,
          justifyContent: 'center',
          mt: 8,
          p: { xs: 2, sm: 3 }
        })}
      >
        <Container maxWidth={centered ? 'sm' : 'xl'} sx={{ py: { xs: 2, md: 4 }, width: '100%' }}>
          <Outlet />
        </Container>
      </Box>
      <PublicLayoutFooter site={site} />
    </Box>
  );
}

function publicBackgroundSx(site: SiteSettings, theme: import('@mui/material/styles').Theme) {
  const desktop = cssBackgroundURL(site.publicBackgroundDesktopUrl);
  const mobile = cssBackgroundURL(site.publicBackgroundMobileUrl || site.publicBackgroundDesktopUrl);
  if (!desktop && !mobile) {
    return { bgcolor: 'background.default' };
  }
  const isDark = theme.palette.mode === 'dark';
  return {
    bgcolor: 'background.default',
    position: 'relative',
    '&::before': {
      backgroundImage: { xs: mobile || desktop || 'none', md: desktop || mobile || 'none' },
      backgroundPosition: 'center',
      backgroundRepeat: 'no-repeat',
      backgroundSize: 'cover',
      bottom: 0,
      content: '""',
      left: 0,
      pointerEvents: 'none',
      position: 'fixed',
      right: 0,
      top: 64,
      zIndex: 0
    },
    '& > .MuiContainer-root': {
      position: 'relative',
      zIndex: 1
    },
    '& .MuiPaper-root, & .MuiCard-root': {
      backgroundColor: isDark ? 'rgba(15, 23, 42, 0.82)' : 'rgba(255, 255, 255, 0.82)',
      borderColor: isDark ? 'rgba(226, 232, 240, 0.22)' : 'rgba(255, 255, 255, 0.55)',
      boxShadow: 'none'
    },
    '& .MuiPaper-root .MuiPaper-root, & .MuiCard-root .MuiPaper-root': {
      backgroundColor: isDark ? 'rgba(15, 23, 42, 0.72)' : 'rgba(255, 255, 255, 0.72)'
    }
  };
}

function cssBackgroundURL(value: string) {
  const trimmed = value.trim();
  if (!trimmed) return '';
  return `url("${trimmed.replace(/\\/g, '\\\\').replace(/"/g, '\\"')}")`;
}

export function PublicLayoutHeader({ authenticated, hideAdminEntry = false, site }: PublicLayoutHeaderProps) {
  const location = useLocation();
  const navigate = useNavigate();
  const [menuAnchor, setMenuAnchor] = useState<null | HTMLElement>(null);
  const navItems = [
    { label: '首页', path: '/', icon: <Home /> },
    { label: '返图', path: '/submit', icon: <CloudUpload /> }
  ];
  const adminTarget = authenticated ? '/admin/events' : '/login';

  function go(path: string) {
    navigate(path);
    setMenuAnchor(null);
  }

  return (
    <AppBar
      elevation={0}
      position="fixed"
      sx={{
        bgcolor: 'background.paper',
        borderBottom: '1px solid',
        borderColor: 'divider',
        color: 'text.primary',
        zIndex: (theme) => theme.zIndex.drawer + 1
      }}
    >
      <Toolbar sx={{ gap: { xs: 1, sm: 3, md: 4 }, height: 64, px: { xs: 1.5, sm: 4, md: 5 } }}>
        <Stack
          direction="row"
          onClick={() => navigate('/')}
          sx={{ alignItems: 'center', cursor: 'pointer', flexShrink: 0, gap: { xs: 1.25, sm: 2 }, minWidth: 0 }}
        >
          {site.logoUrl ? (
            <Box
              alt={site.name}
              component="img"
              src={site.logoUrl}
              sx={{ height: { xs: 36, sm: 44 }, maxWidth: { xs: 120, sm: 200 }, objectFit: 'contain' }}
            />
          ) : (
            <Typography color="primary" noWrap sx={{ fontSize: { xs: '1rem', sm: '1.25rem' }, fontWeight: 600 }} variant="h6">
              {site.name || defaultSite.name}
            </Typography>
          )}
          {site.logoUrl && (
            <Box sx={{ minWidth: 0 }}>
              <Typography color="primary" noWrap sx={{ fontWeight: 600, lineHeight: 1.15 }} variant="h6">
                {site.name || defaultSite.name}
              </Typography>
              <Typography color="text.secondary" noWrap sx={{ display: { xs: 'none', md: 'block' } }} variant="caption">
                {site.subtitle || defaultSite.subtitle}
              </Typography>
            </Box>
          )}
          {!site.logoUrl && (
            <Typography color="text.secondary" noWrap sx={{ display: { xs: 'none', sm: 'block' } }} variant="caption">
              {site.subtitle || defaultSite.subtitle}
            </Typography>
          )}
        </Stack>
        <Box sx={{ display: { xs: 'none', sm: 'flex' }, gap: 1, ml: { sm: 2.5, md: 4 } }}>
          {navItems.map((item) => (
            <Button
              key={item.path + item.label}
              onClick={() => go(item.path)}
              startIcon={item.icon}
              sx={{
                bgcolor: location.pathname === item.path ? 'action.selected' : 'transparent',
                color: location.pathname === item.path ? 'primary.main' : 'text.secondary',
                minWidth: 88,
                px: 2,
                '&:hover': { bgcolor: 'action.hover', color: 'primary.main' }
              }}
            >
              {item.label}
            </Button>
          ))}
        </Box>
        <Box sx={{ flex: 1 }} />
        {!hideAdminEntry && (
          <Button
            onClick={() => go(adminTarget)}
            startIcon={authenticated ? <AdminPanelSettings /> : <Login />}
            sx={{ display: { xs: 'none', sm: 'inline-flex' }, minWidth: 88, px: 2.25 }}
            variant={authenticated ? 'contained' : 'text'}
          >
            {authenticated ? '后台' : '登录'}
          </Button>
        )}
        <IconButton aria-label="打开菜单" color="inherit" onClick={(event) => setMenuAnchor(event.currentTarget)} sx={{ display: { xs: 'flex', sm: 'none' } }}>
          <MenuIcon />
        </IconButton>
      </Toolbar>
      <Menu anchorEl={menuAnchor} onClose={() => setMenuAnchor(null)} open={Boolean(menuAnchor)} sx={{ display: { xs: 'block', sm: 'none' } }}>
        {navItems.map((item) => (
          <MenuItem key={item.path + item.label} onClick={() => go(item.path)} selected={location.pathname === item.path} sx={{ gap: 1.5 }}>
            {item.icon}
            {item.label}
          </MenuItem>
        ))}
        {!hideAdminEntry && (
          <MenuItem onClick={() => go(adminTarget)} sx={{ gap: 1.5 }}>
            {authenticated ? <AdminPanelSettings /> : <Login />}
            {authenticated ? '后台' : '登录'}
          </MenuItem>
        )}
      </Menu>
    </AppBar>
  );
}

function PublicLayoutFooter({ site }: { site: SiteSettings }) {
  const footerItems = footerLinks(site);
  const footerText = site.footerText || defaultSite.footerText;
  if (!footerText && footerItems.length === 0) {
    return null;
  }

  return (
    <Box
      component="footer"
      sx={{
        bgcolor: '#030712',
        borderTop: '1px solid',
        borderColor: 'rgba(255, 255, 255, 0.12)',
        color: 'rgba(255, 255, 255, 0.76)',
        px: { xs: 2, sm: 3 },
        py: 2.5
      }}
    >
      <Container maxWidth="xl">
        <Stack
          direction={{ xs: 'column', md: 'row' }}
          sx={{ alignItems: { xs: 'flex-start', md: 'center' }, gap: 1.25, justifyContent: 'space-between' }}
        >
          {footerText && (
            <Typography sx={{ color: 'rgba(255, 255, 255, 0.76)' }} variant="body2">
              {footerText}
            </Typography>
          )}
          {!!footerItems.length && (
            <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 1.5 }}>
              {footerItems.map((item) => item.href ? (
                <Link
                  href={item.href}
                  key={item.label + item.href}
                  rel="noopener noreferrer"
                  sx={{ color: 'rgba(255, 255, 255, 0.76)', '&:hover': { color: '#ffffff' } }}
                  target="_blank"
                  underline="hover"
                  variant="body2"
                >
                  {item.label}
                </Link>
              ) : (
                <Typography key={item.label} sx={{ color: 'rgba(255, 255, 255, 0.76)' }} variant="body2">
                  {item.label}
                </Typography>
              ))}
            </Stack>
          )}
        </Stack>
      </Container>
    </Box>
  );
}

function footerLinks(site: SiteSettings) {
  const items: { label: string; href?: string }[] = [];
  if (site.icpNumber) {
    items.push({ label: site.icpNumber, href: 'https://beian.miit.gov.cn/' });
  }
  if (site.policeRecordNumber) {
    items.push({ label: site.policeRecordNumber, href: safeExternalURL(site.policeRecordUrl) });
  }
  if (site.contactEmail) {
    items.push({ label: site.contactText || site.contactEmail, href: `mailto:${site.contactEmail}` });
  } else if (site.contactUrl) {
    items.push({ label: site.contactText || '联系方式', href: safeExternalURL(site.contactUrl) });
  } else if (site.contactText) {
    items.push({ label: site.contactText });
  }
  return items;
}

function safeExternalURL(value: string) {
  const trimmed = value.trim();
  if (!trimmed) return '';
  try {
    const parsed = new URL(trimmed);
    return parsed.protocol === 'http:' || parsed.protocol === 'https:' ? parsed.toString() : '';
  } catch {
    return '';
  }
}
