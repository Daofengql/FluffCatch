import { CalendarMonth, CameraAlt, CheckCircle, CloudDownload, CloudUpload, ContentCopy, Delete, Edit, Favorite, FavoriteBorder, LocalOffer, LocationOn, PhotoLibrary, QrCode2, Storage } from '@mui/icons-material';
import {
  Alert,
  Box,
  Button,
  Card,
  CardActionArea,
  CardContent,
  CardMedia,
  Checkbox,
  Chip,
  CircularProgress,
  Collapse,
  FormControl,
  Grid,
  InputLabel,
  MenuItem,
  Paper,
  Select,
  Stack,
  TextField,
  Typography
} from '@mui/material';
import type { SelectChangeEvent } from '@mui/material/Select';
import { useEffect, useMemo, useState } from 'react';
import { useParams } from 'react-router-dom';
import { QRCodeSVG } from 'qrcode.react';
import { deletePhoto, getEvent, getPhotos, likePhoto, updatePhoto, type EventCard, type Photo } from '../../api/client';
import { getCachedMe, refreshMe, subscribeAuthState } from '../../api/authState';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { ImagePreviewDialog } from '../../components/ImagePreviewDialog';
import { EventEditorDialog } from '../../components/EventEditorDialog';
import { SubmissionDialog } from '../../components/SubmissionDialog';
import { SubmissionReviewDialog } from '../../components/SubmissionReviewDialog';
import { formatEventLocation } from '../../utils/eventLocation';

export function EventDetailPage() {
  const eventId = Number(useParams().eventId);
  const [event, setEvent] = useState<EventCard | null>(null);
  const [photos, setPhotos] = useState<Photo[]>([]);
  const [previewIndex, setPreviewIndex] = useState(-1);
  const [submitOpen, setSubmitOpen] = useState(false);
  const [error, setError] = useState('');
  const [message, setMessage] = useState('');
  const [loading, setLoading] = useState(true);
  const [authenticated, setAuthenticated] = useState(() => getCachedMe().authenticated);
  const [likeError, setLikeError] = useState('');
  const [shareOpen, setShareOpen] = useState(false);
  const [reviewOpen, setReviewOpen] = useState(false);
  const [editorOpen, setEditorOpen] = useState(false);
  const [manageMode, setManageMode] = useState(false);
  const [selectedIds, setSelectedIds] = useState<number[]>([]);
  const [editingPhoto, setEditingPhoto] = useState<Photo | null>(null);
  const [photoForm, setPhotoForm] = useState({ accessPassword: '', photographerName: '', tags: '', visibility: 'public' as Photo['visibility'] });
  const [copyMessage, setCopyMessage] = useState('');
  const [deleteConfirm, setDeleteConfirm] = useState<{ type: 'batch' } | { type: 'single'; photo: Photo } | null>(null);

  function load() {
    setLoading(true);
    setError('');
    Promise.all([getEvent(eventId), getPhotos(eventId)])
      .then(([eventData, photoData]) => {
        setEvent(eventData);
        setPhotos(photoData.photos);
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '加载失败'))
      .finally(() => setLoading(false));
  }

  useEffect(load, [eventId]);

  useEffect(() => {
    const unsubscribe = subscribeAuthState((payload) => setAuthenticated(payload.authenticated));
    refreshMe().catch(() => undefined);
    return unsubscribe;
  }, []);

  const previewItems = useMemo(
    () =>
      photos.map((photo) => ({
        src: photo.url,
        subtitle: [
          photo.photographerName ? `摄影师：${photo.photographerName}` : '匿名投稿',
          `${formatContentType(photo.contentType)} · ${formatBytes(photo.sizeBytes)}`,
          `${photo.likeCount || 0} 个喜欢`
        ].join(' · '),
        title: event?.title || '图片预览'
      })),
    [event?.title, photos]
  );

  async function handleLike(photo: Photo) {
    setLikeError('');
    setPhotos((prev) => prev.map((item) => (item.id === photo.id ? { ...item, liked: true, likeCount: (item.likeCount || 0) + (item.liked ? 0 : 1) } : item)));
    try {
      const result = await likePhoto(photo.id);
      setPhotos((prev) => prev.map((item) => (item.id === photo.id ? { ...item, liked: result.liked, likeCount: result.likeCount } : item)));
    } catch (err) {
      setLikeError(err instanceof Error ? err.message : '点赞失败');
      setPhotos((prev) => prev.map((item) => (item.id === photo.id ? photo : item)));
    }
  }

  const shareUrl = useMemo(() => {
    if (!event || typeof window === 'undefined') return '';
    const url = new URL('/submit', window.location.origin);
    url.searchParams.set('eventId', String(event.id));
    if (event.submissionPassword) {
      url.searchParams.set('password', event.submissionPassword);
    }
    return url.toString();
  }, [event]);

  async function copyShareUrl() {
    if (!shareUrl) return;
    try {
      await navigator.clipboard.writeText(shareUrl);
      setCopyMessage('分享链接已复制');
    } catch {
      setCopyMessage('复制失败，请手动复制链接');
    }
  }

  function handleEventSaved() {
    setEditorOpen(false);
    setMessage('兽聚已更新。');
    load();
  }

  function togglePhotoSelection(photoId: number) {
    setSelectedIds((prev) => (prev.includes(photoId) ? prev.filter((id) => id !== photoId) : [...prev, photoId]));
  }

  async function handleBatchDelete() {
    if (!selectedIds.length) return;
    setDeleteConfirm({ type: 'batch' });
  }

  async function confirmBatchDelete(headers: Record<string, string>) {
    setDeleteConfirm(null);
    setError('');
    try {
      await Promise.all(selectedIds.map((id) => deletePhoto(id, headers)));
      setMessage(`已删除 ${selectedIds.length} 张图片。`);
      setSelectedIds([]);
      load();
    } catch (err) {
      setError(err instanceof Error ? err.message : '批量删除失败');
    }
  }

  function requestDeletePhoto(photo: Photo) {
    setDeleteConfirm({ type: 'single', photo });
  }

  async function confirmDeletePhoto(headers: Record<string, string>) {
    if (deleteConfirm?.type !== 'single') return;
    const { photo } = deleteConfirm;
    setDeleteConfirm(null);
    setError('');
    try {
      await deletePhoto(photo.id, headers);
      setSelectedIds((prev) => prev.filter((id) => id !== photo.id));
      load();
    } catch (err) {
      setError(err instanceof Error ? err.message : '删除图片失败');
    }
  }

  function startEditPhoto(photo: Photo) {
    setEditingPhoto(photo);
    setPhotoForm({
      accessPassword: '',
      photographerName: photo.photographerName || '',
      tags: (photo.tags ?? []).map((tag) => tag.name).join(' '),
      visibility: photo.visibility
    });
  }

  async function handlePhotoFormSubmit(formEvent: React.FormEvent<HTMLFormElement>) {
    formEvent.preventDefault();
    if (!editingPhoto) return;
    setError('');
    try {
      await updatePhoto(editingPhoto.id, {
        accessPassword: photoForm.accessPassword,
        photographerName: photoForm.photographerName,
        tags: photoForm.tags.split(/[\s,，]+/).filter(Boolean),
        visibility: photoForm.visibility
      });
      setEditingPhoto(null);
      setMessage('图片属性已更新。');
      load();
    } catch (err) {
      setError(err instanceof Error ? err.message : '保存图片属性失败');
    }
  }

  if (loading) {
    return (
      <Stack sx={{ alignItems: 'center', py: 8 }}>
        <CircularProgress />
      </Stack>
    );
  }
  if (error || !event) return <Alert severity="error">{error || '兽聚不存在'}</Alert>;

  return (
    <Stack sx={{ gap: 3 }}>
      {message && <Alert onClose={() => setMessage('')} severity="success">{message}</Alert>}

      <Paper
        sx={{
          background: 'linear-gradient(135deg, rgba(37,99,235,0.10), rgba(255,255,255,0.96) 48%, rgba(14,165,233,0.10))',
          borderRadius: 4,
          overflow: 'hidden',
          p: { xs: 2.5, md: 3 }
        }}
      >
        <Stack direction={{ xs: 'column', md: 'row' }} sx={{ gap: 3 }}>
          <Box
            sx={{
              bgcolor: 'grey.100',
              borderRadius: 3,
              flexShrink: 0,
              overflow: 'hidden',
              width: { xs: '100%', md: 360 }
            }}
          >
            {event.coverUrl ? (
              <Box component="img" src={event.coverUrl} sx={{ aspectRatio: '16 / 10', display: 'block', objectFit: 'cover', width: '100%' }} />
            ) : (
              <Box sx={{ aspectRatio: '16 / 10', background: 'linear-gradient(135deg, #bfdbfe, #fde68a)' }} />
            )}
          </Box>
          <Stack sx={{ flex: 1, gap: 2, justifyContent: 'center', minWidth: 0 }}>
            <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 1 }}>
              <Chip color={event.isPublic ? 'success' : 'default'} label={event.isPublic ? '公开画廊' : '隐藏'} size="small" />
              <Chip color={event.submissionEnabled ? 'primary' : 'default'} label={event.submissionEnabled ? '开放投稿' : '投稿关闭'} size="small" />
              <Chip label={`${event.photoCount || photos.length} 张图片`} size="small" />
            </Stack>
            <Box>
              <Typography sx={{ fontWeight: 900 }} variant="h4">
                {event.title}
              </Typography>
              <Typography color="text.secondary" sx={{ mt: 1, whiteSpace: 'pre-wrap' }}>
                {event.description || '管理员还没有填写简介。'}
              </Typography>
            </Box>
            <Stack direction={{ xs: 'column', sm: 'row' }} sx={{ color: 'text.secondary', gap: 1.5 }}>
              <Stack direction="row" sx={{ alignItems: 'center', gap: 0.75 }}>
                <LocationOn fontSize="small" />
                <Typography>{formatEventLocation(event)}</Typography>
              </Stack>
              <Stack direction="row" sx={{ alignItems: 'center', gap: 0.75 }}>
                <CalendarMonth fontSize="small" />
                <Typography>{formatDateRange(event.startTime, event.endTime)}</Typography>
              </Stack>
            </Stack>
            <Stack direction={{ xs: 'column', sm: 'row' }} sx={{ alignItems: { xs: 'stretch', sm: 'center' }, flexWrap: 'wrap', gap: 1 }}>
              {event.submissionEnabled && (
                <Button onClick={() => setSubmitOpen(true)} startIcon={<CloudUpload />} sx={{ width: 'fit-content' }} variant="contained">
                  上传返图
                </Button>
              )}
              {authenticated && (
                <>
                  <Button onClick={() => setShareOpen((prev) => !prev)} startIcon={<QrCode2 />} sx={{ width: 'fit-content' }} variant="outlined">
                    分享返图入口
                  </Button>
                  <Button onClick={() => setEditorOpen(true)} startIcon={<Edit />} sx={{ width: 'fit-content' }} variant="outlined">
                    编辑兽聚
                  </Button>
                  <Button onClick={() => setReviewOpen(true)} startIcon={<CloudUpload />} sx={{ width: 'fit-content' }} variant="outlined">
                    审核返图
                  </Button>
                  <Button onClick={() => setManageMode((prev) => !prev)} startIcon={<PhotoLibrary />} color={manageMode ? 'secondary' : 'primary'} sx={{ width: 'fit-content' }} variant={manageMode ? 'contained' : 'outlined'}>
                    {manageMode ? '退出管理' : '管理图片'}
                  </Button>
                </>
              )}
            </Stack>
          </Stack>
        </Stack>
      </Paper>

      <Collapse in={authenticated && shareOpen && Boolean(shareUrl)}>
        <Paper sx={{ borderRadius: 3, p: { xs: 2, sm: 2.5 } }} variant="outlined">
          <Stack direction={{ xs: 'column', md: 'row' }} sx={{ alignItems: { xs: 'stretch', md: 'center' }, gap: 2 }}>
            <Box sx={{ bgcolor: 'white', border: '1px solid', borderColor: 'grey.200', borderRadius: 2, display: 'inline-flex', p: 1.5, width: 'fit-content' }}>
              <QRCodeSVG size={132} value={shareUrl} />
            </Box>
            <Stack sx={{ flex: 1, gap: 1, minWidth: 0 }}>
              <Typography sx={{ fontWeight: 800 }}>返图分享链接</Typography>
              <Typography color="text.secondary" variant="body2">
                访客扫描或打开后会进入独立返图页，并自动选择当前兽聚；如果设置了投稿口令，也会自动填入。
              </Typography>
              {!event.submissionPassword && (
                <Alert severity="warning" sx={{ py: 0.5 }}>
                  当前链接没有携带投稿口令。若这是迁移前已有兽聚，请到后台编辑兽聚并重新保存一次投稿口令。
                </Alert>
              )}
              <Typography
                sx={{
                  bgcolor: 'grey.50',
                  border: '1px solid',
                  borderColor: 'grey.200',
                  borderRadius: 1.5,
                  overflowWrap: 'anywhere',
                  px: 1.5,
                  py: 1
                }}
                variant="body2"
              >
                {shareUrl}
              </Typography>
              {copyMessage && (
                <Typography color="success.main" variant="caption">
                  {copyMessage}
                </Typography>
              )}
            </Stack>
            <Button onClick={copyShareUrl} startIcon={<ContentCopy />} variant="contained">
              复制链接
            </Button>
          </Stack>
        </Paper>
      </Collapse>

      {likeError && <Alert severity="warning">{likeError}</Alert>}

      {manageMode && authenticated && (
        <Stack direction="row" sx={{ alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: 1 }}>
          <Stack direction="row" sx={{ gap: 1, alignItems: 'center' }}>
            <Button
              disabled={!photos.length}
              onClick={() => setSelectedIds((prev) => prev.length === photos.length ? [] : photos.map((p) => p.id))}
              size="small"
              variant={selectedIds.length === photos.length && photos.length > 0 ? 'contained' : 'outlined'}
            >
              {selectedIds.length === photos.length && photos.length > 0 ? '取消全选' : '全选'}
            </Button>
            <Typography color="text.secondary" variant="body2">
              已选择 {selectedIds.length} 张
            </Typography>
          </Stack>
          <Button
            color="error"
            disabled={!selectedIds.length}
            onClick={handleBatchDelete}
            size="small"
            startIcon={<Delete />}
            variant="outlined"
          >
            批量删除{selectedIds.length ? ` (${selectedIds.length})` : ''}
          </Button>
        </Stack>
      )}

      <Grid container spacing={2}>
        {photos.map((photo, index) => (
          <Grid key={photo.id} size={{ xs: 12, sm: 6, md: 4, lg: 3 }}>
            <Card sx={{ borderRadius: 3, height: '100%', overflow: 'hidden', position: 'relative', border: manageMode && selectedIds.includes(photo.id) ? '2px solid' : undefined, borderColor: manageMode && selectedIds.includes(photo.id) ? 'primary.main' : undefined }}>
              {manageMode && authenticated && (
                <Checkbox
                  checked={selectedIds.includes(photo.id)}
                  onChange={() => togglePhotoSelection(photo.id)}
                  sx={{
                    bgcolor: 'rgba(255,255,255,0.9)',
                    borderRadius: 1,
                    left: 8,
                    position: 'absolute',
                    top: 8,
                    zIndex: 2,
                    '&:hover': { bgcolor: 'rgba(255,255,255,1)' }
                  }}
                />
              )}
              <CardActionArea onClick={() => manageMode && authenticated ? togglePhotoSelection(photo.id) : setPreviewIndex(index)} sx={{ display: 'block' }}>
                <CardMedia
                  component="img"
                  image={photo.thumbnailUrl || photo.url}
                  sx={{ aspectRatio: '4 / 3', bgcolor: 'grey.100', cursor: manageMode ? 'pointer' : 'zoom-in', objectFit: 'cover' }}
                />
              </CardActionArea>
              <CardContent sx={{ p: 1.75 }}>
                <Stack sx={{ gap: 1.25 }}>
                  <Stack direction="row" sx={{ alignItems: 'center', gap: 1, minWidth: 0 }}>
                    <CameraAlt color="primary" fontSize="small" />
                    <Typography noWrap sx={{ flex: 1, fontWeight: 700 }}>
                      {photo.photographerName || '匿名投稿'}
                    </Typography>
                    {!manageMode && (
                      <Button
                        color={photo.liked ? 'error' : 'inherit'}
                        onClick={() => void handleLike(photo)}
                        size="small"
                        startIcon={photo.liked ? <Favorite /> : <FavoriteBorder />}
                        sx={{ minWidth: 0 }}
                      >
                        {photo.likeCount || 0}
                      </Button>
                    )}
                  </Stack>
                  {!manageMode && (
                    <Button
                      component="a"
                      download={downloadFilename(event.title, photo)}
                      href={photo.url}
                      onClick={(clickEvent) => clickEvent.stopPropagation()}
                      size="small"
                      startIcon={<CloudDownload />}
                      target="_blank"
                      variant="outlined"
                    >
                      下载原图
                    </Button>
                  )}
                  {manageMode && authenticated && (
                    <Stack direction="row" sx={{ gap: 1 }}>
                      <Button onClick={() => startEditPhoto(photo)} size="small" startIcon={<Edit />}>属性</Button>
                      <Button color="error" onClick={() => requestDeletePhoto(photo)} size="small" startIcon={<Delete />}>删除</Button>
                    </Stack>
                  )}
                  <Stack sx={{ color: 'text.secondary', gap: 0.75 }}>
                    <PhotoMeta icon={<Storage fontSize="inherit" />} text={`${formatContentType(photo.contentType)} · ${formatBytes(photo.sizeBytes)}`} />
                    <PhotoMeta icon={<CalendarMonth fontSize="inherit" />} text={formatPhotoTime(photo.takenAt || photo.createdAt)} />
                  </Stack>
                  {!!(photo.tags ?? []).length && (
                    <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 0.75 }}>
                      {(photo.tags ?? []).slice(0, 3).map((tag) => (
                        <Chip icon={<LocalOffer />} key={tag.id} label={tag.name} size="small" sx={{ maxWidth: '100%' }} />
                      ))}
                      {(photo.tags ?? []).length > 3 && <Chip label={`+${photo.tags.length - 3}`} size="small" />}
                    </Stack>
                  )}
                </Stack>
              </CardContent>
            </Card>
          </Grid>
        ))}
        {!photos.length && (
          <Grid size={{ xs: 12 }}>
            <Alert severity="info">这里暂时还没有公开图片。</Alert>
          </Grid>
        )}
      </Grid>
      <ImagePreviewDialog
        images={previewItems}
        index={previewIndex < 0 ? 0 : previewIndex}
        onClose={() => setPreviewIndex(-1)}
        onIndexChange={setPreviewIndex}
        open={previewIndex >= 0}
      />
      <SubmissionDialog event={event} onClose={() => setSubmitOpen(false)} onSubmitted={load} open={submitOpen} />
      <SubmissionReviewDialog event={event} onChanged={load} onClose={() => setReviewOpen(false)} open={reviewOpen} />
      <EventEditorDialog event={event} mode="edit" onClose={() => setEditorOpen(false)} onSaved={handleEventSaved} open={editorOpen} />

      <Paper component="form" onSubmit={handlePhotoFormSubmit} sx={{ display: editingPhoto ? 'block' : 'none', p: 3 }}>
        <Typography sx={{ fontWeight: 800, mb: 2 }} variant="h6">图片属性 #{editingPhoto?.id}</Typography>
        <Stack sx={{ gap: 2 }}>
          <TextField
            fullWidth
            label="摄影师署名"
            onChange={(e) => setPhotoForm((prev) => ({ ...prev, photographerName: e.target.value }))}
            value={photoForm.photographerName}
          />
          <FormControl fullWidth>
            <InputLabel>可见性</InputLabel>
            <Select
              label="可见性"
              onChange={(e: SelectChangeEvent) => setPhotoForm((prev) => ({ ...prev, visibility: e.target.value as Photo['visibility'] }))}
              value={photoForm.visibility}
            >
              <MenuItem value="public">公开</MenuItem>
              <MenuItem value="protected">密码访问</MenuItem>
              <MenuItem value="private">仅管理员</MenuItem>
            </Select>
          </FormControl>
          <TextField
            fullWidth
            helperText="留空则不修改访问密码"
            label="访问密码"
            onChange={(e) => setPhotoForm((prev) => ({ ...prev, accessPassword: e.target.value }))}
            type="password"
            value={photoForm.accessPassword}
          />
          <TextField
            fullWidth
            helperText="空格或逗号分隔，例如 #合照 #舞台"
            label="标签"
            onChange={(e) => setPhotoForm((prev) => ({ ...prev, tags: e.target.value }))}
            value={photoForm.tags}
          />
          <Stack direction="row" sx={{ gap: 1, justifyContent: 'flex-end' }}>
            <Button onClick={() => setEditingPhoto(null)}>取消</Button>
            <Button type="submit" variant="contained">保存</Button>
          </Stack>
        </Stack>
      </Paper>

      <ConfirmDialog
        onCancel={() => setDeleteConfirm(null)}
        onConfirm={deleteConfirm?.type === 'batch' ? confirmBatchDelete : confirmDeletePhoto}
        open={Boolean(deleteConfirm)}
        subtitle={
          deleteConfirm?.type === 'batch'
            ? `确定批量删除选中的 ${selectedIds.length} 张图片吗？`
            : deleteConfirm?.type === 'single'
              ? `确定删除图片 #${deleteConfirm.photo.id} 吗？`
              : ''
        }
        title="删除图片"
      />
    </Stack>
  );
}

function PhotoMeta({ icon, text }: { icon: React.ReactNode; text: string }) {
  return (
    <Stack direction="row" sx={{ alignItems: 'center', fontSize: 13, gap: 0.75, minWidth: 0 }}>
      <Box sx={{ alignItems: 'center', display: 'inline-flex', flexShrink: 0 }}>{icon}</Box>
      <Typography color="text.secondary" noWrap variant="caption">
        {text}
      </Typography>
    </Stack>
  );
}

function formatContentType(contentType?: string) {
  if (!contentType) return '未知类型';
  const [, subtype] = contentType.split('/');
  return (subtype || contentType).toUpperCase();
}

function formatBytes(size?: number) {
  if (!size || size <= 0) return '未知大小';
  const units = ['B', 'KB', 'MB', 'GB'];
  let value = size;
  let unitIndex = 0;
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex += 1;
  }
  return `${value.toFixed(value >= 10 || unitIndex === 0 ? 0 : 1)} ${units[unitIndex]}`;
}

function formatPhotoTime(value?: string) {
  if (!value) return '时间未知';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '时间未知';
  return new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium' }).format(date);
}

function downloadFilename(eventTitle: string, photo: Photo) {
  const extension = extensionFromContentType(photo.contentType) || extensionFromURL(photo.url) || 'jpg';
  const safeTitle = eventTitle.trim().replace(/[\\/:*?"<>|]+/g, '_') || 'fluffcatch';
  return `${safeTitle}-${photo.id}.${extension}`;
}

function extensionFromContentType(contentType?: string) {
  switch ((contentType || '').toLowerCase()) {
    case 'image/jpeg':
      return 'jpg';
    case 'image/png':
      return 'png';
    case 'image/gif':
      return 'gif';
    case 'image/webp':
      return 'webp';
    case 'image/avif':
      return 'avif';
    default:
      return '';
  }
}

function extensionFromURL(url: string) {
  try {
    const pathname = new URL(url, window.location.origin).pathname;
    const match = pathname.match(/\.([a-z0-9]+)$/i);
    return match?.[1]?.toLowerCase() || '';
  } catch {
    return '';
  }
}

function formatDateRange(start?: string, end?: string) {
  if (!start) return '时间待定';
  const formatter = new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'short' });
  const startText = formatter.format(new Date(start));
  if (!end) return startText;
  return `${startText} - ${formatter.format(new Date(end))}`;
}
