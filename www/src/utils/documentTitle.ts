const fallbackSiteName = 'FluffCatch';

const adminSettingsPageNames: Record<string, string> = {
  background: '前台背景',
  contact: '联系卡片',
  footer: '页脚备案',
  maintenance: '存储维护',
  oidc: 'OIDC 登录',
  security: '账号安全',
  site: '站点信息',
  storage: '存储策略',
  theme: '主题配色',
  upload: '上传限制'
};

export function documentTitleForPath(pathname: string, siteName: string, options: { eventTitle?: string } = {}) {
  return documentTitleFromPageName(pageNameForPath(pathname, options), siteName);
}

export function documentTitleFromPageName(pageName: string, siteName: string) {
  const normalizedSiteName = siteName.trim() || fallbackSiteName;
  const normalizedPageName = pageName.trim();
  if (!normalizedPageName || normalizedPageName === '首页') return normalizedSiteName;
  return `${normalizedPageName} - ${normalizedSiteName}`;
}

export function pageNameForPath(pathname: string, options: { eventTitle?: string } = {}) {
  if (pathname === '/') return '首页';
  if (pathname === '/submit') return '返图入口';
  if (pathname === '/login') return '登录';
  if (pathname.startsWith('/events/')) return options.eventTitle?.trim() || '兽聚详情';
  if (pathname === '/admin' || pathname === '/admin/events' || pathname === '/admin/dashboard' || pathname === '/admin/submissions') return '兽聚管理';
  if (pathname === '/admin/settings') return '系统设置';
  if (pathname.startsWith('/admin/settings/')) {
    const parts = pathname.split('/').filter(Boolean);
    const section = parts[parts.length - 1] || '';
    return adminSettingsPageNames[section] || '系统设置';
  }
  return '';
}
