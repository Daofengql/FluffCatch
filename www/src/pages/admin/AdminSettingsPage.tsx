import MDEditor from '@uiw/react-md-editor/nohighlight';
import '@uiw/react-md-editor/markdown-editor.css';
import '@uiw/react-markdown-preview/markdown.css';
import { Article, ChatBubbleOutlined, ColorLens, Image, Info, Key, Search, Security, Storage, UploadFile } from '@mui/icons-material';
import {
  Alert,
  Avatar,
  Box,
  Button,
  Chip,
  Divider,
  FormControl,
  FormControlLabel,
  Grid,
  InputLabel,
  List,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  MenuItem,
  Paper,
  Select,
  Stack,
  Switch,
  TextField,
  Typography
} from '@mui/material';
import { useColorScheme } from '@mui/material/styles';
import type { SelectChangeEvent } from '@mui/material/Select';
import { type ChangeEvent, type FocusEvent, type FormEvent, type ReactNode, useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import {
  changePassword,
  clearSiteBackground,
  clearSiteLogo,
  getAdminSettings,
  getOIDCBindURL,
  getOIDCStatus,
  scanMissingThumbnails,
  scanOrphanStorageObjects,
  testStorageConnection,
  updateSiteSettings,
  updateStoragePolicies,
  updateUploadSettings,
  unbindOIDC,
  uploadSiteBackground,
  uploadSiteLogo,
  type AdminSettingsResponse,
  type FooterSection,
  type OIDCStatus,
  type S3Settings,
  type SiteSettings,
  type StorageDriver,
  type StoragePolicy,
  type UploadSettings
} from '../../api/client';
import { PageHeader } from '../../components/common/PageHeader';
import { useThemePreference } from '../../theme/ThemePreferenceProvider';
import { appPalettes, normalizeThemeColor } from '../../theme/theme';
import { sanitizeFooterHtml } from '../../utils/html';

type SettingsSection = 'site' | 'theme' | 'background' | 'contact' | 'footer' | 'upload' | 'storage' | 'maintenance' | 'security';
type BackgroundVariant = 'desktop' | 'mobile';

const fallbackFooterSections: FooterSection[] = [
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
];

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
  contactWidgetEnabled: false,
  contactWidgetTitle: '联系我',
  contactWidgetHtml: '',
  footerSections: fallbackFooterSections
};

const fallbackUpload: UploadSettings = {
  maxFileSizeMb: 20,
  maxVideoSizeMb: 500,
  maxFilesPerUpload: 20,
  defaultPageSize: 24,
  maxConcurrentUploads: 2
};

const emptyS3: S3Settings = { endpoint: '', bucket: '', region: '', accessKey: '', secretKey: '', useSsl: false, accountId: '' };

const driverOptions: { value: StorageDriver; label: string }[] = [
  { value: 'local', label: '本地存储' },
  { value: 'minio', label: 'MinIO' },
  { value: 'aws-s3', label: 'AWS S3' },
  { value: 'aliyun-oss', label: '阿里云 OSS' },
  { value: 'tencent-cos', label: '腾讯云 COS' },
  { value: 'cf-r2', label: 'Cloudflare R2' },
  { value: 's3', label: 'S3 兼容存储' }
];

const settingsSections: { icon: ReactNode; key: SettingsSection; label: string }[] = [
  { icon: <Info />, key: 'site', label: '站点信息' },
  { icon: <ColorLens />, key: 'theme', label: '主题配色' },
  { icon: <Image />, key: 'background', label: '前台背景' },
  { icon: <ChatBubbleOutlined />, key: 'contact', label: '联系卡片' },
  { icon: <Article />, key: 'footer', label: '页脚备案' },
  { icon: <UploadFile />, key: 'upload', label: '上传限制' },
  { icon: <Storage />, key: 'storage', label: '存储策略' },
  { icon: <Search />, key: 'maintenance', label: '存储维护' },
  { icon: <Security />, key: 'security', label: '账号安全' }
];

export function AdminSettingsPage() {
  const navigate = useNavigate();
  const params = useParams();
  const { colorScheme } = useColorScheme();
  const { applySiteSettings } = useThemePreference();
  const activeSection = normalizeSection(params.section);
  const [settings, setSettings] = useState<AdminSettingsResponse | null>(null);
  const [site, setSite] = useState<SiteSettings>(fallbackSite);
  const [upload, setUpload] = useState<UploadSettings>(fallbackUpload);
  const [message, setMessage] = useState('');
  const [error, setError] = useState('');
  const [logoUploading, setLogoUploading] = useState(false);
  const [backgroundUploading, setBackgroundUploading] = useState<BackgroundVariant | ''>('');
  const themePreviewRef = useRef<HTMLDivElement | null>(null);
  const colorPreviewFrameRef = useRef<number | null>(null);
  const colorInputDraftRef = useRef(normalizeThemeColor(fallbackSite.themePrimaryColor));

  const [storagePolicy, setStoragePolicy] = useState<StoragePolicy>({
    id: 'default',
    name: '默认存储',
    driver: 'local',
    publicPrefix: '/media',
    localPath: 'data/uploads',
    s3: { ...emptyS3 }
  });
  const [storageSaving, setStorageSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<{ success: boolean; error?: string } | null>(null);

  const markdownColorMode = colorScheme === 'dark' ? 'dark' : 'light';
  const isS3Driver = ['minio', 'aws-s3', 'aliyun-oss', 'tencent-cos', 'cf-r2', 's3'].includes(storagePolicy.driver);
  const usage = settings?.usage[storagePolicy.id] ?? { objectCount: 0, sizeBytes: 0 };
  const activePolicyId = settings?.settings.storagePolicies.activePolicyId || storagePolicy.id;
  const selectedSection = useMemo(() => settingsSections.find((item) => item.key === activeSection) ?? settingsSections[0], [activeSection]);
  const themePreview = useMemo(() => resolveThemePreview(site, colorScheme), [colorScheme, site.themeMode, site.themePreset, site.themePrimaryColor]);

  useEffect(() => {
    if (themePreviewRef.current) {
      applyThemePreviewVariables(themePreviewRef.current, themePreview);
    }
  }, [themePreview]);

  useEffect(() => {
    colorInputDraftRef.current = normalizeThemeColor(site.themePrimaryColor);
  }, [site.themePrimaryColor]);

  useEffect(() => () => {
    if (colorPreviewFrameRef.current !== null) {
      window.cancelAnimationFrame(colorPreviewFrameRef.current);
    }
  }, []);

  function refresh() {
    getAdminSettings()
      .then((payload) => {
        setSettings(payload);
        const nextSite = { ...fallbackSite, ...payload.settings.site };
        setSite(nextSite);
        applySiteSettings(nextSite);
        setUpload({ ...fallbackUpload, ...payload.settings.upload });
        const policies = payload.settings.storagePolicies.policies;
        if (policies.length > 0) {
          const activeId = payload.settings.storagePolicies.activePolicyId;
          const active = policies.find((p) => p.id === activeId) ?? policies[0];
          setStoragePolicy(active);
        }
      })
      .catch((err: unknown) => {
        setError(err instanceof Error ? err.message : '设置加载失败');
        setSettings(null);
      });
  }

  useEffect(refresh, [applySiteSettings]);

  useEffect(() => {
    if (params.section && !isSettingsSection(params.section)) {
      navigate('/admin/settings/site', { replace: true });
    }
  }, [navigate, params.section]);

  function handleSiteChange(event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) {
    setSite((prev) => ({ ...prev, [event.target.name]: event.target.value }));
  }

  function updateFooterSection(index: number, patch: Partial<FooterSection>) {
    setSite((prev) => {
      const sections = normalizeFooterSections(prev.footerSections);
      sections[index] = { ...sections[index], ...patch };
      return { ...prev, footerSections: sections };
    });
  }

  function updateThemeDraft(patch: Partial<Pick<SiteSettings, 'themeMode' | 'themePreset' | 'themePrimaryColor'>>) {
    setSite((prev) => {
      const next = { ...prev, ...patch };
      if (patch.themePrimaryColor !== undefined) {
        colorInputDraftRef.current = normalizeThemeColor(patch.themePrimaryColor);
      }
      if (patch.themePreset && patch.themePreset !== 'custom') {
        colorInputDraftRef.current = normalizeThemeColor(next.themePrimaryColor);
      }
      return next;
    });
  }

  function previewThemeColor(value: string) {
    const nextColor = normalizeThemeColor(value);
    colorInputDraftRef.current = nextColor;

    if (colorPreviewFrameRef.current !== null) {
      window.cancelAnimationFrame(colorPreviewFrameRef.current);
    }
    colorPreviewFrameRef.current = window.requestAnimationFrame(() => {
      colorPreviewFrameRef.current = null;
      if (!themePreviewRef.current) return;
      applyThemePreviewVariables(
        themePreviewRef.current,
        resolveThemePreview({ ...site, themePreset: 'custom', themePrimaryColor: nextColor }, colorScheme)
      );
    });
  }

  function commitThemeColor(value: string) {
    const nextColor = normalizeThemeColor(value);
    updateThemeDraft({ themePreset: 'custom', themePrimaryColor: nextColor });
  }

  async function saveSite(nextSite: SiteSettings, successMessage: string) {
    setError('');
    setMessage('');
    try {
      const result = await updateSiteSettings(nextSite);
      const mergedSite = { ...fallbackSite, ...result.site };
      setSite(mergedSite);
      applySiteSettings(mergedSite);
      setMessage(successMessage);
      refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : '站点设置保存失败');
    }
  }

  async function handleSiteSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    await saveSite(site, '站点信息已保存。');
  }

  async function handleThemeSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const colorDraft = colorInputDraftRef.current;
    const nextSite = colorDraft !== normalizeThemeColor(site.themePrimaryColor)
      ? { ...site, themePreset: 'custom' as const, themePrimaryColor: colorDraft }
      : site;
    await saveSite(nextSite, '主题配色已保存。');
  }

  async function handleFooterSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    await saveSite({ ...site, footerSections: normalizeFooterSections(site.footerSections) }, '页脚内容已保存。');
  }

  async function handleContactSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    await saveSite(site, '联系卡片已保存。');
  }

  async function handleUploadSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError('');
    setMessage('');
    try {
      const result = await updateUploadSettings(upload);
      setUpload(result.upload);
      setMessage('上传限制已保存。');
      refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : '上传限制保存失败');
    }
  }

  async function handleLogoSelect(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0] ?? null;
    event.target.value = '';
    if (!file) return;
    setError('');
    setMessage('');
    setLogoUploading(true);
    try {
      const result = await uploadSiteLogo(file);
      const nextSite = { ...fallbackSite, ...result.site };
      setSite(nextSite);
      applySiteSettings(nextSite);
      setMessage('Logo 已上传。');
      refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Logo 上传失败');
    } finally {
      setLogoUploading(false);
    }
  }

  async function handleClearLogo() {
    setError('');
    setMessage('');
    try {
      const result = await clearSiteLogo();
      const nextSite = { ...fallbackSite, ...result.site };
      setSite(nextSite);
      applySiteSettings(nextSite);
      setMessage('Logo 已清空。');
      refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Logo 清空失败');
    }
  }

  async function handleBackgroundSelect(variant: BackgroundVariant, event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0] ?? null;
    event.target.value = '';
    if (!file) return;
    setError('');
    setMessage('');
    setBackgroundUploading(variant);
    try {
      const result = await uploadSiteBackground(variant, file);
      const nextSite = { ...fallbackSite, ...result.site };
      setSite(nextSite);
      applySiteSettings(nextSite);
      setMessage(`${variant === 'desktop' ? '桌面端' : '移动端'}背景已上传并处理。`);
      refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : '背景上传失败');
    } finally {
      setBackgroundUploading('');
    }
  }

  async function handleClearBackground(variant: BackgroundVariant) {
    setError('');
    setMessage('');
    try {
      const result = await clearSiteBackground(variant);
      const nextSite = { ...fallbackSite, ...result.site };
      setSite(nextSite);
      applySiteSettings(nextSite);
      setMessage(`${variant === 'desktop' ? '桌面端' : '移动端'}背景已清空。`);
      refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : '背景清空失败');
    }
  }

  function handleDriverChange(event: SelectChangeEvent) {
    const driver = event.target.value as StorageDriver;
    setStoragePolicy((prev) => {
      const next = { ...prev, driver, s3: { ...emptyS3 } };
      if (driver === 'aws-s3') {
        next.s3!.region = 'us-east-1';
        next.s3!.useSsl = true;
      }
      if (driver === 'cf-r2') {
        next.s3!.region = 'auto';
        next.s3!.useSsl = true;
      }
      return next;
    });
    setTestResult(null);
  }

  function updateS3Field(field: keyof S3Settings, value: string | boolean) {
    setStoragePolicy((prev) => ({ ...prev, s3: { ...prev.s3!, [field]: value } }));
    setTestResult(null);
  }

  async function handleTestConnection() {
    setTesting(true);
    setTestResult(null);
    try {
      const result = await testStorageConnection(storagePolicy);
      setTestResult(result);
    } catch (err) {
      setTestResult({ success: false, error: err instanceof Error ? err.message : '测试失败' });
    } finally {
      setTesting(false);
    }
  }

  async function handleStorageSave() {
    setStorageSaving(true);
    setError('');
    setMessage('');
    try {
      await updateStoragePolicies({ activePolicyId: storagePolicy.id, policies: [storagePolicy] });
      setMessage('存储策略已保存。');
      refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : '存储策略保存失败');
    } finally {
      setStorageSaving(false);
    }
  }

  return (
    <Stack sx={{ gap: 3 }}>
      <PageHeader subtitle="站点与上传设置即时生效，账号与 OIDC 设置会写入本地配置文件" title="系统设置" />
      {message && <Alert onClose={() => setMessage('')} severity="success">{message}</Alert>}
      {error && <Alert onClose={() => setError('')} severity="error">{error}</Alert>}

      <Stack direction={{ xs: 'column', md: 'row' }} sx={{ alignItems: 'flex-start', gap: 2.5 }}>
        <Paper
          sx={{
            flexShrink: 0,
            maxWidth: '100%',
            overflowX: { xs: 'auto', md: 'visible' },
            p: 1,
            position: { xs: 'sticky', md: 'static' },
            top: { xs: 72, md: 'auto' },
            width: { xs: '100%', md: 220 },
            zIndex: 2
          }}
          variant="outlined"
        >
          <List disablePadding sx={{ display: { xs: 'flex', md: 'block' }, gap: { xs: 0.75, md: 0 }, minWidth: { xs: 'max-content', md: 'auto' } }}>
            {settingsSections.map((item) => (
              <ListItemButton
                key={item.key}
                onClick={() => navigate(`/admin/settings/${item.key}`)}
                selected={activeSection === item.key}
                sx={{
                  borderRadius: 1.5,
                  flexShrink: 0,
                  mb: { xs: 0, md: 0.25 },
                  minHeight: 42,
                  px: { xs: 1.25, md: 2 }
                }}
              >
                <ListItemIcon sx={{ minWidth: { xs: 28, md: 38 } }}>{item.icon}</ListItemIcon>
                <ListItemText primary={item.label} slotProps={{ primary: { noWrap: true } }} />
              </ListItemButton>
            ))}
          </List>
        </Paper>

        <Paper sx={{ flex: 1, minWidth: 0, p: { xs: 2, sm: 3 } }} variant="outlined">
          <Typography sx={{ fontWeight: 900, mb: 0.75 }} variant="h5">
            {selectedSection.label}
          </Typography>
          <Divider sx={{ mb: 2.5 }} />

          {activeSection === 'site' && (
            <Stack component="form" onSubmit={handleSiteSubmit} sx={{ gap: 2.5 }}>
              <Stack direction={{ xs: 'column', sm: 'row' }} sx={{ alignItems: { xs: 'flex-start', sm: 'center' }, gap: 2 }}>
                <Avatar src={site.logoUrl || undefined} sx={{ bgcolor: 'primary.main', color: 'primary.contrastText', fontWeight: 900, height: 64, width: 64 }} variant="rounded">
                  {(site.name || 'F').slice(0, 1).toUpperCase()}
                </Avatar>
                <Stack sx={{ gap: 1 }}>
                  <Typography color="text.secondary">公开页导航栏会显示站点名称、副标题与 Logo。</Typography>
                  <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 1 }}>
                    <Button component="label" disabled={logoUploading} size="small" variant="outlined">
                      {logoUploading ? '上传中...' : site.logoUrl ? '更换 Logo' : '上传 Logo'}
                      <input accept="image/*" hidden onChange={handleLogoSelect} type="file" />
                    </Button>
                    {site.logoUrl && (
                      <Button color="secondary" onClick={() => void handleClearLogo()} size="small">清空 Logo</Button>
                    )}
                  </Stack>
                </Stack>
              </Stack>
              <Grid container spacing={2}>
                <Grid size={{ xs: 12, md: 4 }}>
                  <TextField fullWidth label="站点名称" name="name" onChange={handleSiteChange} value={site.name} />
                </Grid>
                <Grid size={{ xs: 12, md: 8 }}>
                  <TextField fullWidth label="副标题" name="subtitle" onChange={handleSiteChange} value={site.subtitle} />
                </Grid>
                <Grid size={{ xs: 12 }}>
                  <Typography sx={{ fontWeight: 700, mb: 1 }}>首页介绍 Markdown</Typography>
                  <Box data-color-mode={markdownColorMode} sx={{ '& .w-md-editor': { bgcolor: 'background.paper', boxShadow: 'none' }, '& .wmde-markdown': { bgcolor: 'transparent' } }}>
                    <MDEditor height={320} onChange={(value) => setSite((prev) => ({ ...prev, homeMarkdown: value || '' }))} preview="edit" value={site.homeMarkdown} />
                  </Box>
                </Grid>
              </Grid>
              <Box>
                <Button type="submit" variant="contained">保存站点信息</Button>
              </Box>
            </Stack>
          )}

          {activeSection === 'theme' && (
            <Stack component="form" onSubmit={handleThemeSubmit} sx={{ gap: 2.5 }}>
              <Grid container spacing={2}>
                <Grid size={{ xs: 12, sm: 6 }}>
                  <FormControl fullWidth>
                    <InputLabel>明暗模式</InputLabel>
                    <Select
                      label="明暗模式"
                      onChange={(event) => updateThemeDraft({ themeMode: event.target.value as SiteSettings['themeMode'] })}
                      value={site.themeMode}
                    >
                      <MenuItem value="system">跟随系统</MenuItem>
                      <MenuItem value="light">浅色</MenuItem>
                      <MenuItem value="dark">深色</MenuItem>
                    </Select>
                  </FormControl>
                </Grid>
                <Grid size={{ xs: 12, sm: 6 }}>
                  <FormControl fullWidth>
                    <InputLabel>主题方案</InputLabel>
                    <Select
                      label="主题方案"
                      onChange={(event) => updateThemeDraft({ themePreset: event.target.value as SiteSettings['themePreset'] })}
                      value={site.themePreset}
                    >
                      {appPalettes.map((item) => (
                        <MenuItem key={item.key} value={item.key}>{item.label}</MenuItem>
                      ))}
                      <MenuItem value="custom">自定义</MenuItem>
                    </Select>
                  </FormControl>
                </Grid>
              </Grid>

              <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 1 }}>
                {appPalettes.map((item) => (
                  <Button
                    key={item.key}
                    onClick={() => updateThemeDraft({ themePreset: item.key })}
                    startIcon={<Box sx={{ bgcolor: item.swatch, borderRadius: '50%', height: 16, width: 16 }} />}
                    variant={site.themePreset === item.key ? 'contained' : 'outlined'}
                  >
                    {item.label}
                  </Button>
                ))}
                <Button
                  onClick={() => updateThemeDraft({ themePreset: 'custom' })}
                  startIcon={<Box sx={{ bgcolor: normalizeThemeColor(site.themePrimaryColor), borderRadius: '50%', height: 16, width: 16 }} />}
                  variant={site.themePreset === 'custom' ? 'contained' : 'outlined'}
                >
                  自定义
                </Button>
              </Stack>

              <Grid container spacing={2}>
                <Grid size={{ xs: 12, sm: 3 }}>
                  <TextField
                    defaultValue={normalizeThemeColor(site.themePrimaryColor)}
                    fullWidth
                    helperText="选色后自动使用自定义方案"
                    key={`theme-color-${normalizeThemeColor(site.themePrimaryColor)}`}
                    label="自定义色盘"
                    name="themePrimaryColor"
                    slotProps={{
                      htmlInput: {
                        onBlur: (event: FocusEvent<HTMLInputElement>) => commitThemeColor(event.currentTarget.value),
                        onInput: (event: FormEvent<HTMLInputElement>) => previewThemeColor(event.currentTarget.value),
                        style: { height: 42 }
                      }
                    }}
                    type="color"
                  />
                </Grid>
                <Grid size={{ xs: 12, sm: 9 }}>
                  <TextField
                    fullWidth
                    helperText="输入合法 #RRGGBB 后会实时预览"
                    label="自定义主色 Hex"
                    name="themePrimaryColor"
                    onChange={(event) => updateThemeDraft({ themePreset: 'custom', themePrimaryColor: event.target.value })}
                    placeholder="#2563eb"
                    value={site.themePrimaryColor}
                  />
                </Grid>
              </Grid>

              <Box
                ref={themePreviewRef}
                sx={{
                  '--theme-preview-background': themePreview.background,
                  '--theme-preview-border': themePreview.border,
                  '--theme-preview-contrast': themePreview.contrastText,
                  '--theme-preview-primary': themePreview.primary,
                  '--theme-preview-secondary': themePreview.secondary,
                  '--theme-preview-selected': themePreview.selected,
                  '--theme-preview-text': themePreview.text,
                  bgcolor: 'var(--theme-preview-background)',
                  border: '1px solid',
                  borderColor: 'var(--theme-preview-border)',
                  borderRadius: 2,
                  color: 'var(--theme-preview-text)',
                  p: 2
                }}
              >
                <Stack direction={{ xs: 'column', sm: 'row' }} sx={{ alignItems: { xs: 'flex-start', sm: 'center' }, gap: 1.5, justifyContent: 'space-between' }}>
                  <Box>
                    <Typography sx={{ color: 'var(--theme-preview-text)', fontWeight: 800 }}>主题预览</Typography>
                    <Typography sx={{ color: 'var(--theme-preview-secondary)' }} variant="body2">实时效果只在这个预览容器内生效；保存后前台和后台才会使用这套配色。</Typography>
                  </Box>
                  <Stack direction="row" sx={{ gap: 1 }}>
                    <Button
                      sx={{
                        bgcolor: 'var(--theme-preview-primary)',
                        color: 'var(--theme-preview-contrast)',
                        '&:hover': { bgcolor: 'var(--theme-preview-primary)' }
                      }}
                      variant="contained"
                    >
                      主按钮
                    </Button>
                    <Button
                      sx={{
                        borderColor: 'var(--theme-preview-primary)',
                        color: 'var(--theme-preview-primary)',
                        '&:hover': { bgcolor: 'var(--theme-preview-selected)', borderColor: 'var(--theme-preview-primary)' }
                      }}
                      variant="outlined"
                    >
                      次按钮
                    </Button>
                  </Stack>
                </Stack>
              </Box>

              <Box>
                <Button type="submit" variant="contained">保存主题配色</Button>
              </Box>
            </Stack>
          )}

          {activeSection === 'background' && (
            <Stack sx={{ gap: 2.5 }}>
              <Grid container spacing={2.5}>
                <Grid size={{ xs: 12, md: 7 }}>
                  <BackgroundPanel
                    aspectRatio="16 / 9"
                    disabled={backgroundUploading === 'desktop'}
                    label="桌面端背景"
                    onClear={() => void handleClearBackground('desktop')}
                    onSelect={(event) => void handleBackgroundSelect('desktop', event)}
                    processingText="上传后会处理为 1920 x 1080 JPEG"
                    url={site.publicBackgroundDesktopUrl}
                  />
                </Grid>
                <Grid size={{ xs: 12, md: 5 }}>
                  <BackgroundPanel
                    aspectRatio="9 / 16"
                    disabled={backgroundUploading === 'mobile'}
                    label="移动端背景"
                    onClear={() => void handleClearBackground('mobile')}
                    onSelect={(event) => void handleBackgroundSelect('mobile', event)}
                    processingText="上传后会处理为 1080 x 1920 JPEG"
                    url={site.publicBackgroundMobileUrl}
                  />
                </Grid>
              </Grid>
              <Typography color="text.secondary" variant="body2">
                前台实际显示时使用居中 cover，视口看到的是原图中按容器比例裁出的最大内接矩形；页头和页脚不使用背景图。
              </Typography>
            </Stack>
          )}

          {activeSection === 'footer' && (
            <Stack component="form" onSubmit={handleFooterSubmit} sx={{ gap: 2.5 }}>
              <Grid container spacing={2}>
                {normalizeFooterSections(site.footerSections).map((section, index) => (
                  <Grid key={`footer-editor-${index}`} size={{ xs: 12, md: index === 0 ? 12 : 6 }}>
                    <Paper sx={{ p: 2 }} variant="outlined">
                      <Stack sx={{ gap: 1.5 }}>
                        <TextField
                          fullWidth
                          label={`分区 ${index + 1} 标题`}
                          onChange={(event) => updateFooterSection(index, { title: event.target.value })}
                          value={section.title}
                        />
                        <TextField
                          fullWidth
                          label={`分区 ${index + 1} HTML`}
                          minRows={index === 0 ? 5 : 8}
                          multiline
                          onChange={(event) => updateFooterSection(index, { html: event.target.value })}
                          placeholder='<p>联系方式、备案号、社群链接等内容都可以放在这里。</p>'
                          value={section.html}
                        />
                      </Stack>
                    </Paper>
                  </Grid>
                ))}
              </Grid>
              <FooterPreview sections={normalizeFooterSections(site.footerSections)} site={site} />
              <Box>
                <Button type="submit" variant="contained">保存页脚内容</Button>
              </Box>
            </Stack>
          )}

          {activeSection === 'contact' && (
            <Stack component="form" onSubmit={handleContactSubmit} sx={{ gap: 2.5 }}>
              <Paper sx={{ p: 2 }} variant="outlined">
                <Stack sx={{ gap: 2 }}>
                  <Stack direction={{ xs: 'column', md: 'row' }} sx={{ alignItems: { xs: 'stretch', md: 'center' }, gap: 2, justifyContent: 'space-between' }}>
                    <Box>
                      <Typography sx={{ fontWeight: 900 }}>悬浮联系卡片</Typography>
                      <Typography color="text.secondary" variant="body2">显示在前台右下角，适合放 QQ、微信或社群二维码。</Typography>
                    </Box>
                    <FormControlLabel
                      control={<Switch checked={site.contactWidgetEnabled} onChange={(event) => setSite((prev) => ({ ...prev, contactWidgetEnabled: event.target.checked }))} />}
                      label="启用"
                    />
                  </Stack>
                  <Grid container spacing={2}>
                    <Grid size={{ xs: 12, md: 4 }}>
                      <TextField
                        fullWidth
                        label="按钮标题"
                        name="contactWidgetTitle"
                        onChange={handleSiteChange}
                        placeholder="联系我"
                        value={site.contactWidgetTitle}
                      />
                    </Grid>
                    <Grid size={{ xs: 12, md: 8 }}>
                      <TextField
                        fullWidth
                        label="卡片 HTML"
                        minRows={6}
                        multiline
                        name="contactWidgetHtml"
                        onChange={handleSiteChange}
                        placeholder='<p>扫码联系</p><img src="https://example.com/qrcode.png" alt="二维码" />'
                        value={site.contactWidgetHtml}
                      />
                    </Grid>
                  </Grid>
                  <ContactWidgetPreview site={site} />
                </Stack>
              </Paper>
              <Box>
                <Button type="submit" variant="contained">保存联系卡片</Button>
              </Box>
            </Stack>
          )}

          {activeSection === 'upload' && (
            <Stack component="form" onSubmit={handleUploadSubmit} sx={{ gap: 2.5 }}>
              <Grid container spacing={2}>
                <Grid size={{ xs: 12, sm: 4 }}>
                  <TextField
                    fullWidth
                    label="图片单文件最大大小（MB）"
                    onChange={(event) => setUpload((prev) => ({ ...prev, maxFileSizeMb: Number(event.target.value) }))}
                    slotProps={{ htmlInput: { min: 1, max: 1024, step: 1 } }}
                    type="number"
                    value={upload.maxFileSizeMb}
                  />
                </Grid>
                <Grid size={{ xs: 12, sm: 4 }}>
                  <TextField
                    fullWidth
                    label="视频单文件最大大小（MB）"
                    onChange={(event) => setUpload((prev) => ({ ...prev, maxVideoSizeMb: Number(event.target.value) }))}
                    slotProps={{ htmlInput: { min: 1, max: 10240, step: 1 } }}
                    type="number"
                    value={upload.maxVideoSizeMb}
                  />
                </Grid>
                <Grid size={{ xs: 12, sm: 4 }}>
                  <TextField
                    fullWidth
                    label="单次上传最大数量"
                    onChange={(event) => setUpload((prev) => ({ ...prev, maxFilesPerUpload: Number(event.target.value) }))}
                    slotProps={{ htmlInput: { min: 1, max: 200, step: 1 } }}
                    type="number"
                    value={upload.maxFilesPerUpload}
                  />
                </Grid>
                <Grid size={{ xs: 12, sm: 4 }}>
                  <TextField
                    fullWidth
                    label="画廊默认分页大小"
                    onChange={(event) => setUpload((prev) => ({ ...prev, defaultPageSize: Number(event.target.value) }))}
                    slotProps={{ htmlInput: { min: 1, max: 100, step: 1 } }}
                    type="number"
                    value={upload.defaultPageSize}
                  />
                </Grid>
                <Grid size={{ xs: 12, sm: 4 }}>
                  <TextField
                    fullWidth
                    helperText="后端硬上限为 8，超出会自动压到 8"
                    label="并发上传数量"
                    onChange={(event) => setUpload((prev) => ({ ...prev, maxConcurrentUploads: Number(event.target.value) }))}
                    slotProps={{ htmlInput: { min: 1, max: 8, step: 1 } }}
                    type="number"
                    value={upload.maxConcurrentUploads}
                  />
                </Grid>
              </Grid>
              <Box>
                <Button type="submit" variant="contained">保存上传限制</Button>
              </Box>
            </Stack>
          )}

          {activeSection === 'storage' && (
            <Stack sx={{ gap: 2.5 }}>
              <Typography color="text.secondary">
                同一时间只启用一个存储策略。切换存储只影响新上传文件；已有文件按记录中的策略地址读取。
              </Typography>
              <Stack direction={{ xs: 'column', sm: 'row' }} sx={{ alignItems: { xs: 'stretch', sm: 'center' }, gap: 1.5 }}>
                <TextField
                  fullWidth
                  label="策略 ID"
                  slotProps={{ input: { readOnly: true } }}
                  value={storagePolicy.id || '保存后自动生成'}
                />
                <Chip color="primary" label={`当前启用：${activePolicyId || '未设置'}`} variant="outlined" />
              </Stack>
              <Grid container spacing={2}>
                <Grid size={{ xs: 12, sm: 6 }}>
                  <TextField fullWidth label="策略名称" onChange={(event) => setStoragePolicy((prev) => ({ ...prev, name: event.target.value }))} value={storagePolicy.name} />
                </Grid>
                <Grid size={{ xs: 12, sm: 6 }}>
                  <FormControl fullWidth>
                    <InputLabel>存储类型</InputLabel>
                    <Select label="存储类型" onChange={handleDriverChange} value={storagePolicy.driver}>
                      {driverOptions.map((opt) => (
                        <MenuItem key={opt.value} value={opt.value}>{opt.label}</MenuItem>
                      ))}
                    </Select>
                  </FormControl>
                </Grid>
              </Grid>

              {storagePolicy.driver === 'local' && (
                <TextField fullWidth helperText="例如 data/uploads" label="本地存储路径" onChange={(event) => setStoragePolicy((prev) => ({ ...prev, localPath: event.target.value }))} value={storagePolicy.localPath || ''} />
              )}

              {isS3Driver && (
                <Stack sx={{ gap: 2 }}>
                  {storagePolicy.driver === 'cf-r2' && (
                    <TextField fullWidth helperText="在 Cloudflare Dashboard 右侧找到 Account ID" label="Cloudflare Account ID" onChange={(event) => updateS3Field('accountId', event.target.value)} value={storagePolicy.s3?.accountId || ''} />
                  )}
                  {(storagePolicy.driver === 'minio' || storagePolicy.driver === 'aliyun-oss' || storagePolicy.driver === 'tencent-cos' || storagePolicy.driver === 's3') && (
                    <TextField fullWidth label="Endpoint" onChange={(event) => updateS3Field('endpoint', event.target.value)} value={storagePolicy.s3?.endpoint || ''} />
                  )}
                  <TextField fullWidth label="Bucket" onChange={(event) => updateS3Field('bucket', event.target.value)} value={storagePolicy.s3?.bucket || ''} />
                  {storagePolicy.driver !== 'cf-r2' && (
                    <TextField fullWidth label="Region" onChange={(event) => updateS3Field('region', event.target.value)} value={storagePolicy.s3?.region || ''} />
                  )}
                  <Grid container spacing={2}>
                    <Grid size={{ xs: 12, sm: 6 }}>
                      <TextField fullWidth label="Access Key" onChange={(event) => updateS3Field('accessKey', event.target.value)} value={storagePolicy.s3?.accessKey || ''} />
                    </Grid>
                    <Grid size={{ xs: 12, sm: 6 }}>
                      <TextField fullWidth label="Secret Key" onChange={(event) => updateS3Field('secretKey', event.target.value)} type="password" value={storagePolicy.s3?.secretKey || ''} />
                    </Grid>
                  </Grid>
                  {storagePolicy.driver === 'minio' && (
                    <FormControl fullWidth>
                      <InputLabel>使用 SSL</InputLabel>
                      <Select label="使用 SSL" onChange={(event) => updateS3Field('useSsl', event.target.value === 'true')} value={String(storagePolicy.s3?.useSsl ?? false)}>
                        <MenuItem value="false">HTTP</MenuItem>
                        <MenuItem value="true">HTTPS</MenuItem>
                      </Select>
                    </FormControl>
                  )}
                </Stack>
              )}

              <TextField fullWidth label="公网访问地址（CDN URL）" onChange={(event) => setStoragePolicy((prev) => ({ ...prev, publicBaseUrl: event.target.value }))} value={storagePolicy.publicBaseUrl || ''} />

              {testResult && (
                <Alert severity={testResult.success ? 'success' : 'error'}>
                  {testResult.success ? '连接测试成功！可以正常读写。' : `连接测试失败：${testResult.error}`}
                </Alert>
              )}

              <Stack direction={{ xs: 'column', sm: 'row' }} sx={{ gap: 1, justifyContent: 'space-between' }}>
                <Typography color="text.secondary" variant="body2">
                  当前策略关联 {usage.objectCount} 条应用记录（{formatBytes(usage.sizeBytes)}）
                </Typography>
                <Stack direction="row" sx={{ gap: 1, justifyContent: 'flex-end' }}>
                  {storagePolicy.driver !== 'local' && (
                    <Button disabled={testing} onClick={handleTestConnection} size="small" variant="outlined">
                      {testing ? '测试中...' : '测试连接'}
                    </Button>
                  )}
                  <Button disabled={storageSaving} onClick={handleStorageSave} variant="contained">
                    {storageSaving ? '保存中...' : '保存存储策略'}
                  </Button>
                </Stack>
              </Stack>
            </Stack>
          )}

          {activeSection === 'maintenance' && (
            <StorageMaintenancePanel />
          )}

          {activeSection === 'security' && (
            <AccountSecurityPanel />
          )}
        </Paper>
      </Stack>
    </Stack>
  );
}

function resolveThemePreview(site: SiteSettings, colorScheme: string | undefined) {
  const isDark = site.themeMode === 'dark' || (site.themeMode === 'system' && colorScheme === 'dark');
  const preset = appPalettes.find((item) => item.key === site.themePreset);
  const primary = site.themePreset === 'custom' ? normalizeThemeColor(site.themePrimaryColor) : preset?.swatch ?? '#2563eb';
  const displayPrimary = isDark ? mixPreviewColor(primary, '#ffffff', 0.38) : primary;

  return {
    background: isDark ? '#0f172a' : '#f8fafc',
    border: isDark ? 'rgba(203, 213, 225, 0.24)' : '#d8dee8',
    contrastText: isDark ? '#07111f' : '#ffffff',
    primary: displayPrimary,
    secondary: isDark ? '#d5deeb' : '#475569',
    selected: withPreviewAlpha(displayPrimary, isDark ? 0.24 : 0.12),
    text: isDark ? '#f8fafc' : '#111827'
  };
}

function FooterPreview({ sections, site }: { sections: FooterSection[]; site: SiteSettings }) {
  const siteName = site.name || fallbackSite.name;
  const subtitle = site.subtitle || fallbackSite.subtitle;
  const [introSection, ...detailSections] = sections;

  return (
    <Box sx={{ bgcolor: '#030712', border: '1px solid rgba(255, 255, 255, 0.12)', borderRadius: 2, color: 'rgba(255, 255, 255, 0.76)', p: 2.5 }}>
      <Typography sx={{ color: '#ffffff', fontWeight: 700, mb: 2 }} variant="body2">页脚预览</Typography>
      <Stack direction={{ xs: 'column', md: 'row' }} sx={{ alignItems: 'flex-start', gap: { xs: 3, md: 4 } }}>
        <Stack sx={{ flex: 1.1, gap: 1.75, maxWidth: 520, minWidth: { xs: '100%', md: 0 } }}>
          <Stack direction="row" sx={{ alignItems: 'center', gap: 1.5, minWidth: 0 }}>
            {site.logoUrl ? (
              <Box alt={siteName} component="img" src={site.logoUrl} sx={{ flexShrink: 0, height: 40, maxWidth: 150, objectFit: 'contain' }} />
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
                  fontWeight: 900,
                  height: 40,
                  justifyContent: 'center',
                  width: 40
                }}
              >
                {siteName.slice(0, 1).toUpperCase()}
              </Box>
            )}
            <Box sx={{ minWidth: 0 }}>
              <Typography noWrap sx={{ color: '#ffffff', fontWeight: 900 }} variant="subtitle1">{siteName}</Typography>
              <Typography sx={{ color: 'rgba(255, 255, 255, 0.58)', lineHeight: 1.5 }} variant="body2">{subtitle}</Typography>
            </Box>
          </Stack>
          <FooterPreviewColumn section={introSection} showEmpty />
        </Stack>
        <Stack direction={{ xs: 'column', sm: 'row' }} sx={{ flex: 1.25, gap: { xs: 3, sm: 4 }, minWidth: 0, width: '100%' }}>
          {detailSections.map((section, index) => (
            <FooterPreviewColumn key={`footer-preview-${index}`} section={section} showEmpty />
          ))}
        </Stack>
      </Stack>
    </Box>
  );
}

function FooterPreviewColumn({ section, showEmpty = false }: { section?: FooterSection; showEmpty?: boolean }) {
  const title = section?.title.trim() || '未命名分区';
  const html = section?.html.trim() || (showEmpty ? '<p>暂无内容</p>' : '');
  return (
    <Stack sx={{ flex: 1, gap: 1.1, minWidth: 0 }}>
      <Typography sx={{ color: '#ffffff', fontSize: '0.78rem', fontWeight: 900 }} variant="overline">{title}</Typography>
      <Box dangerouslySetInnerHTML={{ __html: sanitizeFooterHtml(html) }} sx={footerPreviewHtmlSx} />
    </Stack>
  );
}

function ContactWidgetPreview({ site }: { site: SiteSettings }) {
  const html = site.contactWidgetHtml.trim() || '<p>暂无内容</p>';
  return (
    <Box sx={{ bgcolor: 'background.default', border: '1px dashed', borderColor: 'divider', borderRadius: 2, p: 2 }}>
      <Stack direction="row" sx={{ alignItems: 'center', gap: 1, justifyContent: 'space-between', mb: 1 }}>
        <Typography sx={{ fontWeight: 900 }} variant="subtitle2">
          {site.contactWidgetTitle || '联系我'}
        </Typography>
        <Typography color={site.contactWidgetEnabled ? 'success.main' : 'text.secondary'} variant="caption">
          {site.contactWidgetEnabled ? '已启用' : '未启用'}
        </Typography>
      </Stack>
      <Box dangerouslySetInnerHTML={{ __html: sanitizeFooterHtml(html) }} sx={contactWidgetPreviewHtmlSx} />
    </Box>
  );
}

function normalizeFooterSections(sections: FooterSection[] | undefined) {
  const normalized: FooterSection[] = [];
  for (let index = 0; index < 3; index += 1) {
    const source = sections?.[index] ?? fallbackFooterSections[index];
    normalized.push({
      html: source.html.trim(),
      title: source.title.trim()
    });
  }
  return normalized;
}

const footerPreviewHtmlSx = {
  color: 'rgba(255, 255, 255, 0.72)',
  fontSize: '0.875rem',
  lineHeight: 1.75,
  overflowWrap: 'anywhere',
  '& :first-of-type': { mt: 0 },
  '& :last-child': { mb: 0 },
  '& a': { color: 'rgba(255, 255, 255, 0.8)', textDecoration: 'none' },
  '& p': { mb: 1, mt: 0 },
  '& ul, & ol': { mb: 1, mt: 0, pl: 2.2 },
  '& li': { mb: 0.5 },
  '& strong, & b': { color: '#ffffff', fontWeight: 800 },
  '& img': { borderRadius: 1, display: 'block', height: 'auto', maxHeight: 110, maxWidth: '100%', objectFit: 'contain' },
  '& h1, & h2, & h3, & h4, & h5, & h6': { color: '#ffffff', fontSize: '0.95rem', fontWeight: 800, lineHeight: 1.4, mb: 1, mt: 0 }
};

const contactWidgetPreviewHtmlSx = {
  color: 'text.secondary',
  fontSize: '0.875rem',
  lineHeight: 1.7,
  overflowWrap: 'anywhere',
  '& :first-of-type': { mt: 0 },
  '& :last-child': { mb: 0 },
  '& a': { color: 'primary.main', textDecoration: 'none' },
  '& p': { mb: 1, mt: 0 },
  '& ul, & ol': { mb: 1, mt: 0, pl: 2.2 },
  '& li': { mb: 0.5 },
  '& strong, & b': { color: 'text.primary', fontWeight: 800 },
  '& img': { border: '1px solid', borderColor: 'divider', borderRadius: 1.5, display: 'block', height: 'auto', maxHeight: 180, maxWidth: '100%', objectFit: 'contain' },
  '& h1, & h2, & h3, & h4, & h5, & h6': { color: 'text.primary', fontSize: '1rem', fontWeight: 900, lineHeight: 1.35, mb: 1, mt: 0 }
};

function applyThemePreviewVariables(node: HTMLElement, preview: ReturnType<typeof resolveThemePreview>) {
  node.style.setProperty('--theme-preview-background', preview.background);
  node.style.setProperty('--theme-preview-border', preview.border);
  node.style.setProperty('--theme-preview-contrast', preview.contrastText);
  node.style.setProperty('--theme-preview-primary', preview.primary);
  node.style.setProperty('--theme-preview-secondary', preview.secondary);
  node.style.setProperty('--theme-preview-selected', preview.selected);
  node.style.setProperty('--theme-preview-text', preview.text);
}

function withPreviewAlpha(hex: string, alpha: number) {
  const rgb = previewHexToRgb(hex);
  return `rgba(${rgb.r}, ${rgb.g}, ${rgb.b}, ${alpha})`;
}

function mixPreviewColor(hex: string, targetHex: string, amount: number) {
  const color = previewHexToRgb(hex);
  const target = previewHexToRgb(targetHex);
  const mixed = {
    r: Math.round(color.r + (target.r - color.r) * amount),
    g: Math.round(color.g + (target.g - color.g) * amount),
    b: Math.round(color.b + (target.b - color.b) * amount)
  };
  return `#${[mixed.r, mixed.g, mixed.b].map((channel) => channel.toString(16).padStart(2, '0')).join('')}`;
}

function previewHexToRgb(hex: string) {
  const normalized = normalizeThemeColor(hex).slice(1);
  return {
    r: parseInt(normalized.slice(0, 2), 16),
    g: parseInt(normalized.slice(2, 4), 16),
    b: parseInt(normalized.slice(4, 6), 16)
  };
}

function BackgroundPanel({
  aspectRatio,
  disabled,
  label,
  onClear,
  onSelect,
  processingText,
  url
}: {
  aspectRatio: string;
  disabled: boolean;
  label: string;
  onClear: () => void;
  onSelect: (event: ChangeEvent<HTMLInputElement>) => void;
  processingText: string;
  url: string;
}) {
  return (
    <Stack sx={{ gap: 1.25 }}>
      <Box
        sx={{
          alignItems: 'center',
          aspectRatio,
          backgroundImage: url ? `url("${url}")` : undefined,
          backgroundPosition: 'center',
          backgroundRepeat: 'no-repeat',
          backgroundSize: 'cover',
          bgcolor: 'action.hover',
          border: '1px solid',
          borderColor: 'divider',
          borderRadius: 2,
          display: 'flex',
          justifyContent: 'center',
          overflow: 'hidden',
          p: 2
        }}
      >
        {!url && <Typography color="text.secondary">{label}</Typography>}
      </Box>
      <Stack direction="row" sx={{ alignItems: 'center', gap: 1, justifyContent: 'space-between' }}>
        <Box>
          <Typography sx={{ fontWeight: 800 }}>{label}</Typography>
          <Typography color="text.secondary" variant="caption">{processingText}</Typography>
        </Box>
        <Stack direction="row" sx={{ gap: 1 }}>
          <Button component="label" disabled={disabled} size="small" variant="outlined">
            {disabled ? '上传中...' : url ? '更换' : '上传'}
            <input accept="image/*" hidden onChange={onSelect} type="file" />
          </Button>
          {url && <Button color="secondary" onClick={onClear} size="small">清空</Button>}
        </Stack>
      </Stack>
    </Stack>
  );
}

function normalizeSection(section: string | undefined): SettingsSection {
  if (section === 'password') return 'security';
  return isSettingsSection(section) ? section : 'site';
}

function isSettingsSection(section: string | undefined): section is SettingsSection {
  return section === 'site' || section === 'theme' || section === 'background' || section === 'contact' || section === 'footer' || section === 'upload' || section === 'storage' || section === 'maintenance' || section === 'security';
}

function AccountSecurityPanel() {
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [oidcStatus, setOIDCStatus] = useState<OIDCStatus | null>(null);
  const [message, setMessage] = useState('');
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);
  const [oidcLoading, setOIDCLoading] = useState(false);

  function loadOIDCStatus() {
    getOIDCStatus()
      .then(setOIDCStatus)
      .catch(() => setOIDCStatus(null));
  }

  useEffect(() => {
    loadOIDCStatus();
    const params = new URLSearchParams(window.location.search);
    const success = params.get('oidc_success');
    const oidcError = params.get('oidc_error');
    if (success) setMessage(success);
    if (oidcError) setError(oidcError);
    if (success || oidcError) {
      window.history.replaceState({}, '', window.location.pathname);
    }
  }, []);

  async function handleSubmit(formEvent: FormEvent<HTMLFormElement>) {
    formEvent.preventDefault();
    setError('');
    setMessage('');
    if (!currentPassword) {
      setError('请输入当前密码');
      return;
    }
    if (newPassword.length < 8) {
      setError('新密码至少需要 8 位字符');
      return;
    }
    if (newPassword !== confirmPassword) {
      setError('两次输入的新密码不一致');
      return;
    }
    if (currentPassword === newPassword) {
      setError('新密码不能与当前密码相同');
      return;
    }
    setSaving(true);
    try {
      const result = await changePassword(currentPassword, newPassword);
      setMessage(result.message);
      setCurrentPassword('');
      setNewPassword('');
      setConfirmPassword('');
    } catch (err) {
      setError(err instanceof Error ? err.message : '修改密码失败');
    } finally {
      setSaving(false);
    }
  }

  return (
    <Stack sx={{ gap: 3 }}>
      {message && <Alert severity="success">{message}</Alert>}
      {error && <Alert severity="error">{error}</Alert>}
      <Stack sx={{ gap: 2 }}>
        <Typography sx={{ fontWeight: 800 }} variant="h6">Keycloak 绑定</Typography>
        <Paper sx={{ p: 2 }} variant="outlined">
          <Stack direction={{ xs: 'column', sm: 'row' }} sx={{ alignItems: { xs: 'stretch', sm: 'center' }, gap: 2, justifyContent: 'space-between' }}>
            <Box>
              <Typography sx={{ fontWeight: 800 }}>
                {oidcStatus?.bound ? '已绑定' : oidcStatus?.enabled ? '未绑定' : 'OIDC 未启用'}
              </Typography>
              <Typography color="text.secondary" variant="body2">
                {oidcStatus?.bound ? `OIDC ID：${oidcStatus.subject}` : `绑定后可使用 ${oidcStatus?.providerName || 'Keycloak'} 登录后台`}
              </Typography>
            </Box>
            <Stack direction="row" sx={{ gap: 1 }}>
              {oidcStatus?.bound ? (
                <Button
                  color="error"
                  disabled={oidcLoading}
                  onClick={() => {
                    setOIDCLoading(true);
                    setError('');
                    unbindOIDC()
                      .then(() => { setMessage('Keycloak 绑定已解除。'); loadOIDCStatus(); })
                      .catch((err: unknown) => setError(err instanceof Error ? err.message : '解绑失败'))
                      .finally(() => setOIDCLoading(false));
                  }}
                  variant="outlined"
                >
                  解绑
                </Button>
              ) : (
                <Button
                  disabled={!oidcStatus?.enabled || oidcLoading}
                  onClick={() => {
                    setOIDCLoading(true);
                    getOIDCBindURL()
                      .then((result) => { window.location.href = result.url; })
                      .catch((err: unknown) => { setError(err instanceof Error ? err.message : '发起绑定失败'); setOIDCLoading(false); });
                  }}
                  variant="contained"
                >
                  绑定 {oidcStatus?.providerName || 'Keycloak'}
                </Button>
              )}
            </Stack>
          </Stack>
        </Paper>
      </Stack>
      <Stack component="form" onSubmit={handleSubmit} sx={{ gap: 2.5 }}>
        <Typography sx={{ fontWeight: 800 }} variant="h6">修改密码</Typography>
        <Typography color="text.secondary">
          修改管理员登录密码。你需要先输入当前密码进行验证。
        </Typography>
        <TextField
          autoComplete="current-password"
          fullWidth
          label="当前密码"
          onChange={(e) => setCurrentPassword(e.target.value)}
          type="password"
          value={currentPassword}
        />
        <TextField
          autoComplete="new-password"
          fullWidth
          helperText="至少 8 位字符"
          label="新密码"
          onChange={(e) => setNewPassword(e.target.value)}
          type="password"
          value={newPassword}
        />
        <TextField
          autoComplete="new-password"
          fullWidth
          label="确认新密码"
          onChange={(e) => setConfirmPassword(e.target.value)}
          type="password"
          value={confirmPassword}
        />
        <Box>
          <Button disabled={saving} type="submit" variant="contained">
            {saving ? '保存中...' : '修改密码'}
          </Button>
        </Box>
      </Stack>
    </Stack>
  );
}

function StorageMaintenancePanel() {
  const [orphanResult, setOrphanResult] = useState<Awaited<ReturnType<typeof scanOrphanStorageObjects>> | null>(null);
  const [thumbnailResult, setThumbnailResult] = useState<Awaited<ReturnType<typeof scanMissingThumbnails>> | null>(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState('');

  async function runScan(kind: 'orphans' | 'thumbnails') {
    setError('');
    setLoading(kind);
    try {
      if (kind === 'orphans') setOrphanResult(await scanOrphanStorageObjects());
      else setThumbnailResult(await scanMissingThumbnails());
    } catch (err) {
      setError(err instanceof Error ? err.message : '扫描失败');
    } finally {
      setLoading('');
    }
  }

  return (
    <Stack sx={{ gap: 2.5 }}>
      {error && <Alert severity="error">{error}</Alert>}
      <Paper sx={{ p: 2 }} variant="outlined">
        <Stack direction={{ xs: 'column', sm: 'row' }} sx={{ alignItems: { xs: 'stretch', sm: 'center' }, gap: 2, justifyContent: 'space-between' }}>
          <Box>
            <Typography sx={{ fontWeight: 800 }}>孤儿文件扫描</Typography>
            <Typography color="text.secondary" variant="body2">扫描本地存储中未被数据库引用的文件，只生成报告，不自动删除。</Typography>
          </Box>
          <Button disabled={loading === 'orphans'} onClick={() => void runScan('orphans')} variant="contained">{loading === 'orphans' ? '扫描中...' : '开始扫描'}</Button>
        </Stack>
        {orphanResult && (
          <Box sx={{ mt: 2 }}>
            <Typography color="text.secondary" variant="body2">
              发现 {orphanResult.total} 个可疑文件，约 {formatBytes(orphanResult.totalSizeBytes)}；已扫描 {orphanResult.scannedPolicies} 个本地策略，跳过 {orphanResult.skippedPolicies} 个非本地策略。
            </Typography>
            {orphanResult.items.slice(0, 20).map((item) => (
              <Typography key={`${item.policyId}-${item.key}`} sx={{ display: 'block', overflowWrap: 'anywhere' }} variant="caption">
                {item.policyId}: {item.key}
              </Typography>
            ))}
          </Box>
        )}
      </Paper>
      <Paper sx={{ p: 2 }} variant="outlined">
        <Stack direction={{ xs: 'column', sm: 'row' }} sx={{ alignItems: { xs: 'stretch', sm: 'center' }, gap: 2, justifyContent: 'space-between' }}>
          <Box>
            <Typography sx={{ fontWeight: 800 }}>缩略图缺失扫描</Typography>
            <Typography color="text.secondary" variant="body2">查找图片记录中没有缩略图键的正式图片和待审投稿。</Typography>
          </Box>
          <Button disabled={loading === 'thumbnails'} onClick={() => void runScan('thumbnails')} variant="contained">{loading === 'thumbnails' ? '扫描中...' : '开始扫描'}</Button>
        </Stack>
        {thumbnailResult && (
          <Box sx={{ mt: 2 }}>
            <Typography color="text.secondary" variant="body2">发现 {thumbnailResult.total} 条记录。</Typography>
            {thumbnailResult.items.slice(0, 20).map((item) => (
              <Typography key={`${item.kind}-${item.id}`} sx={{ display: 'block', overflowWrap: 'anywhere' }} variant="caption">
                {item.kind} #{item.id} / event #{item.eventId}: {item.objectKey}
              </Typography>
            ))}
          </Box>
        )}
      </Paper>
    </Stack>
  );
}

function formatBytes(bytes: number) {
  if (!bytes) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  return `${(bytes / 1024 ** index).toFixed(index === 0 ? 0 : 1)} ${units[index]}`;
}
