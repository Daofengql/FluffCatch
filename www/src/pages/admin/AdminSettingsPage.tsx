import MDEditor from '@uiw/react-md-editor';
import { Alert, Avatar, Box, Button, Card, CardContent, Chip, Divider, Grid, Paper, Stack, TextField, Typography } from '@mui/material';
import { type ChangeEvent, type FormEvent, useEffect, useState } from 'react';
import { useLocation } from 'react-router-dom';
import { clearSiteLogo, getAdminSettings, updateSiteSettings, uploadSiteLogo, type AdminSettingsResponse, type SiteSettings } from '../../api/client';
import { PageHeader } from '../../components/common/PageHeader';

const fallbackSite: SiteSettings = {
  name: 'FluffCatch',
  subtitle: '兽聚返图收集与画廊',
  logoUrl: '',
  homeMarkdown: ''
};

export function AdminSettingsPage() {
  const location = useLocation();
  const [settings, setSettings] = useState<AdminSettingsResponse | null>(null);
  const [site, setSite] = useState<SiteSettings>(fallbackSite);
  const [message, setMessage] = useState('');
  const [error, setError] = useState('');
  const [logoUploading, setLogoUploading] = useState(false);

  function refresh() {
    getAdminSettings()
      .then((payload) => {
        setSettings(payload);
        setSite({ ...fallbackSite, ...payload.settings.site });
      })
      .catch((err: unknown) => {
        setError(err instanceof Error ? err.message : '设置加载失败');
        setSettings(null);
      });
  }

  useEffect(refresh, [location.key]);

  function handleSiteChange(event: ChangeEvent<HTMLInputElement>) {
    setSite((prev) => ({ ...prev, [event.target.name]: event.target.value }));
  }

  async function handleSiteSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError('');
    setMessage('');

    try {
      const result = await updateSiteSettings(site);
      setSite(result.site);
      setMessage('站点信息已保存。刷新公开页即可看到新标题、Logo 与首页介绍。');
      refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : '站点信息保存失败');
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
      setSite(result.site);
      setMessage('Logo 已上传，并已写入站点设置。');
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
      setSite(result.site);
      setMessage('Logo 已清空。');
      refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Logo 清空失败');
    }
  }

  const storagePolicies = settings?.settings.storagePolicies.policies ?? [];
  const activePolicyId = settings?.settings.storagePolicies.activePolicyId ?? '';

  return (
    <Stack sx={{ gap: 3 }}>
      <PageHeader subtitle="存储策略与运行时设置保存在数据库中" title="系统设置" />
      {message && <Alert severity="success">{message}</Alert>}
      {error && <Alert severity="error">{error}</Alert>}
      <Paper component="form" onSubmit={handleSiteSubmit} sx={{ p: 3 }}>
        <Stack sx={{ gap: 2.5 }}>
          <Stack direction={{ xs: 'column', sm: 'row' }} sx={{ alignItems: { xs: 'flex-start', sm: 'center' }, gap: 2 }}>
            <Avatar
              src={site.logoUrl || undefined}
              sx={{
                bgcolor: 'primary.main',
                color: 'primary.contrastText',
                fontWeight: 900,
                height: 64,
                width: 64
              }}
              variant="rounded"
            >
              {(site.name || 'F').slice(0, 1).toUpperCase()}
            </Avatar>
            <Box>
              <Typography gutterBottom sx={{ fontWeight: 800 }} variant="h6">
                站点信息
              </Typography>
              <Typography color="text.secondary">
                公开页导航栏会显示站点名称、副标题与 Logo；首页介绍卡片支持 Markdown。
              </Typography>
              <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 1, mt: 1.5 }}>
                <Button component="label" disabled={logoUploading} size="small" variant="outlined">
                  {logoUploading ? '上传中...' : site.logoUrl ? '更换 Logo' : '上传 Logo'}
                  <input accept="image/*" hidden onChange={handleLogoSelect} type="file" />
                </Button>
                {site.logoUrl && (
                  <Button color="secondary" onClick={() => void handleClearLogo()} size="small">
                    清空 Logo
                  </Button>
                )}
              </Stack>
            </Box>
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
              <Box data-color-mode="light" sx={{ '& .w-md-editor': { boxShadow: 'none' }, '& .wmde-markdown': { bgcolor: 'transparent' } }}>
                <MDEditor
                  height={280}
                  onChange={(value) => setSite((prev) => ({ ...prev, homeMarkdown: value || '' }))}
                  preview="edit"
                  value={site.homeMarkdown}
                />
              </Box>
              <Typography color="text.secondary" sx={{ mt: 1 }} variant="caption">
                支持标题、列表、链接、表格等常用 Markdown。出于安全考虑，公开页会跳过原始 HTML。
              </Typography>
            </Grid>
          </Grid>
          <Box>
            <Button type="submit" variant="contained">
              保存站点信息
            </Button>
          </Box>
        </Stack>
      </Paper>
      <Paper sx={{ p: 3 }}>
        <Typography gutterBottom sx={{ fontWeight: 800 }} variant="h6">
          存储策略
        </Typography>
        <Typography color="text.secondary" sx={{ mb: 2 }}>
          切换默认策略只影响新上传文件；已有文件会继续按记录中的策略读取。
        </Typography>
        <Divider sx={{ mb: 2 }} />
        <Grid container spacing={2}>
          {storagePolicies.map((policy) => {
            const usage = settings?.usage[policy.id] ?? { objectCount: 0, sizeBytes: 0 };
            const active = activePolicyId === policy.id;
            return (
              <Grid key={policy.id} size={{ xs: 12, md: 6 }}>
                <Card>
                  <CardContent>
                    <Stack sx={{ gap: 1 }}>
                      <Stack direction="row" sx={{ alignItems: 'center', gap: 1 }}>
                        <Typography sx={{ fontWeight: 800 }}>{policy.name}</Typography>
                        {active && <Chip color="primary" label="当前默认" size="small" />}
                      </Stack>
                      <Typography color="text.secondary">策略 ID：{policy.id}</Typography>
                      <Typography color="text.secondary">类型：{policy.driver}</Typography>
                      <Typography color="text.secondary">直链：{policy.publicBaseUrl || '本地后端 /media'}</Typography>
                      <Typography color="text.secondary">对象数：{usage.objectCount}</Typography>
                      <Typography color="text.secondary">占用：{formatBytes(usage.sizeBytes)}</Typography>
                    </Stack>
                  </CardContent>
                </Card>
              </Grid>
            );
          })}
          {!storagePolicies.length && (
            <Grid size={{ xs: 12 }}>
              <Alert severity="info">还没有可用的存储策略。</Alert>
            </Grid>
          )}
        </Grid>
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
