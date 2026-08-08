import { Close } from '@mui/icons-material';
import { Alert, Box, Button, Dialog, DialogActions, DialogContent, DialogTitle, IconButton, Stack, TextField, Typography } from '@mui/material';
import { type FormEvent, useEffect, useState } from 'react';
import { resolveSubmissionToken, type EventCard, type SubmissionLink } from '../api/client';
import { getCachedMe, refreshMe, subscribeAuthState } from '../api/authState';
import { SubmissionForm } from './SubmissionForm';

type SubmissionDialogProps = {
  event: EventCard | null;
  open: boolean;
  onClose: () => void;
  onUploaded?: () => void;
};

export function SubmissionDialog({ event, onClose, onUploaded, open }: SubmissionDialogProps) {
  const [authenticated, setAuthenticated] = useState(() => getCachedMe().authenticated);
  const [submissionToken, setSubmissionToken] = useState('');
  const [submissionLink, setSubmissionLink] = useState<SubmissionLink | null>(null);
  const [photographerName, setPhotographerName] = useState('');
  const [tokenError, setTokenError] = useState('');
  const [verifyingToken, setVerifyingToken] = useState(false);
  const canUpload = authenticated || Boolean(submissionToken);

  useEffect(() => {
    const unsubscribe = subscribeAuthState((payload) => setAuthenticated(payload.authenticated));
    refreshMe().catch(() => undefined);
    return unsubscribe;
  }, []);

  useEffect(() => {
    if (!open) {
      setSubmissionToken('');
      setSubmissionLink(null);
      setPhotographerName('');
      setTokenError('');
      setVerifyingToken(false);
    }
  }, [open]);

  async function handleTokenSubmit(formEvent: FormEvent<HTMLFormElement>) {
    formEvent.preventDefault();
    if (!event || verifyingToken) return;
    const form = new FormData(formEvent.currentTarget);
    const code = normalizeSubmissionCode(String(form.get('submissionCode') || ''));
    if (!code) {
      setTokenError('请输入管理员提供的投稿码。');
      return;
    }
    setVerifyingToken(true);
    setTokenError('');
    try {
      const result = await resolveSubmissionToken(event.id, code);
      if (!result.valid) {
        setTokenError('投稿码无效、已过期或已达到使用次数。');
        return;
      }
      setSubmissionToken(code);
      setSubmissionLink(result.link || null);
      setPhotographerName(result.link?.photographerName || '');
    } catch {
      setTokenError('投稿码校验失败，请稍后重试。');
    } finally {
      setVerifyingToken(false);
    }
  }

  return (
    <Dialog
      fullScreen={false}
      fullWidth
      maxWidth="md"
      onClose={(_, reason) => {
        if (reason === 'backdropClick' || reason === 'escapeKeyDown') return;
        onClose();
      }}
      open={open}
      slotProps={{
        paper: {
          sx: {
            borderRadius: { xs: 0, sm: 3 },
            m: { xs: 0, sm: 4 },
            maxHeight: { xs: '100%', sm: 'calc(100% - 64px)' },
            width: { xs: '100%', sm: 'calc(100% - 64px)' }
          }
        }
      }}
    >
      <DialogTitle sx={{ pb: 1, px: { xs: 2, sm: 3 }, pt: { xs: 1.5, sm: 2 } }}>
        <Stack direction="row" sx={{ alignItems: 'flex-start', gap: 1.5 }}>
          <Box sx={{ flex: 1, minWidth: 0 }}>
            <Typography sx={{ fontWeight: 900 }} variant="h6">
              上传返图
            </Typography>
            {canUpload && (
              <Typography color="text.secondary" sx={{ mt: 0.5 }} variant="body2">
                {event ? `投稿到「${event.title}」；批量选择图片或视频后会按队列逐个提交审核。` : '请选择要投稿的活动。'}
              </Typography>
            )}
          </Box>
          <IconButton onClick={onClose}>
            <Close />
          </IconButton>
        </Stack>
      </DialogTitle>
      <DialogContent sx={{ px: { xs: 2, sm: 3 }, pt: 1 }}>
        {canUpload ? (
          <SubmissionForm
            event={event}
            initialPhotographerName={photographerName}
            initialSubmissionLink={submissionLink}
            initialSubmissionToken={submissionToken}
            lockPhotographerName={Boolean(photographerName)}
            onRequestClose={onClose}
            onUploaded={onUploaded}
            showCloseButton
          />
        ) : (
          <Stack component="form" id="submission-code-form" onSubmit={handleTokenSubmit} sx={{ gap: 2 }}>
            <Alert severity="info">
              公开投稿需要管理员生成的投稿码或限时投稿链接。可以联系管理员获取后在这里输入。
            </Alert>
            {tokenError && <Alert severity="warning">{tokenError}</Alert>}
            <TextField
              autoFocus
              fullWidth
              label="投稿码"
              name="submissionCode"
              placeholder="粘贴投稿码或完整投稿链接"
            />
          </Stack>
        )}
      </DialogContent>
      {!canUpload && (
        <DialogActions sx={{ px: { xs: 2, sm: 3 } }}>
          <Button onClick={onClose}>关闭</Button>
          <Button disabled={verifyingToken || !event} form="submission-code-form" type="submit" variant="contained">
            {verifyingToken ? '验证中...' : '验证'}
          </Button>
        </DialogActions>
      )}
    </Dialog>
  );
}

function normalizeSubmissionCode(value: string) {
  const trimmed = value.trim();
  if (!trimmed) return '';
  try {
    const url = new URL(trimmed, window.location.origin);
    return url.searchParams.get('token') || url.searchParams.get('submissionToken') || trimmed;
  } catch {
    return trimmed;
  }
}
