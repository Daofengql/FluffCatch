import { Alert, Box, Button, FormControl, InputLabel, LinearProgress, MenuItem, Paper, Select, Stack, TextField, Typography } from '@mui/material';
import type { SelectChangeEvent } from '@mui/material/Select';
import { type ChangeEvent, type FormEvent, useEffect, useRef, useState } from 'react';
import { submitPhotoWithProgress, type EventCard } from '../api/client';
import { getCachedMe, refreshMe, subscribeAuthState } from '../api/authState';

type QueueItem = {
  id: string;
  file: File;
  previewUrl: string;
  mediaType: 'image' | 'video';
  progress: number;
  status: 'waiting' | 'uploading' | 'done' | 'error';
  message?: string;
};

type SubmissionFormProps = {
  event: EventCard | null;
  footer?: React.ReactNode;
  initialSubmissionPassword?: string;
  onRequestClose?: () => void;
  onSubmitted?: () => void;
  showCloseButton?: boolean;
};

export function SubmissionForm({ event, footer, initialSubmissionPassword = '', onRequestClose, onSubmitted, showCloseButton = false }: SubmissionFormProps) {
  const [message, setMessage] = useState('');
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [submissionPassword, setSubmissionPassword] = useState(initialSubmissionPassword);
  const [authenticated, setAuthenticated] = useState(() => getCachedMe().authenticated);
  const [adminVisibility, setAdminVisibility] = useState<'public' | 'private'>('public');
  const [queue, setQueue] = useState<QueueItem[]>([]);
  const queueRef = useRef<QueueItem[]>([]);
  const hasInitialPassword = initialSubmissionPassword.trim() !== '';

  useEffect(() => {
    queueRef.current = queue;
  }, [queue]);

  useEffect(() => () => {
    queueRef.current.forEach((item) => URL.revokeObjectURL(item.previewUrl));
  }, []);

  useEffect(() => {
    setMessage('');
    setError('');
    setSubmissionPassword(initialSubmissionPassword);
  }, [event?.id, initialSubmissionPassword]);

  useEffect(() => {
    const unsubscribe = subscribeAuthState((payload) => setAuthenticated(payload.authenticated));
    refreshMe().catch(() => undefined);
    return unsubscribe;
  }, []);

  function handleFiles(changeEvent: ChangeEvent<HTMLInputElement>) {
    const files = Array.from(changeEvent.target.files ?? []);
    const nextItems = files.map((file) => ({
      file,
      id: `${file.name}-${file.size}-${file.lastModified}-${crypto.randomUUID()}`,
      mediaType: file.type.startsWith('video/') ? 'video' as const : 'image' as const,
      message: '等待上传',
      previewUrl: URL.createObjectURL(file),
      progress: 0,
      status: 'waiting' as const
    }));
    setQueue((prev) => [...prev, ...nextItems]);
    changeEvent.target.value = '';
  }

  function removeQueueItem(id: string) {
    setQueue((prev) => {
      const item = prev.find((entry) => entry.id === id);
      if (item) URL.revokeObjectURL(item.previewUrl);
      return prev.filter((entry) => entry.id !== id);
    });
  }

  async function handleSubmit(formEvent: FormEvent<HTMLFormElement>) {
    formEvent.preventDefault();
    if (submitting || !event) return;
    if (!queue.length) {
      setError('请先选择图片或视频');
      return;
    }
    setError('');
    setMessage('');
    const formData = new FormData(formEvent.currentTarget);
    const currentSubmissionPassword = authenticated ? '' : submissionPassword;
    const photographerName = String(formData.get('photographerName') || '');
    setSubmitting(true);
    let successCount = 0;
    let failureCount = 0;
    let firstError = '';

    try {
      for (const item of queue) {
        if (item.status === 'done') continue;
        setQueue((prev) => prev.map((entry) => (entry.id === item.id ? { ...entry, message: '上传中...', progress: 1, status: 'uploading' } : entry)));
        const uploadForm = new FormData();
        uploadForm.append('submissionPassword', currentSubmissionPassword);
        uploadForm.append('photographerName', photographerName);
        if (authenticated) {
          uploadForm.append('visibility', adminVisibility);
        }
        uploadForm.append('file', item.file);

        try {
          await submitPhotoWithProgress(event.id, uploadForm, (progress) => {
            setQueue((prev) => prev.map((entry) => (entry.id === item.id ? { ...entry, progress } : entry)));
          });
          successCount++;
          setQueue((prev) => prev.map((entry) => (entry.id === item.id ? { ...entry, message: authenticated ? '已加入画廊' : '已提交审核', progress: 100, status: 'done' } : entry)));
        } catch (err) {
          const reason = err instanceof Error ? err.message : '上传失败';
          failureCount++;
          firstError ||= reason;
          const isPasswordError = reason.toLowerCase().includes('submission password');
          setQueue((prev) =>
            prev.map((entry) =>
              entry.id === item.id
                ? { ...entry, message: isPasswordError ? '等待重试' : reason, progress: isPasswordError ? 0 : 100, status: isPasswordError ? 'waiting' : 'error' }
                : entry
            )
          );
          if (isPasswordError) break;
        }
      }

      if (successCount > 0 && failureCount === 0) {
        setMessage(authenticated ? `队列完成，已加入画廊 ${successCount} 个媒体文件。` : `队列完成，成功提交 ${successCount} 个媒体文件。`);
        onSubmitted?.();
      } else if (successCount > 0) {
        setError(`队列完成，成功提交 ${successCount} 个媒体文件，${failureCount} 个失败。`);
        onSubmitted?.();
      } else {
        setError(formatUploadError(firstError));
      }
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Stack component="form" onSubmit={handleSubmit} sx={{ gap: 2 }}>
      {footer}
      {message && <Alert severity="success">{message}</Alert>}
      {error && <Alert severity="error">{error}</Alert>}
      {authenticated && <Alert severity="info">当前为管理员登录状态：上传会直接进入正式画廊，不需要投稿口令，也不会进入审核池。</Alert>}
      {!authenticated && hasInitialPassword && <Alert severity="success">已从分享链接自动填入投稿口令。</Alert>}
      {authenticated && hasInitialPassword && <Alert severity="info">分享链接里带了投稿口令；但当前是管理员登录状态，系统会跳过口令校验。</Alert>}
      <Stack direction={{ xs: 'column', sm: 'row' }} sx={{ gap: 2 }}>
        {!authenticated && (
          <TextField
            fullWidth
            label="投稿口令"
            name="submissionPassword"
            onChange={(event) => setSubmissionPassword(event.target.value)}
            type="password"
            value={submissionPassword}
          />
        )}
        <TextField fullWidth label="摄影师署名（可选）" name="photographerName" />
      </Stack>
      {authenticated && (
        <FormControl fullWidth>
          <InputLabel>返图可见性</InputLabel>
          <Select
            label="返图可见性"
            onChange={(e: SelectChangeEvent) => setAdminVisibility(e.target.value as 'public' | 'private')}
            value={adminVisibility}
          >
            <MenuItem value="public">公开</MenuItem>
            <MenuItem value="private">私密</MenuItem>
          </Select>
        </FormControl>
      )}
      <Button component="label" variant="outlined">
        选择图片或视频，可多选
        <input accept="image/*,video/mp4,video/webm,video/ogg,video/quicktime" hidden multiple onChange={handleFiles} type="file" />
      </Button>
      <Stack sx={{ gap: 1.5, maxHeight: 420, overflowY: 'auto', pr: 0.5 }}>
        {queue.map((item) => (
          <Paper key={item.id} variant="outlined" sx={{ p: 1.5 }}>
            <Stack direction={{ xs: 'column', sm: 'row' }} sx={{ gap: 1.5 }}>
              {item.mediaType === 'video' ? (
                <Box
                  component="video"
                  muted
                  preload="metadata"
                  src={item.previewUrl}
                  sx={{ aspectRatio: '4 / 3', bgcolor: 'common.black', borderRadius: 1, objectFit: 'cover', width: { xs: '100%', sm: 150 } }}
                />
              ) : (
                <Box component="img" src={item.previewUrl} sx={{ aspectRatio: '4 / 3', borderRadius: 1, objectFit: 'cover', width: { xs: '100%', sm: 150 } }} />
              )}
              <Stack sx={{ flex: 1, gap: 0.5, minWidth: 0 }}>
                <Typography noWrap sx={{ fontWeight: 800 }}>
                  {item.file.name}
                </Typography>
                <Typography color="text.secondary" variant="body2">
                  类型：{item.file.type || '未知'} / 大小：{formatBytes(item.file.size)}
                </Typography>
                <Typography color={item.status === 'error' ? 'error' : 'text.secondary'} variant="body2">
                  {item.message || '等待上传'}
                </Typography>
                <LinearProgress color={item.status === 'error' ? 'error' : item.status === 'done' ? 'success' : 'primary'} value={item.progress} variant="determinate" />
                <Box>
                  <Button disabled={submitting || item.status === 'uploading'} onClick={() => removeQueueItem(item.id)} size="small">
                    移除
                  </Button>
                </Box>
              </Stack>
            </Stack>
          </Paper>
        ))}
        {!queue.length && (
          <Paper sx={{ bgcolor: 'action.hover', p: 3, textAlign: 'center' }} variant="outlined">
            <Typography color="text.secondary">还没有选择图片或视频。点上面的按钮一次选多个文件，毛毛们排队进审核池。</Typography>
          </Paper>
        )}
      </Stack>
      <Stack direction="row" sx={{ gap: 1, justifyContent: 'flex-end' }}>
        {showCloseButton && (
          <Button disabled={submitting} onClick={onRequestClose}>
            关闭
          </Button>
        )}
        <Button disabled={submitting || !event} type="submit" variant="contained">
          {submitting ? '提交中...' : '提交审核'}
        </Button>
      </Stack>
    </Stack>
  );
}

function formatBytes(size: number) {
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  return `${(size / 1024 / 1024).toFixed(1)} MB`;
}

function formatUploadError(reason: string) {
  if (!reason) return '上传失败，请检查投稿口令或稍后重试。';
  if (reason.toLowerCase().includes('submission password')) return '投稿口令不正确，请检查后重试。';
  return reason;
}
