import { Refresh } from '@mui/icons-material';
import { Box, Button, Dialog, DialogActions, DialogContent, DialogTitle, IconButton, Stack, TextField, Typography } from '@mui/material';
import { type FormEvent, useEffect, useState } from 'react';
import { captchaHeaders, getCaptcha, type CaptchaChallenge } from '../api/client';

type ConfirmDialogProps = {
  onCancel: () => void;
  onConfirm: (headers: Record<string, string>) => void;
  open: boolean;
  subtitle?: string;
  title: string;
};

export function ConfirmDialog({ onCancel, onConfirm, open, subtitle, title }: ConfirmDialogProps) {
  const [captcha, setCaptcha] = useState<CaptchaChallenge | null>(null);
  const [answer, setAnswer] = useState('');
  const [error, setError] = useState('');

  function refreshCaptcha() {
    getCaptcha()
      .then(setCaptcha)
      .catch(() => setError('验证码加载失败'));
  }

  useEffect(() => {
    if (open) {
      setAnswer('');
      setError('');
      refreshCaptcha();
    }
  }, [open]);

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!captcha || !answer.trim()) {
      setError('请输入验证码');
      return;
    }
    onConfirm(captchaHeaders(captcha, answer.trim()));
  }

  return (
    <Dialog maxWidth="xs" onClose={onCancel} open={open}>
      <DialogTitle>{title}</DialogTitle>
      <DialogContent>
        <Stack component="form" id="confirm-dialog-form" onSubmit={handleSubmit} sx={{ gap: 2, pt: 0.5 }}>
          {subtitle && <Typography color="text.secondary" variant="body2">{subtitle}</Typography>}
          {error && <Typography color="error" variant="body2">{error}</Typography>}
          <Stack direction="row" sx={{ alignItems: 'center', gap: 1 }}>
            <TextField
              fullWidth
              label="验证码"
              onChange={(e) => { setAnswer(e.target.value); setError(''); }}
              size="small"
              value={answer}
            />
            <Box
              dangerouslySetInnerHTML={{ __html: captcha?.imageSvg || '' }}
              sx={{
                border: '1px solid',
                borderColor: 'grey.300',
                borderRadius: 2,
                display: 'flex',
                height: 40,
                justifyContent: 'center',
                minWidth: 120,
                overflow: 'hidden',
                '& svg': { height: 40, width: 120 }
              }}
            />
            <IconButton aria-label="刷新验证码" onClick={() => { setError(''); refreshCaptcha(); }} size="small">
              <Refresh fontSize="small" />
            </IconButton>
          </Stack>
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={onCancel}>取消</Button>
        <Button disabled={!captcha} form="confirm-dialog-form" type="submit" variant="contained" color="error">
          确认删除
        </Button>
      </DialogActions>
    </Dialog>
  );
}
