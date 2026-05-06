import { Refresh } from '@mui/icons-material';
import { Alert, Box, Button, IconButton, Paper, Stack, TextField, Typography } from '@mui/material';
import { type FormEvent, useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { getCaptcha, getMe, login, type CaptchaChallenge } from '../../api/client';

export function LoginPage() {
  const navigate = useNavigate();
  const [error, setError] = useState('');
  const [captcha, setCaptcha] = useState<CaptchaChallenge | null>(null);

  async function refreshCaptcha() {
    setCaptcha(await getCaptcha());
  }

  useEffect(() => {
    let cancelled = false;

    getMe()
      .then((payload) => {
        if (cancelled) return;
        if (payload.authenticated) {
          navigate('/admin/dashboard', { replace: true });
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

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError('');
    const formData = new FormData(event.currentTarget);
    try {
      await login(
        String(formData.get('username') || ''),
        String(formData.get('password') || ''),
        captcha?.id || '',
        String(formData.get('captchaAnswer') || '')
      );
      navigate('/admin/dashboard', { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : '登录失败');
      refreshCaptcha().catch(() => undefined);
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
          <Button disabled={!captcha} size="large" type="submit" variant="contained">
            登录
          </Button>
        </Stack>
      </Paper>
    </Box>
  );
}
