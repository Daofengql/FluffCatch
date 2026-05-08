import { Login, Refresh } from '@mui/icons-material';
import { Alert, Box, Button, Divider, IconButton, Paper, Stack, TextField, Typography } from '@mui/material';
import { type FormEvent, useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { getCaptcha, getOIDCLoginURL, getPublicOIDCSettings, type CaptchaChallenge, type PublicOIDCSettings } from '../../api/client';
import { getCachedMe, loginAdmin, refreshMe } from '../../api/authState';

export function LoginPage() {
  const navigate = useNavigate();
  const [error, setError] = useState('');
  const [message, setMessage] = useState('');
  const [captcha, setCaptcha] = useState<CaptchaChallenge | null>(null);
  const [oidcSettings, setOIDCSettings] = useState<PublicOIDCSettings | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [oidcSubmitting, setOIDCSubmitting] = useState(false);

  async function refreshCaptcha() {
    setCaptcha(await getCaptcha());
  }

  useEffect(() => {
    let cancelled = false;
    const params = new URLSearchParams(window.location.search);
    const oidcSuccess = params.get('oidc_success');
    const oidcError = params.get('oidc_error');

    if (oidcSuccess || oidcError) {
      window.history.replaceState({}, '', window.location.pathname);
    }

    if (oidcError) {
      setError(oidcError);
    }

    getPublicOIDCSettings()
      .then((settings) => {
        if (!cancelled) setOIDCSettings(settings);
      })
      .catch(() => {
        if (!cancelled) setOIDCSettings({ enabled: false, providerName: 'Keycloak' });
      });

    if (oidcSuccess) {
      setMessage(oidcSuccess);
      refreshMe(true)
        .then((payload) => {
          if (!cancelled && payload.authenticated) {
            navigate('/admin/events', { replace: true });
          }
        })
        .catch(() => {
          if (!cancelled) setError('OIDC 登录状态校验失败，请重试');
        });
      return () => {
        cancelled = true;
      };
    }

    if (getCachedMe().authenticated) {
      navigate('/admin/events', { replace: true });
      return () => {
        cancelled = true;
      };
    }

    refreshMe()
      .then((payload) => {
        if (cancelled) return;
        if (payload.authenticated) {
          navigate('/admin/events', { replace: true });
          return;
        }
        refreshCaptcha().catch(() => setError('验证码加载失败'));
      })
      .catch(() => {
        if (!cancelled) {
          refreshCaptcha().catch(() => setError('验证码加载失败'));
        }
      });

    return () => {
      cancelled = true;
    };
  }, [navigate]);

  async function handleOIDCLogin() {
    if (oidcSubmitting) return;
    setError('');
    setMessage('');
    setOIDCSubmitting(true);
    try {
      const result = await getOIDCLoginURL();
      window.location.href = result.url;
    } catch (err) {
      setError(err instanceof Error ? err.message : '发起 OIDC 登录失败');
      setOIDCSubmitting(false);
    }
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submitting) return;
    setError('');
    setSubmitting(true);
    const formData = new FormData(event.currentTarget);
    try {
      await loginAdmin(
        String(formData.get('username') || ''),
        String(formData.get('password') || ''),
        captcha?.id || '',
        String(formData.get('captchaAnswer') || '')
      );
      navigate('/admin/events', { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : '登录失败');
      refreshCaptcha().catch(() => undefined);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Box sx={{ alignItems: 'center', display: 'flex', justifyContent: 'center', width: '100%' }}>
      <Paper component="form" elevation={3} onSubmit={handleSubmit} sx={{ maxWidth: 460, p: { xs: 3, sm: 4 }, width: '100%' }}>
        <Stack sx={{ gap: 2.5 }}>
          <Box>
            <Typography sx={{ fontWeight: 800 }} variant="h4">
              管理员登录
            </Typography>
            <Typography color="text.secondary" sx={{ mt: 0.5 }}>
              输入账号、密码和图片验证码进入后台。
            </Typography>
          </Box>
          {message && <Alert severity="success">{message}</Alert>}
          {error && <Alert severity="error">{error}</Alert>}
          <TextField autoComplete="username" label="用户名" name="username" />
          <TextField autoComplete="current-password" label="密码" name="password" type="password" />
          <Stack direction="row" sx={{ alignItems: 'center', gap: 1 }}>
            <TextField fullWidth label="验证码" name="captchaAnswer" />
            <Box
              dangerouslySetInnerHTML={{ __html: captcha?.imageSvg || '' }}
              sx={{
                alignItems: 'center',
                border: '1px solid',
                borderColor: 'grey.300',
                borderRadius: 2,
                display: 'flex',
                height: 54,
                justifyContent: 'center',
                minWidth: 150,
                overflow: 'hidden'
              }}
            />
            <IconButton aria-label="刷新验证码" onClick={() => refreshCaptcha().catch(() => setError('验证码刷新失败'))}>
              <Refresh />
            </IconButton>
          </Stack>
          <Button disabled={!captcha || submitting} size="large" type="submit" variant="contained">
            {submitting ? '登录中...' : '登录'}
          </Button>
          {oidcSettings?.enabled && (
            <>
              <Divider>或</Divider>
              <Button disabled={oidcSubmitting} onClick={() => void handleOIDCLogin()} size="large" startIcon={<Login />} variant="outlined">
                {oidcSubmitting ? '跳转中...' : `使用 ${oidcSettings.providerName || 'Keycloak'} 登录`}
              </Button>
            </>
          )}
        </Stack>
      </Paper>
    </Box>
  );
}
