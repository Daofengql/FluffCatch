import { Alert, Box, Button, LinearProgress, Paper, Stack, TextField, Typography } from '@mui/material';
import { type ChangeEvent, type FormEvent, useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { submitPhotoWithProgress } from '../../api/client';

type QueueItem = {
  id: string;
  file: File;
  previewUrl: string;
  progress: number;
  status: 'waiting' | 'uploading' | 'done' | 'error';
  message?: string;
};

export function SubmissionPage() {
  const eventId = Number(useParams().eventId);
  const [message, setMessage] = useState('');
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [queue, setQueue] = useState<QueueItem[]>([]);

  useEffect(() => () => {
    queue.forEach((item) => URL.revokeObjectURL(item.previewUrl));
  }, []);

  function handleFiles(changeEvent: ChangeEvent<HTMLInputElement>) {
    const files = Array.from(changeEvent.target.files ?? []);
    const nextItems = files.map((file) => ({
      file,
      id: `${file.name}-${file.size}-${file.lastModified}-${crypto.randomUUID()}`,
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

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submitting) return;
    if (!queue.length) {
      setError('请先选择图片');
      return;
    }
    setError('');
    setMessage('');
    const formData = new FormData(event.currentTarget);
    const submissionPassword = String(formData.get('submissionPassword') || '');
    const photographerName = String(formData.get('photographerName') || '');
    setSubmitting(true);
    let successCount = 0;

    try {
      for (const item of queue) {
        setQueue((prev) => prev.map((entry) => (entry.id === item.id ? { ...entry, message: '上传中...', progress: 1, status: 'uploading' } : entry)));
        const uploadForm = new FormData();
        uploadForm.append('submissionPassword', submissionPassword);
        uploadForm.append('photographerName', photographerName);
        uploadForm.append('file', item.file);

        try {
          await submitPhotoWithProgress(eventId, uploadForm, (progress) => {
            setQueue((prev) => prev.map((entry) => (entry.id === item.id ? { ...entry, progress } : entry)));
          });
          successCount++;
          setQueue((prev) => prev.map((entry) => (entry.id === item.id ? { ...entry, message: '已提交审核', progress: 100, status: 'done' } : entry)));
        } catch (err) {
          const reason = err instanceof Error ? err.message : '上传失败';
          setQueue((prev) => prev.map((entry) => (entry.id === item.id ? { ...entry, message: reason, progress: 100, status: 'error' } : entry)));
        }
      }

      setMessage(`队列完成，成功提交 ${successCount} 张图片。`);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Box sx={{ alignItems: 'center', display: 'flex', justifyContent: 'center', width: '100%' }}>
      <Paper component="form" elevation={3} onSubmit={handleSubmit} sx={{ maxWidth: 680, p: { xs: 3, sm: 4 }, width: '100%' }}>
        <Stack sx={{ gap: 2 }}>
          <Box>
            <Typography sx={{ fontWeight: 800 }} variant="h4">
              上传返图
            </Typography>
            <Typography color="text.secondary" sx={{ mt: 1 }}>
              输入投稿口令后批量选择图片，系统会按队列逐张提交审核。
            </Typography>
          </Box>
          {message && <Alert severity="success">{message}</Alert>}
          {error && <Alert severity="error">{error}</Alert>}
          <TextField label="投稿口令" name="submissionPassword" type="password" />
          <TextField label="摄影师署名（可选）" name="photographerName" />
          <Button component="label" variant="outlined">
            选择图片，可多选
            <input accept="image/*" hidden multiple onChange={handleFiles} type="file" />
          </Button>
          <Stack sx={{ gap: 1.5 }}>
            {queue.map((item) => (
              <Paper key={item.id} variant="outlined" sx={{ p: 1.5 }}>
                <Stack direction={{ xs: 'column', sm: 'row' }} sx={{ gap: 1.5 }}>
                  <Box
                    component="img"
                    src={item.previewUrl}
                    sx={{ aspectRatio: '4 / 3', borderRadius: 1, objectFit: 'cover', width: { xs: '100%', sm: 150 } }}
                  />
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
                    <LinearProgress
                      color={item.status === 'error' ? 'error' : item.status === 'done' ? 'success' : 'primary'}
                      value={item.progress}
                      variant="determinate"
                    />
                    <Box>
                      <Button disabled={submitting || item.status === 'uploading'} onClick={() => removeQueueItem(item.id)} size="small">
                        移除
                      </Button>
                    </Box>
                  </Stack>
                </Stack>
              </Paper>
            ))}
          </Stack>
          <Button disabled={submitting} type="submit" variant="contained">
            {submitting ? '提交中...' : '提交审核'}
          </Button>
        </Stack>
      </Paper>
    </Box>
  );
}

function formatBytes(size: number) {
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  return `${(size / 1024 / 1024).toFixed(1)} MB`;
}
