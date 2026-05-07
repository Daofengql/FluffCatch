import MDEditor from '@uiw/react-md-editor';
import { Alert, Avatar, Box, Button, Card, CardContent, Chip, Divider, FormControl, Grid, InputLabel, MenuItem, Paper, Select, Stack, TextField, Typography } from '@mui/material';
import type { SelectChangeEvent } from '@mui/material/Select';
import { type ChangeEvent, type FormEvent, useEffect, useState } from 'react';
import { useLocation } from 'react-router-dom';
import {
  clearSiteLogo,
  getAdminSettings,
  testStorageConnection,
  updateSiteSettings,
  updateStoragePolicies,
  updateUploadSettings,
  uploadSiteLogo,
  type AdminSettingsResponse,
  type SiteSettings,
  type S3Settings,
  type StorageDriver,
  type StoragePolicy,
  type UploadSettings
} from '../../api/client';
import { PageHeader } from '../../components/common/PageHeader';

const fallbackSite: SiteSettings = {
  name: 'FluffCatch',
  subtitle: '兽聚返图收集与画廊',
  logoUrl: '',
  homeMarkdown: ''
};

const fallbackUpload: UploadSettings = {
  maxFileSizeMb: 20,
  maxFilesPerUpload: 20
};

const emptyS3: S3Settings = { endpoint: '', bucket: '', region: '', accessKey: '', secretKey: '', useSsl: false, accountId: '' };

const driverOptions: { value: StorageDriver; label: string }[] = [
  { value: 'local', label: '本地存储' },
  { value: 'minio', label: 'MinIO' },
  { value: 'aws-s3', label: 'AWS S3' },
  { value: 'aliyun-oss', label: '阿里云 OSS' },
  { value: 'tencent-cos', label: '腾讯云 COS' },
  { value: 'cf-r2', label: 'Cloudflare R2' },
];

export function AdminSettingsPage() {
  const location = useLocation();
  const [settings, setSettings] = useState<AdminSettingsResponse | null>(null);
  const [site, setSite] = useState<SiteSettings>(fallbackSite);
  const [upload, setUpload] = useState<UploadSettings>(fallbackUpload);
  const [message, setMessage] = useState('');
  const [error, setError] = useState('');
  const [logoUploading, setLogoUploading] = useState(false);

  // Storage editor state
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

  function refresh() {
    getAdminSettings()
      .then((payload) => {
        setSettings(payload);
        setSite({ ...fallbackSite, ...payload.settings.site });
        setUpload({ ...fallbackUpload, ...payload.settings.upload });
        const policies = payload.settings.storagePolicies.policies;
        if (policies.length > 0) {
          setStoragePolicy(policies[0]);
        }
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
      setMessage('站点信息已保存。');
      refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : '站点信息保存失败');
    }
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
      setSite(result.site);
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
      setSite(result.site);
      setMessage('Logo 已清空。');
      refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Logo 清空失败');
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

  const isS3Driver = ['minio', 'aws-s3', 'aliyun-oss', 'tencent-cos', 'cf-r2', 's3'].includes(storagePolicy.driver);
  const usage = settings?.usage[storagePolicy.id] ?? { objectCount: 0, sizeBytes: 0 };

  return (
    <Stack sx={{ gap: 3 }}>
      <PageHeader subtitle="存储策略与运行时设置保存在数据库中" title="系统设置" />
      {message && <Alert onClose={() => setMessage('')} severity="success">{message}</Alert>}
      {error && <Alert onClose={() => setError('')} severity="error">{error}</Alert>}

      <Paper component="form" onSubmit={handleSiteSubmit} sx={{ p: 3 }}>
        <Stack sx={{ gap: 2.5 }}>
          <Stack direction={{ xs: 'column', sm: 'row' }} sx={{ alignItems: { xs: 'flex-start', sm: 'center' }, gap: 2 }}>
            <Avatar src={site.logoUrl || undefined} sx={{ bgcolor: 'primary.main', color: 'primary.contrastText', fontWeight: 900, height: 64, width: 64 }} variant="rounded">
              {(site.name || 'F').slice(0, 1).toUpperCase()}
            </Avatar>
            <Box>
              <Typography gutterBottom sx={{ fontWeight: 800 }} variant="h6">站点信息</Typography>
              <Typography color="text.secondary">公开页导航栏会显示站点名称、副标题与 Logo；首页介绍卡片支持 Markdown。</Typography>
              <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 1, mt: 1.5 }}>
                <Button component="label" disabled={logoUploading} size="small" variant="outlined">
                  {logoUploading ? '上传中...' : site.logoUrl ? '更换 Logo' : '上传 Logo'}
                  <input accept="image/*" hidden onChange={handleLogoSelect} type="file" />
                </Button>
                {site.logoUrl && (
                  <Button color="secondary" onClick={() => void handleClearLogo()} size="small">清空 Logo</Button>
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
                <MDEditor height={280} onChange={(value) => setSite((prev) => ({ ...prev, homeMarkdown: value || '' }))} preview="edit" value={site.homeMarkdown} />
              </Box>
              <Typography color="text.secondary" sx={{ mt: 1 }} variant="caption">
                支持标题、列表、链接、表格等常用 Markdown。出于安全考虑，公开页会跳过原始 HTML。
              </Typography>
            </Grid>
          </Grid>
          <Box>
            <Button type="submit" variant="contained">保存站点信息</Button>
          </Box>
        </Stack>
      </Paper>

      <Paper component="form" onSubmit={handleUploadSubmit} sx={{ p: 3 }}>
        <Stack sx={{ gap: 2.5 }}>
          <Box>
            <Typography gutterBottom sx={{ fontWeight: 800 }} variant="h6">上传限制</Typography>
            <Typography color="text.secondary">控制投稿与管理员直传的单文件大小、单次上传数量。默认均为 20。</Typography>
          </Box>
          <Grid container spacing={2}>
            <Grid size={{ xs: 12, sm: 6 }}>
              <TextField
                fullWidth
                label="单文件最大大小（MB）"
                onChange={(e) => setUpload((prev) => ({ ...prev, maxFileSizeMb: Number(e.target.value) }))}
                slotProps={{ htmlInput: { min: 1, max: 1024, step: 1 } }}
                type="number"
                value={upload.maxFileSizeMb}
              />
            </Grid>
            <Grid size={{ xs: 12, sm: 6 }}>
              <TextField
                fullWidth
                label="单次上传最大数量"
                onChange={(e) => setUpload((prev) => ({ ...prev, maxFilesPerUpload: Number(e.target.value) }))}
                slotProps={{ htmlInput: { min: 1, max: 200, step: 1 } }}
                type="number"
                value={upload.maxFilesPerUpload}
              />
            </Grid>
          </Grid>
          <Box>
            <Button type="submit" variant="contained">保存上传限制</Button>
          </Box>
        </Stack>
      </Paper>

      <Paper sx={{ p: 3 }}>
        <Typography gutterBottom sx={{ fontWeight: 800 }} variant="h6">存储策略</Typography>
        <Typography color="text.secondary" sx={{ mb: 2 }}>
          同一时间只启用一个存储策略。切换存储只影响新上传文件；已有文件按记录中的策略地址读取，无需主动迁移。
        </Typography>
        <Divider sx={{ mb: 2 }} />

        <Stack sx={{ gap: 2.5 }}>
          <Grid container spacing={2}>
            <Grid size={{ xs: 12, sm: 6 }}>
              <TextField fullWidth label="策略名称" onChange={(e) => setStoragePolicy((prev) => ({ ...prev, name: e.target.value }))} value={storagePolicy.name} />
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
            <TextField fullWidth label="本地存储路径" onChange={(e) => setStoragePolicy((prev) => ({ ...prev, localPath: e.target.value }))} value={storagePolicy.localPath || ''} helperText="例如 data/uploads" />
          )}

          {isS3Driver && (
            <Stack sx={{ gap: 2 }}>
              {storagePolicy.driver === 'cf-r2' && (
                <TextField fullWidth label="Cloudflare Account ID" onChange={(e) => updateS3Field('accountId', e.target.value)} value={storagePolicy.s3?.accountId || ''} helperText="在 Cloudflare Dashboard 右侧找到 Account ID" />
              )}
              {(storagePolicy.driver === 'minio' || storagePolicy.driver === 'aliyun-oss' || storagePolicy.driver === 'tencent-cos' || storagePolicy.driver === 's3') && (
                <TextField fullWidth label="Endpoint" onChange={(e) => updateS3Field('endpoint', e.target.value)} value={storagePolicy.s3?.endpoint || ''} helperText={
                  storagePolicy.driver === 'aliyun-oss' ? '例如 oss-cn-hangzhou.aliyuncs.com' :
                  storagePolicy.driver === 'tencent-cos' ? '例如 https://bucket-1250000000.cos.ap-guangzhou.myqcloud.com' :
                  storagePolicy.driver === 'minio' ? '例如 minio.example.com:9000' :
                  'S3 兼容服务的 Endpoint 地址'
                } />
              )}
              <TextField fullWidth label="Bucket" onChange={(e) => updateS3Field('bucket', e.target.value)} value={storagePolicy.s3?.bucket || ''} />
              {(storagePolicy.driver !== 'cf-r2') && (
                <TextField fullWidth label="Region" onChange={(e) => updateS3Field('region', e.target.value)} value={storagePolicy.s3?.region || ''} helperText={storagePolicy.driver === 'aws-s3' ? '例如 us-east-1、ap-northeast-1' : '可留空使用默认值'} />
              )}
              <Grid container spacing={2}>
                <Grid size={{ xs: 12, sm: 6 }}>
                  <TextField fullWidth label="Access Key" onChange={(e) => updateS3Field('accessKey', e.target.value)} value={storagePolicy.s3?.accessKey || ''} />
                </Grid>
                <Grid size={{ xs: 12, sm: 6 }}>
                  <TextField fullWidth label="Secret Key" onChange={(e) => updateS3Field('secretKey', e.target.value)} type="password" value={storagePolicy.s3?.secretKey || ''} />
                </Grid>
              </Grid>
              {storagePolicy.driver === 'minio' && (
                <FormControl fullWidth>
                  <InputLabel>使用 SSL</InputLabel>
                  <Select label="使用 SSL" onChange={(e) => updateS3Field('useSsl', e.target.value === 'true')} value={String(storagePolicy.s3?.useSsl ?? false)}>
                    <MenuItem value="false">HTTP</MenuItem>
                    <MenuItem value="true">HTTPS</MenuItem>
                  </Select>
                </FormControl>
              )}
            </Stack>
          )}

          <TextField fullWidth label="公网访问地址（CDN URL）" onChange={(e) => setStoragePolicy((prev) => ({ ...prev, publicBaseUrl: e.target.value }))} value={storagePolicy.publicBaseUrl || ''} helperText={
            storagePolicy.driver === 'local' ? '留空则通过后端 /media 路径提供文件' :
            '文件对外访问的公网地址前缀，例如 https://cdn.example.com'
          } />

          {testResult && (
            <Alert severity={testResult.success ? 'success' : 'error'}>
              {testResult.success ? '连接测试成功！可以正常读写。' : `连接测试失败：${testResult.error}`}
            </Alert>
          )}

          <Stack direction="row" sx={{ gap: 1, justifyContent: 'space-between', flexWrap: 'wrap' }}>
            <Stack direction="row" sx={{ gap: 1, alignItems: 'center' }}>
              {usage.objectCount > 0 && (
                <Typography color="text.secondary" variant="body2">
                  当前策略已存储 {usage.objectCount} 个对象（{formatBytes(usage.sizeBytes)}）
                </Typography>
              )}
            </Stack>
            <Stack direction="row" sx={{ gap: 1 }}>
              {storagePolicy.driver !== 'local' && (
                <Button disabled={testing} onClick={handleTestConnection} variant="outlined" size="small">
                  {testing ? '测试中...' : '测试连接'}
                </Button>
              )}
              <Button disabled={storageSaving} onClick={handleStorageSave} variant="contained">
                {storageSaving ? '保存中...' : '保存存储策略'}
              </Button>
            </Stack>
          </Stack>
        </Stack>
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
