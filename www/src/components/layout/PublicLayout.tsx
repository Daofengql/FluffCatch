import { AdminPanelSettings, Home, Login, Menu as MenuIcon } from '@mui/icons-material';
import { AppBar, Box, Button, Container, IconButton, Menu, MenuItem, Stack, Toolbar, Typography } from '@mui/material';
import { useEffect, useMemo, useState } from 'react';
import { Outlet, useLocation, useNavigate } from 'react-router-dom';
import { getMe, getSiteSettings, type SiteSettings } from '../../api/client';

const defaultSite: SiteSettings = {
  name: 'FluffCatch',
  subtitle: '兽聚返图收集与画廊',
  logoUrl: ''
};

type PublicLayoutHeaderProps = {
  authenticated: boolean;
  site: SiteSettings;
};

export function PublicLayout() {
  const location = useLocation();
  const [site, setSite] = useState<SiteSettings>(defaultSite);
  const [authenticated, setAuthenticated] = useState(false);

  useEffect(() => {
    getSiteSettings()
      .then((payload) => setSite({ ...defaultSite, ...payload }))
      .catch(() => setSite(defaultSite));
  }, []);

  useEffect(() => {
    getMe()
      .then((payload) => setAuthenticated(payload.authenticated))
      .catch(() => setAuthenticated(false));
  }, [location.pathname]);

  useEffect(() => {
    document.title = site.name || defaultSite.name;
  }, [site.name]);

  const centered = useMemo(() => location.pathname === '/login' || /^\/events\/[^/]+\/submit$/.test(location.pathname), [location.pathname]);

  return (
    <Box sx={{ bgcolor: 'background.default', display: 'flex', flexDirection: 'column', minHeight: '100vh' }}>
      <PublicLayoutHeader authenticated={authenticated} site={site} />
      <Box
        component="main"
        sx={{
          alignItems: centered ? 'center' : 'flex-start',
          display: 'flex',
          flex: 1,
          justifyContent: 'center',
          mt: 8,
          p: { xs: 2, sm: 3 }
        }}
      >
        <Container maxWidth={centered ? 'sm' : 'lg'} sx={{ py: { xs: 2, md: 4 }, width: '100%' }}>
          <Outlet />
        </Container>
      </Box>
    </Box>
  );
}

export function PublicLayoutHeader({ authenticated, site }: PublicLayoutHeaderProps) {
  const location = useLocation();
  const navigate = useNavigate();
  const [menuAnchor, setMenuAnchor] = useState<null | HTMLElement>(null);
  const navItems = [
    { label: '首页', path: '/', icon: <Home /> }
  ];
  const adminTarget = authenticated ? '/admin/dashboard' : '/login';

  function go(path: string) {
    navigate(path);
    setMenuAnchor(null);
  }

  return (
    <AppBar
      elevation={0}
      position="fixed"
      sx={{
        bgcolor: '#ffffff',
        borderBottom: '1px solid',
        borderColor: 'grey.200',
        color: 'text.primary',
        zIndex: (theme) => theme.zIndex.drawer + 1
      }}
    >
      <Toolbar sx={{ gap: { xs: 1, sm: 2 }, height: 64, px: { xs: 1.5, sm: 3 } }}>
        <Stack
          direction="row"
          onClick={() => navigate('/')}
          sx={{ alignItems: 'center', cursor: 'pointer', flexShrink: 0, gap: 1.5, minWidth: 0 }}
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
        <Box sx={{ display: { xs: 'none', sm: 'flex' }, gap: 0.5, ml: 2 }}>
          {navItems.map((item) => (
            <Button
              key={item.path + item.label}
              onClick={() => go(item.path)}
              startIcon={item.icon}
              sx={{
                bgcolor: location.pathname === item.path ? '#eff6ff' : 'transparent',
                color: location.pathname === item.path ? 'primary.main' : 'text.secondary',
                '&:hover': { bgcolor: 'action.hover', color: 'primary.main' }
              }}
            >
              {item.label}
            </Button>
          ))}
        </Box>
        <Box sx={{ flex: 1 }} />
        <Button
          onClick={() => go(adminTarget)}
          startIcon={authenticated ? <AdminPanelSettings /> : <Login />}
          sx={{ display: { xs: 'none', sm: 'inline-flex' } }}
          variant={authenticated ? 'contained' : 'text'}
        >
          {authenticated ? '后台' : '登录'}
        </Button>
        <IconButton onClick={(event) => setMenuAnchor(event.currentTarget)} sx={{ display: { xs: 'flex', sm: 'none' } }}>
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
        <MenuItem onClick={() => go(adminTarget)} sx={{ gap: 1.5 }}>
          {authenticated ? <AdminPanelSettings /> : <Login />}
          {authenticated ? '后台' : '登录'}
        </MenuItem>
      </Menu>
    </AppBar>
  );
}
