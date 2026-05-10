import { ContentCopy, Delete, LinkOff } from '@mui/icons-material';
import { Alert, Box, Button, Dialog, DialogActions, DialogContent, DialogTitle, Grid, Paper, Stack, TextField, Typography } from '@mui/material';
import { type FormEvent, useEffect, useState } from 'react';
import { QRCodeSVG } from 'qrcode.react';
import { createSubmissionLink, deleteRevokedSubmissionLink, getSubmissionLinks, revokeSubmissionLink, type EventCard, type SubmissionLink } from '../api/client';

type Props = {
  event: EventCard | null;
  onClose: () => void;
  open: boolean;
};

export function SubmissionLinkDialog({ event, onClose, open }: Props) {
  const [links, setLinks] = useState<SubmissionLink[]>([]);
  const [createdUrl, setCreatedUrl] = useState('');
  const [createdCode, setCreatedCode] = useState('');
  const [error, setError] = useState('');
  const [copied, setCopied] = useState(false);

  function refresh() {
    if (!event) return;
    getSubmissionLinks(event.id).then(setLinks).catch((err) => setError(err instanceof Error ? err.message : '加载链接失败'));
  }

  useEffect(() => {
    if (open) {
      setCreatedUrl('');
      setCreatedCode('');
      setError('');
      setCopied(false);
      refresh();
    }
  }, [event?.id, open]);

  async function handleSubmit(formEvent: FormEvent<HTMLFormElement>) {
    formEvent.preventDefault();
    if (!event) return;
    setError('');
    const formElement = formEvent.currentTarget;
    const form = new FormData(formElement);
    try {
      const link = await createSubmissionLink(event.id, {
        label: String(form.get('label') || ''),
        photographerName: String(form.get('photographerName') || ''),
        expiresInHours: Number(form.get('expiresInHours') || 0),
        maxUses: Number(form.get('maxUses') || 0)
      });
      const url = new URL('/submit', window.location.origin);
      url.searchParams.set('eventId', String(event.id));
      if (link.token) url.searchParams.set('token', link.token);
      setCreatedUrl(url.toString());
      setCreatedCode(link.token || '');
      setCopied(false);
      refresh();
      formElement.reset();
    } catch (err) {
      setError(err instanceof Error ? err.message : '生成链接失败');
    }
  }

  async function copyCreatedText(value: string) {
    if (!value) return;
    await navigator.clipboard?.writeText(value);
    setCopied(true);
  }

  return (
    <Dialog fullWidth maxWidth="md" onClose={onClose} open={open}>
      <DialogTitle>限时投稿链接</DialogTitle>
      <DialogContent dividers>
        <Stack sx={{ gap: 2 }}>
          {error && <Alert severity="error">{error}</Alert>}
          {createdUrl && (
            <Paper sx={{ p: 2 }} variant="outlined">
              <Stack direction={{ xs: 'column', sm: 'row' }} sx={{ alignItems: { xs: 'stretch', sm: 'center' }, gap: 2 }}>
                <Box sx={{ bgcolor: '#fff', border: '1px solid', borderColor: 'divider', borderRadius: 2, display: 'inline-flex', p: 1.25, width: 'fit-content' }}>
                  <QRCodeSVG size={132} value={createdUrl} />
                </Box>
                <Stack sx={{ flex: 1, gap: 1, minWidth: 0 }}>
                  <Alert severity="success" sx={{ py: 0.5 }}>
                    限时投稿链接已生成。
                  </Alert>
                  <Typography
                    sx={{
                      bgcolor: 'action.hover',
                      border: '1px solid',
                      borderColor: 'divider',
                      borderRadius: 1.5,
                      overflowWrap: 'anywhere',
                      px: 1.5,
                      py: 1
                    }}
                    variant="body2"
                  >
                    {createdUrl}
                  </Typography>
                  {createdCode && (
                    <Typography
                      sx={{
                        bgcolor: 'action.hover',
                        border: '1px solid',
                        borderColor: 'divider',
                        borderRadius: 1.5,
                        overflowWrap: 'anywhere',
                        px: 1.5,
                        py: 1
                      }}
                      variant="body2"
                    >
                      投稿码：{createdCode}
                    </Typography>
                  )}
                  {copied && <Typography color="success.main" variant="caption">链接已复制</Typography>}
                </Stack>
                <Stack sx={{ gap: 1 }}>
                  <Button onClick={() => copyCreatedText(createdUrl)} startIcon={<ContentCopy />} variant="contained">
                    复制链接
                  </Button>
                  {createdCode && (
                    <Button onClick={() => copyCreatedText(createdCode)} startIcon={<ContentCopy />} variant="outlined">
                      复制投稿码
                    </Button>
                  )}
                </Stack>
              </Stack>
            </Paper>
          )}
          <Paper component="form" onSubmit={handleSubmit} sx={{ p: 2 }} variant="outlined">
            <Grid container spacing={1.5}>
              <Grid size={{ xs: 12, sm: 6 }}>
                <TextField fullWidth label="链接名称" name="label" placeholder="例如：摄影师小组 A" size="small" />
              </Grid>
              <Grid size={{ xs: 12, sm: 6 }}>
                <TextField fullWidth label="归属摄影师" name="photographerName" size="small" />
              </Grid>
              <Grid size={{ xs: 12, sm: 6 }}>
                <TextField defaultValue={72} fullWidth label="有效期（小时，0 表示长期）" name="expiresInHours" size="small" type="number" />
              </Grid>
              <Grid size={{ xs: 12, sm: 6 }}>
                <TextField defaultValue={0} fullWidth label="最大使用次数（0 表示不限）" name="maxUses" size="small" type="number" />
              </Grid>
              <Grid size={{ xs: 12 }}>
                <Button type="submit" variant="contained">生成链接</Button>
              </Grid>
            </Grid>
          </Paper>
          <Stack sx={{ gap: 1 }}>
            {links.map((link) => (
              <Paper key={link.id} sx={{ p: 1.5 }} variant="outlined">
                <Stack direction={{ xs: 'column', sm: 'row' }} sx={{ alignItems: { xs: 'stretch', sm: 'center' }, gap: 1, justifyContent: 'space-between' }}>
                  <Box>
                    <Typography sx={{ fontWeight: 800 }}>{link.label}</Typography>
                    <Typography color="text.secondary" variant="body2">
                      {link.photographerName || '未绑定摄影师'} / 使用 {link.useCount}{link.maxUses ? `/${link.maxUses}` : ''} / {link.expiresAt ? `到期 ${formatDate(link.expiresAt)}` : '长期有效'}
                      {link.revokedAt ? ' / 已撤销' : ''}
                    </Typography>
                  </Box>
                  {link.revokedAt ? (
                    <Button color="error" onClick={() => event && deleteRevokedSubmissionLink(event.id, link.id).then(refresh).catch((err) => setError(err instanceof Error ? err.message : '删除失败'))} size="small" startIcon={<Delete />} variant="outlined">
                      删除
                    </Button>
                  ) : (
                    <Button color="error" onClick={() => event && revokeSubmissionLink(event.id, link.id).then(refresh).catch((err) => setError(err instanceof Error ? err.message : '撤销失败'))} size="small" startIcon={<LinkOff />} variant="outlined">
                      撤销
                    </Button>
                  )}
                </Stack>
              </Paper>
            ))}
            {!links.length && <Typography color="text.secondary" sx={{ py: 2, textAlign: 'center' }}>还没有生成过限时投稿链接。</Typography>}
          </Stack>
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>关闭</Button>
      </DialogActions>
    </Dialog>
  );
}

function formatDate(value: string) {
  return new Intl.DateTimeFormat('zh-CN', { dateStyle: 'short', timeStyle: 'short' }).format(new Date(value));
}
