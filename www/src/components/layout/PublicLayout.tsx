import { AdminPanelSettings, ChatBubbleOutlined, Close, CloudUpload, Home, Login, Menu as MenuIcon, Search } from '@mui/icons-material';
import { AppBar, Box, Button, CircularProgress, Container, Fab, IconButton, InputAdornment, Menu, MenuItem, Paper, Stack, TextField, Toolbar, Tooltip, Typography } from '@mui/material';
import type { FocusEvent, MouseEvent } from 'react';
import { useEffect, useMemo, useState } from 'react';
import { createPortal } from 'react-dom';
import { Outlet, useLocation, useNavigate } from 'react-router-dom';
import { getEvents, getSiteSettings, type EventCard, type SiteSettings } from '../../api/client';
import { getCachedMe, refreshMe, subscribeAuthState } from '../../api/authState';
import { useThemePreference } from '../../theme/ThemePreferenceProvider';
import { sanitizeFooterHtml } from '../../utils/html';

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
  contactWidgetEnabled: false,
  contactWidgetTitle: '联系我',
  contactWidgetHtml: '',
  footerSections: [
    {
      title: '关于站点',
      html: `<p>兽聚返图收集与画廊</p><p>© ${new Date().getFullYear()} FluffCatch. All rights reserved.</p>`
    },
    {
      title: '快速入口',
      html: '<ul><li><a href="/">首页</a></li><li><a href="/submit">返图入口</a></li></ul>'
    },
    {
      title: '站点信息',
      html: '<p>公开画廊、限时投稿和活动返图都会在这里汇总。</p>'
    }
  ]
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
      <FloatingContactWidget site={site} />
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
  const [homeSearch, setHomeSearch] = useState('');
  const [suggestions, setSuggestions] = useState<EventCard[]>([]);
  const [suggestionsLoading, setSuggestionsLoading] = useState(false);
  const [suggestionsOpen, setSuggestionsOpen] = useState(false);
  const navItems = [
    { label: '首页', path: '/', icon: <Home /> },
    { label: '返图', path: '/submit', icon: <CloudUpload /> }
  ];
  const adminTarget = authenticated ? '/admin/events' : '/login';
  const showHomeSearch = location.pathname === '/';

  useEffect(() => {
    if (!showHomeSearch) {
      setHomeSearch('');
      return;
    }
    setHomeSearch(new URLSearchParams(location.search).get('q') ?? '');
  }, [location.search, showHomeSearch]);

  useEffect(() => {
    if (!showHomeSearch) {
      setSuggestions([]);
      setSuggestionsOpen(false);
      return;
    }
    const keyword = homeSearch.trim();
    if (!keyword) {
      setSuggestions([]);
      setSuggestionsOpen(false);
      return;
    }

    let cancelled = false;
    const handle = window.setTimeout(() => {
      setSuggestionsLoading(true);
      getEvents(false, { query: keyword })
        .then((items) => {
          if (cancelled) return;
          setSuggestions(rankEventSuggestions(items, keyword).slice(0, 5));
          setSuggestionsOpen(true);
        })
        .catch(() => {
          if (cancelled) return;
          setSuggestions([]);
        })
        .finally(() => {
          if (!cancelled) setSuggestionsLoading(false);
        });
    }, 180);

    return () => {
      cancelled = true;
      window.clearTimeout(handle);
    };
  }, [homeSearch, showHomeSearch]);

  function go(path: string) {
    navigate(path);
    setMenuAnchor(null);
  }

  function updateHomeSearch(value: string) {
    setHomeSearch(value);
  }

  function submitHomeSearch(value = homeSearch) {
    const params = new URLSearchParams(location.search);
    const keyword = value.trim();
    if (keyword) {
      params.set('q', keyword);
    } else {
      params.delete('q');
    }
    const search = params.toString();
    navigate({ pathname: '/', search: search ? `?${search}` : '' }, { replace: true });
    setSuggestionsOpen(false);
  }

  function closeSuggestions(event: FocusEvent<HTMLDivElement>) {
    if (event.currentTarget.contains(event.relatedTarget as Node | null)) return;
    setSuggestionsOpen(false);
  }

  function openEvent(eventId: number) {
    setSuggestionsOpen(false);
    navigate(`/events/${eventId}`);
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
        {showHomeSearch && (
          <Box onBlur={closeSuggestions} sx={{ display: { xs: 'none', md: 'block' }, flexShrink: 0, maxWidth: 320, minWidth: 220, position: 'relative', width: '22vw' }}>
            <TextField
              aria-label="按名称搜索兽聚"
              fullWidth
              onChange={(event) => updateHomeSearch(event.target.value)}
              onFocus={() => { if (homeSearch.trim()) setSuggestionsOpen(true); }}
              onKeyDown={(event) => {
                if (event.key === 'Enter') {
                  event.preventDefault();
                  submitHomeSearch();
                }
              }}
              placeholder="搜索兽聚名称"
              size="small"
              slotProps={{
                input: {
                  startAdornment: (
                    <InputAdornment position="start">
                      <Search fontSize="small" />
                    </InputAdornment>
                  ),
                  endAdornment: suggestionsLoading ? (
                    <InputAdornment position="end">
                      <CircularProgress size={16} />
                    </InputAdornment>
                  ) : undefined
                }
              }}
              sx={{ '& .MuiOutlinedInput-root': { bgcolor: 'action.hover' } }}
              value={homeSearch}
            />
            {suggestionsOpen && homeSearch.trim() && (
              <Paper
                elevation={8}
                sx={{
                  border: '1px solid',
                  borderColor: 'divider',
                  left: 0,
                  mt: 1,
                  overflow: 'hidden',
                  position: 'absolute',
                  right: 0,
                  zIndex: (theme) => theme.zIndex.modal
                }}
              >
                {suggestions.map((event) => (
                  <Button
                    key={event.id}
                    onMouseDown={(mouseEvent) => mouseEvent.preventDefault()}
                    onClick={() => openEvent(event.id)}
                    sx={{ alignItems: 'flex-start', borderRadius: 0, color: 'text.primary', display: 'flex', justifyContent: 'flex-start', px: 1.5, py: 1.1, textAlign: 'left', width: '100%' }}
                  >
                    <Box sx={{ minWidth: 0 }}>
                      <Typography noWrap sx={{ fontWeight: 800 }} variant="body2">{event.title}</Typography>
                      <Typography noWrap color="text.secondary" variant="caption">
                        {[event.provinceName, event.cityName].filter(Boolean).join(' / ') || event.location || '未填写地点'}
                      </Typography>
                    </Box>
                  </Button>
                ))}
                {!suggestions.length && !suggestionsLoading && (
                  <Typography color="text.secondary" sx={{ px: 1.5, py: 1.25 }} variant="body2">
                    没有匹配的兽聚
                  </Typography>
                )}
              </Paper>
            )}
          </Box>
        )}
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
  const sections = normalizeFooterSections(site);
  const siteName = site.name || defaultSite.name;
  const subtitle = site.subtitle || defaultSite.subtitle;
  if (!sections.some((section) => section.title || section.html)) {
    return null;
  }
  const [introSection, ...detailSections] = sections;

  return (
    <Box
      component="footer"
      data-public-footer="true"
      sx={{
        bgcolor: '#030712',
        borderTop: '1px solid',
        borderColor: 'rgba(255, 255, 255, 0.12)',
        color: 'rgba(255, 255, 255, 0.76)',
        position: 'relative',
        px: { xs: 2, sm: 3 },
        py: { xs: 4, md: 5 },
        zIndex: 1
      }}
    >
      <Container maxWidth="xl">
        <Stack
          direction={{ xs: 'column', md: 'row' }}
          sx={{ alignItems: 'flex-start', gap: { xs: 3.5, md: 5 }, justifyContent: 'space-between' }}
        >
          <FooterIntroColumn html={introSection.html} site={site} siteName={siteName} subtitle={subtitle} title={introSection.title} />
          <Stack
            direction={{ xs: 'column', sm: 'row' }}
            sx={{ flex: 1.25, gap: { xs: 3, sm: 5, lg: 7 }, justifyContent: { xs: 'flex-start', md: 'flex-end' }, minWidth: 0, width: '100%' }}
          >
            {detailSections.map((section, index) => (
            <FooterColumn html={section.html} key={`${section.title}-${index}`} title={section.title} />
            ))}
          </Stack>
        </Stack>
      </Container>
    </Box>
  );
}

function FloatingContactWidget({ site }: { site: SiteSettings }) {
  const [open, setOpen] = useState(false);
  const [portalRoot, setPortalRoot] = useState<HTMLElement | null>(null);
  const [footerOverlap, setFooterOverlap] = useState(0);
  const sanitizedHtml = useMemo(() => sanitizeFooterHtml(site.contactWidgetHtml || ''), [site.contactWidgetHtml]);

  useEffect(() => {
    setPortalRoot(document.body);
  }, []);

  useEffect(() => {
    const footer = document.querySelector<HTMLElement>('[data-public-footer="true"]');
    if (!footer) {
      setFooterOverlap(0);
      return undefined;
    }

    let frame = 0;
    const updateOverlap = () => {
      frame = 0;
      const rect = footer.getBoundingClientRect();
      const maxOverlap = Math.max(0, window.innerHeight - 96);
      const nextOverlap = Math.ceil(Math.min(Math.max(0, window.innerHeight - rect.top), maxOverlap));
      setFooterOverlap((prev) => (prev === nextOverlap ? prev : nextOverlap));
    };
    const requestUpdate = () => {
      if (frame) return;
      frame = window.requestAnimationFrame(updateOverlap);
    };

    updateOverlap();
    window.addEventListener('resize', requestUpdate);
    window.addEventListener('scroll', requestUpdate, { passive: true });
    const resizeObserver = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(requestUpdate);
    resizeObserver?.observe(footer);

    return () => {
      if (frame) {
        window.cancelAnimationFrame(frame);
      }
      window.removeEventListener('resize', requestUpdate);
      window.removeEventListener('scroll', requestUpdate);
      resizeObserver?.disconnect();
    };
  }, []);

  if (!site.contactWidgetEnabled || !site.contactWidgetHtml.trim()) {
    return null;
  }
  if (!portalRoot) {
    return null;
  }

  return createPortal(
    <Box
      sx={{
        alignItems: 'flex-end',
        bottom: { xs: `calc(18px + ${footerOverlap}px)`, md: `calc(24px + ${footerOverlap}px)` },
        display: 'flex',
        flexDirection: 'column',
        position: 'fixed',
        right: { xs: 18, md: 24 },
        zIndex: (theme) => theme.zIndex.speedDial
      }}
    >
      {open && (
        <Paper
          elevation={10}
          sx={{
            border: '1px solid',
            borderColor: 'divider',
            borderRadius: 2,
            mb: 1.5,
            maxHeight: { xs: '60vh', sm: '70vh' },
            maxWidth: 'calc(100vw - 36px)',
            overflow: 'auto',
            p: 2,
            transformOrigin: 'bottom right',
            width: { xs: 300, sm: 320 }
          }}
        >
          <Stack direction="row" sx={{ alignItems: 'center', gap: 1, justifyContent: 'space-between', mb: 1.25 }}>
            <Typography sx={{ fontWeight: 900 }} variant="subtitle1">
              {site.contactWidgetTitle || '联系我'}
            </Typography>
            <IconButton aria-label="关闭联系卡片" onClick={() => setOpen(false)} size="small">
              <Close fontSize="small" />
            </IconButton>
          </Stack>
          <Box dangerouslySetInnerHTML={{ __html: sanitizedHtml }} sx={contactWidgetHtmlSx} />
        </Paper>
      )}
      <Tooltip title={open ? '关闭' : site.contactWidgetTitle || '联系我'}>
        <Fab
          aria-label={open ? '关闭联系卡片' : site.contactWidgetTitle || '联系我'}
          color="primary"
          onClick={() => setOpen((prev) => !prev)}
          size="medium"
          sx={{ boxShadow: 6 }}
        >
          {open ? <Close /> : <ChatBubbleOutlined />}
        </Fab>
      </Tooltip>
    </Box>,
    portalRoot
  );
}

function FooterIntroColumn({ html, site, siteName, subtitle, title }: { html: string; site: SiteSettings; siteName: string; subtitle: string; title: string }) {
  const navigate = useNavigate();
  const sanitizedHtml = useMemo(() => sanitizeFooterHtml(html), [html]);

  function handleClick(event: MouseEvent<HTMLDivElement>) {
    const anchor = (event.target as HTMLElement).closest<HTMLAnchorElement>('a[href]');
    if (!anchor) return;
    const href = anchor.getAttribute('href') || '';
    if (!isInternalPath(href)) return;
    event.preventDefault();
    navigate(href);
  }

  return (
    <Stack sx={{ flex: 1.1, gap: 2, maxWidth: 560, minWidth: { xs: '100%', md: 0 } }}>
      <Stack direction="row" sx={{ alignItems: 'center', gap: 1.5, minWidth: 0 }}>
        {site.logoUrl ? (
          <Box
            alt={siteName}
            component="img"
            src={site.logoUrl}
            sx={{ flexShrink: 0, height: 42, maxWidth: 160, objectFit: 'contain' }}
          />
        ) : (
          <Box
            sx={{
              alignItems: 'center',
              bgcolor: 'rgba(255, 255, 255, 0.1)',
              border: '1px solid rgba(255, 255, 255, 0.16)',
              borderRadius: 1.5,
              color: '#ffffff',
              display: 'flex',
              flexShrink: 0,
              fontSize: '1.1rem',
              fontWeight: 900,
              height: 42,
              justifyContent: 'center',
              width: 42
            }}
          >
            {siteName.slice(0, 1).toUpperCase()}
          </Box>
        )}
        <Box sx={{ minWidth: 0 }}>
          <Typography noWrap sx={{ color: '#ffffff', fontWeight: 900, letterSpacing: 0 }} variant="h6">
            {siteName}
          </Typography>
          {subtitle && (
            <Typography sx={{ color: 'rgba(255, 255, 255, 0.58)', lineHeight: 1.5 }} variant="body2">
              {subtitle}
            </Typography>
          )}
        </Box>
      </Stack>
      {title && (
        <Typography sx={{ color: '#ffffff', fontSize: '0.78rem', fontWeight: 900 }} variant="overline">
          {title}
        </Typography>
      )}
      <Box dangerouslySetInnerHTML={{ __html: sanitizedHtml }} onClick={handleClick} sx={footerHtmlSx} />
    </Stack>
  );
}

function FooterColumn({ html, title }: { html: string; title: string }) {
  const navigate = useNavigate();
  const sanitizedHtml = useMemo(() => sanitizeFooterHtml(html), [html]);

  function handleClick(event: MouseEvent<HTMLDivElement>) {
    const anchor = (event.target as HTMLElement).closest<HTMLAnchorElement>('a[href]');
    if (!anchor) return;
    const href = anchor.getAttribute('href') || '';
    if (!isInternalPath(href)) return;
    event.preventDefault();
    navigate(href);
  }

  return (
    <Stack sx={{ flex: 1, gap: 1.2, minWidth: { xs: '100%', md: 0 } }}>
      <Typography sx={{ color: '#ffffff', fontSize: '0.78rem', fontWeight: 900 }} variant="overline">
        {title}
      </Typography>
      <Box
        dangerouslySetInnerHTML={{ __html: sanitizedHtml }}
        onClick={handleClick}
        sx={footerHtmlSx}
      />
    </Stack>
  );
}

function normalizeFooterSections(site: SiteSettings) {
  const source = site.footerSections?.length ? site.footerSections : defaultSite.footerSections;
  return source.slice(0, 3).map((section) => ({
    html: section.html.trim(),
    title: section.title.trim()
  }));
}

function isInternalPath(href: string) {
  return href.startsWith('/') && !href.startsWith('//');
}

function rankEventSuggestions(events: EventCard[], keyword: string) {
  const normalizedKeyword = keyword.trim().toLowerCase();
  if (!normalizedKeyword) return events;
  return [...events].sort((a, b) => suggestionScore(b, normalizedKeyword) - suggestionScore(a, normalizedKeyword));
}

function suggestionScore(event: EventCard, keyword: string) {
  const title = event.title.toLowerCase();
  if (title === keyword) return 1000;
  const index = title.indexOf(keyword);
  const coverage = keyword.length / Math.max(title.length, 1);
  let score = Math.round(coverage * 100);
  if (index === 0) score += 300;
  else if (index > 0) score += 180 - Math.min(index * 8, 120);

  let cursor = 0;
  let orderedHits = 0;
  for (const char of keyword) {
    const foundAt = title.indexOf(char, cursor);
    if (foundAt < 0) continue;
    orderedHits += 1;
    cursor = foundAt + 1;
  }
  score += Math.round((orderedHits / keyword.length) * 80);
  return score;
}

const footerHtmlSx = {
  color: 'rgba(255, 255, 255, 0.72)',
  fontSize: '0.875rem',
  lineHeight: 1.75,
  overflowWrap: 'anywhere',
  '& :first-of-type': { mt: 0 },
  '& :last-child': { mb: 0 },
  '& a': {
    color: 'rgba(255, 255, 255, 0.78)',
    textDecoration: 'none',
    transition: 'color 160ms ease',
    '&:hover': { color: '#ffffff' }
  },
  '& p': { mb: 1, mt: 0 },
  '& ul, & ol': { mb: 1, mt: 0, pl: 2.2 },
  '& li': { mb: 0.5 },
  '& strong, & b': { color: '#ffffff', fontWeight: 800 },
  '& img': {
    borderRadius: 1,
    display: 'block',
    height: 'auto',
    maxHeight: 120,
    maxWidth: '100%',
    objectFit: 'contain'
  },
  '& h1, & h2, & h3, & h4, & h5, & h6': {
    color: '#ffffff',
    fontSize: '0.95rem',
    fontWeight: 800,
    lineHeight: 1.4,
    mb: 1,
    mt: 0
  }
};

const contactWidgetHtmlSx = {
  color: 'text.secondary',
  fontSize: '0.9rem',
  lineHeight: 1.7,
  overflowWrap: 'anywhere',
  '& :first-of-type': { mt: 0 },
  '& :last-child': { mb: 0 },
  '& a': { color: 'primary.main', textDecoration: 'none', '&:hover': { textDecoration: 'underline' } },
  '& p': { mb: 1, mt: 0 },
  '& ul, & ol': { mb: 1, mt: 0, pl: 2.2 },
  '& li': { mb: 0.5 },
  '& strong, & b': { color: 'text.primary', fontWeight: 800 },
  '& img': {
    border: '1px solid',
    borderColor: 'divider',
    borderRadius: 1.5,
    display: 'block',
    height: 'auto',
    maxHeight: 220,
    maxWidth: '100%',
    objectFit: 'contain'
  },
  '& h1, & h2, & h3, & h4, & h5, & h6': {
    color: 'text.primary',
    fontSize: '1rem',
    fontWeight: 900,
    lineHeight: 1.35,
    mb: 1,
    mt: 0
  }
};
