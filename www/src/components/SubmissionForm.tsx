import { Cancel, ClearAll } from '@mui/icons-material';
import { Alert, Box, Button, FormControl, FormControlLabel, InputLabel, LinearProgress, MenuItem, Paper, Select, Stack, Switch, TextField, Typography } from '@mui/material';
import type { SelectChangeEvent } from '@mui/material/Select';
import { type ChangeEvent, type FormEvent, useEffect, useMemo, useRef, useState } from 'react';
import { getUploadSettings, submitPhotoWithProgress, type EventCard } from '../api/client';
import { getCachedMe, refreshMe, subscribeAuthState } from '../api/authState';

type QueueItem = {
  id: string;
  file: File;
  previewUrl?: string;
  mediaType: 'image' | 'video';
  progress: number;
  status: 'waiting' | 'uploading' | 'done' | 'error' | 'canceled';
  message?: string;
};

type SubmissionFormProps = {
  event: EventCard | null;
  footer?: React.ReactNode;
  initialPhotographerName?: string;
  initialSubmissionToken?: string;
  lockPhotographerName?: boolean;
  onRequestClose?: () => void;
  showCloseButton?: boolean;
};

export function SubmissionForm({ event, footer, initialPhotographerName = '', initialSubmissionToken = '', lockPhotographerName = false, onRequestClose, showCloseButton = false }: SubmissionFormProps) {
  const [message, setMessage] = useState('');
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [photographerName, setPhotographerName] = useState(initialPhotographerName);
  const [authenticated, setAuthenticated] = useState(() => getCachedMe().authenticated);
  const [adminVisibility, setAdminVisibility] = useState<'public' | 'private'>('public');
  const [queue, setQueue] = useState<QueueItem[]>([]);
  const [previewEnabled, setPreviewEnabled] = useState(false);
  const [maxConcurrentUploads, setMaxConcurrentUploads] = useState(2);
  const [maxFilesPerUpload, setMaxFilesPerUpload] = useState(20);
  const activeUploadsRef = useRef<Map<string, AbortController>>(new Map());
  const cancelRequestedRef = useRef(false);
  const queueRef = useRef<QueueItem[]>([]);
  const queueSummary = useMemo(() => {
    const total = queue.length;
    const uploaded = queue.filter((item) => item.status === 'done').length;
    const uploading = queue.filter((item) => item.status === 'uploading').length;
    const failed = queue.filter((item) => item.status === 'error').length;
    const waiting = queue.filter((item) => item.status === 'waiting').length;
    const canceled = queue.filter((item) => item.status === 'canceled').length;
    const overallProgress = total > 0 ? Math.round(queue.reduce((sum, item) => sum + item.progress, 0) / total) : 0;
    return { canceled, failed, overallProgress, total, uploaded, uploading, waiting };
  }, [queue]);

  useEffect(() => {
    queueRef.current = queue;
  }, [queue]);

  useEffect(() => () => {
    activeUploadsRef.current.forEach((controller) => controller.abort());
    activeUploadsRef.current.clear();
    queueRef.current.forEach((item) => {
      if (item.previewUrl) URL.revokeObjectURL(item.previewUrl);
    });
  }, []);

  useEffect(() => {
    setQueue((prev) =>
      prev.map((item) => {
        if (previewEnabled && !item.previewUrl) {
          return { ...item, previewUrl: URL.createObjectURL(item.file) };
        }
        if (!previewEnabled && item.previewUrl) {
          URL.revokeObjectURL(item.previewUrl);
          return { ...item, previewUrl: undefined };
        }
        return item;
      })
    );
  }, [previewEnabled]);

  useEffect(() => {
    setMessage('');
    setError('');
    setPhotographerName(initialPhotographerName);
  }, [event?.id, initialPhotographerName]);

  useEffect(() => {
    const unsubscribe = subscribeAuthState((payload) => setAuthenticated(payload.authenticated));
    refreshMe().catch(() => undefined);
    return unsubscribe;
  }, []);

  useEffect(() => {
    getUploadSettings()
      .then((settings) => {
        setMaxConcurrentUploads(clampUploadConcurrency(settings.maxConcurrentUploads));
        setMaxFilesPerUpload(clampMaxFiles(settings.maxFilesPerUpload));
      })
      .catch(() => {
        setMaxConcurrentUploads(2);
        setMaxFilesPerUpload(20);
      });
  }, []);

  function appendFiles(files: File[]) {
    if (!files.length) return;
    setError('');
    const availableSlots = Math.max(0, maxFilesPerUpload - queueRef.current.length);
    if (availableSlots <= 0) {
      setError(`每次最多选择 ${maxFilesPerUpload} 个媒体文件。`);
      return;
    }
    const acceptedFiles = files.slice(0, availableSlots);
    if (acceptedFiles.length < files.length) {
      setError(`每次最多选择 ${maxFilesPerUpload} 个媒体文件，已加入前 ${acceptedFiles.length} 个。`);
    }
    const nextItems = acceptedFiles.map((file) => ({
      file,
      id: `${file.name}-${file.size}-${file.lastModified}-${crypto.randomUUID()}`,
      mediaType: file.type.startsWith('video/') ? 'video' as const : 'image' as const,
      message: '等待上传',
      previewUrl: previewEnabled ? URL.createObjectURL(file) : undefined,
      progress: 0,
      status: 'waiting' as const
    }));
    setQueue((prev) => [...prev, ...nextItems]);
  }

  function handleFiles(changeEvent: ChangeEvent<HTMLInputElement>) {
    appendFiles(Array.from(changeEvent.target.files ?? []));
    changeEvent.target.value = '';
  }

  function removeQueueItem(id: string) {
    setQueue((prev) => {
      const item = prev.find((entry) => entry.id === id);
      if (item?.previewUrl) URL.revokeObjectURL(item.previewUrl);
      return prev.filter((entry) => entry.id !== id);
    });
  }

  function clearCompletedItems() {
    setQueue((prev) => {
      prev.forEach((item) => {
        if (item.status === 'done' && item.previewUrl) URL.revokeObjectURL(item.previewUrl);
      });
      return prev.filter((item) => item.status !== 'done');
    });
  }

  function cancelAllUploads() {
    cancelRequestedRef.current = true;
    activeUploadsRef.current.forEach((controller) => controller.abort());
    setQueue((prev) =>
      prev.map((item) => {
        if (item.status === 'waiting' || item.status === 'uploading' || item.status === 'error') {
          return { ...item, message: '已取消，可重新提交', status: 'canceled' };
        }
        return item;
      })
    );
    setError('');
    setMessage('已取消未完成的上传。');
  }

  async function handleSubmit(formEvent: FormEvent<HTMLFormElement>) {
    formEvent.preventDefault();
    if (submitting || !event) return;
    if (!queue.length) {
      setError('请先选择图片或视频');
      return;
    }
    cancelRequestedRef.current = false;
    setError('');
    setMessage('');
    const formData = new FormData(formEvent.currentTarget);
    const eventId = event.id;
    const currentPhotographerName = lockPhotographerName ? photographerName : String(formData.get('photographerName') || '');
    const uploadItems = queue.filter((item) => item.status === 'waiting' || item.status === 'error' || item.status === 'canceled');
    if (!uploadItems.length) {
      setError('队列里没有待上传的文件。');
      return;
    }
    const concurrency = Math.min(clampUploadConcurrency(maxConcurrentUploads), uploadItems.length || 1);
    setSubmitting(true);
    let successCount = 0;
    let failureCount = 0;
    let canceledCount = 0;
    let firstError = '';
    let nextIndex = 0;
    let shouldStop = false;

    try {
      async function uploadItem(item: QueueItem) {
        const latest = queueRef.current.find((entry) => entry.id === item.id);
        if (!latest || latest.status === 'done' || latest.status === 'uploading' || cancelRequestedRef.current) return;
        setQueue((prev) => prev.map((entry) => (entry.id === item.id ? { ...entry, message: '上传中...', progress: 1, status: 'uploading' } : entry)));
        const controller = new AbortController();
        activeUploadsRef.current.set(item.id, controller);
        const uploadForm = new FormData();
        uploadForm.append('submissionToken', initialSubmissionToken);
        uploadForm.append('photographerName', currentPhotographerName);
        if (authenticated) {
          uploadForm.append('visibility', adminVisibility);
        }
        uploadForm.append('file', item.file);

        try {
          await submitPhotoWithProgress(eventId, uploadForm, (progress) => {
            setQueue((prev) => prev.map((entry) => (entry.id === item.id ? { ...entry, progress } : entry)));
          }, controller.signal);
          successCount++;
          setQueue((prev) => prev.map((entry) => (entry.id === item.id ? { ...entry, message: authenticated ? '已加入画廊' : '已提交审核', progress: 100, status: 'done' } : entry)));
        } catch (err) {
          if (isAbortError(err) || cancelRequestedRef.current) {
            canceledCount++;
            setQueue((prev) => prev.map((entry) => (entry.id === item.id ? { ...entry, message: '已取消，可重新提交', status: 'canceled' } : entry)));
            return;
          }
          const reason = err instanceof Error ? err.message : '上传失败';
          failureCount++;
          firstError ||= reason;
          setQueue((prev) =>
            prev.map((entry) =>
              entry.id === item.id
                ? { ...entry, message: reason, progress: 100, status: 'error' }
                : entry
            )
          );
          if (reason.toLowerCase().includes('submission link')) {
            shouldStop = true;
          }
        } finally {
          activeUploadsRef.current.delete(item.id);
        }
      }

      async function worker() {
        while (!shouldStop && !cancelRequestedRef.current) {
          const item = uploadItems[nextIndex];
          nextIndex++;
          if (!item) return;
          await uploadItem(item);
        }
      }

      await Promise.all(Array.from({ length: concurrency }, () => worker()));

      if (cancelRequestedRef.current || canceledCount > 0) {
        setMessage(successCount > 0 ? `已取消剩余上传，已完成 ${successCount} 个媒体文件。` : '已取消未完成的上传。');
      } else if (successCount > 0 && failureCount === 0) {
        setMessage(authenticated ? `队列完成，已加入画廊 ${successCount} 个媒体文件。` : `队列完成，成功提交 ${successCount} 个媒体文件。`);
      } else if (successCount > 0) {
        setError(`队列完成，成功提交 ${successCount} 个媒体文件，${failureCount} 个失败。`);
      } else {
        setError(formatUploadError(firstError));
      }
    } finally {
      cancelRequestedRef.current = false;
      setSubmitting(false);
    }
  }

  return (
    <Stack component="form" onSubmit={handleSubmit} sx={{ gap: 2 }}>
      {footer}
      {message && <Alert severity="success">{message}</Alert>}
      {error && <Alert severity="error">{error}</Alert>}
      {authenticated && <Alert severity="info">当前为管理员登录状态：上传会直接进入正式画廊，不需要限时投稿链接，也不会进入审核池。</Alert>}
      <Stack direction={{ xs: 'column', sm: 'row' }} sx={{ gap: 2 }}>
        <TextField
          fullWidth
          label={lockPhotographerName ? '摄影师署名' : '摄影师署名（可选）'}
          name="photographerName"
          onChange={(event) => setPhotographerName(event.target.value)}
          slotProps={{ input: { readOnly: lockPhotographerName } }}
          value={photographerName}
        />
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
      <Stack
        direction={{ xs: 'column', sm: 'row' }}
        onDragOver={(event) => {
          event.preventDefault();
          event.dataTransfer.dropEffect = 'copy';
        }}
        onDrop={(event) => {
          event.preventDefault();
          appendFiles(Array.from(event.dataTransfer.files ?? []));
        }}
        sx={{ alignItems: { xs: 'stretch', sm: 'center' }, border: '1px dashed', borderColor: 'divider', borderRadius: 2, gap: 1.5, justifyContent: 'space-between', p: 1.5 }}
      >
        <Button component="label" variant="outlined">
          选择图片或视频，可多选
          <input accept="image/*,video/mp4,video/webm,video/ogg,video/quicktime" hidden multiple onChange={handleFiles} type="file" />
        </Button>
        <FormControlLabel
          control={<Switch checked={previewEnabled} onChange={(event) => setPreviewEnabled(event.target.checked)} />}
          label="显示上传预览"
          sx={{ m: 0 }}
        />
      </Stack>
      {!!queue.length && (
        <Paper sx={{ p: 1.5 }} variant="outlined">
          <Stack sx={{ gap: 1 }}>
            <Stack direction={{ xs: 'column', sm: 'row' }} sx={{ gap: 1, justifyContent: 'space-between' }}>
              <Typography sx={{ fontWeight: 800 }} variant="body2">
                上传进度 {queueSummary.uploaded}/{queueSummary.total}
              </Typography>
              <Typography color="text.secondary" variant="body2">
                已上传 {queueSummary.uploaded} 个，未上传 {queueSummary.waiting} 个，上传中 {queueSummary.uploading} 个，失败 {queueSummary.failed} 个，已取消 {queueSummary.canceled} 个，并发 {clampUploadConcurrency(maxConcurrentUploads)} 个，上限 {maxFilesPerUpload} 个
              </Typography>
            </Stack>
            <LinearProgress
              color={queueSummary.failed > 0 ? 'warning' : queueSummary.uploaded === queueSummary.total && queueSummary.total > 0 ? 'success' : 'primary'}
              value={queueSummary.overallProgress}
              variant="determinate"
            />
            <Stack direction={{ xs: 'column', sm: 'row' }} sx={{ gap: 1, justifyContent: 'flex-end' }}>
              <Button
                disabled={!queueSummary.uploaded}
                onClick={clearCompletedItems}
                size="small"
                startIcon={<ClearAll />}
                variant="outlined"
              >
                清除已完成
              </Button>
              <Button
                color="warning"
                disabled={!queue.some((item) => item.status === 'waiting' || item.status === 'uploading' || item.status === 'error')}
                onClick={cancelAllUploads}
                size="small"
                startIcon={<Cancel />}
                variant="outlined"
              >
                取消全部
              </Button>
            </Stack>
          </Stack>
        </Paper>
      )}
      <Stack sx={{ gap: 1.5, maxHeight: 420, overflowY: 'auto', pr: 0.5 }}>
        {queue.map((item) => (
          <Paper key={item.id} variant="outlined" sx={{ p: previewEnabled ? 1.5 : 1 }}>
            <Stack direction={{ xs: previewEnabled ? 'column' : 'row', sm: 'row' }} sx={{ alignItems: previewEnabled ? 'stretch' : 'center', gap: previewEnabled ? 1.5 : 1 }}>
              {previewEnabled && item.previewUrl && item.mediaType === 'video' ? (
                <Box
                  component="video"
                  muted
                  preload="metadata"
                  src={item.previewUrl}
                  sx={{ aspectRatio: '4 / 3', bgcolor: 'common.black', borderRadius: 1, objectFit: 'cover', width: { xs: '100%', sm: 150 } }}
                />
              ) : previewEnabled && item.previewUrl ? (
                <Box component="img" src={item.previewUrl} sx={{ aspectRatio: '4 / 3', borderRadius: 1, objectFit: 'cover', width: { xs: '100%', sm: 150 } }} />
              ) : null}
              <Stack sx={{ flex: 1, gap: 0.5, minWidth: 0 }}>
                <Typography noWrap sx={{ fontWeight: 800 }}>
                  {item.file.name}
                </Typography>
                <Typography color="text.secondary" variant="body2">
                  类型：{item.file.type || '未知'} / 大小：{formatBytes(item.file.size)}
                </Typography>
                <Typography color={item.status === 'error' ? 'error' : item.status === 'canceled' ? 'warning.main' : 'text.secondary'} variant="body2">
                  {item.message || '等待上传'}
                </Typography>
                <LinearProgress color={item.status === 'error' ? 'error' : item.status === 'done' ? 'success' : item.status === 'canceled' ? 'warning' : 'primary'} value={item.progress} variant="determinate" />
                <Box sx={{ display: previewEnabled ? 'block' : 'none' }}>
                  <Button disabled={submitting || item.status === 'uploading'} onClick={() => removeQueueItem(item.id)} size="small">
                    移除
                  </Button>
                </Box>
              </Stack>
              {!previewEnabled && (
                <Button disabled={submitting || item.status === 'uploading'} onClick={() => removeQueueItem(item.id)} size="small">
                  移除
                </Button>
              )}
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
  if (!reason) return '上传失败，请检查限时投稿链接或稍后重试。';
  if (reason.toLowerCase().includes('submission link')) return '限时投稿链接无效、已过期或已达到使用次数。';
  return reason;
}

function isAbortError(err: unknown) {
  return err instanceof DOMException && err.name === 'AbortError';
}

function clampUploadConcurrency(value?: number) {
  const parsed = Number(value) || 2;
  return Math.max(1, Math.min(8, Math.floor(parsed)));
}

function clampMaxFiles(value?: number) {
  const parsed = Number(value) || 20;
  return Math.max(1, Math.min(200, Math.floor(parsed)));
}
