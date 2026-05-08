import { CalendarMonth, CameraAlt, CheckCircle, CloudDownload, CloudUpload, ContentCopy, Delete, Edit, Favorite, FavoriteBorder, LocalOffer, LocationOn, PhotoLibrary, PlayCircle, QrCode2, Storage } from '@mui/icons-material';
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
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  Grid,
  InputLabel,
  MenuItem,
  Pagination,
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
import { batchDeletePhotos, batchUpdatePhotos, deletePhoto, getEvent, getPhotos, likePhoto, unlockEventPrivatePhotos, updatePhoto, type EventCard, type Photo } from '../../api/client';
import { getCachedMe, refreshMe, subscribeAuthState } from '../../api/authState';
import { BatchDownloadDialog } from '../../components/BatchDownloadDialog';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { ImagePreviewDialog } from '../../components/ImagePreviewDialog';
import { EventEditorDialog } from '../../components/EventEditorDialog';
import { SubmissionDialog } from '../../components/SubmissionDialog';
import { SubmissionReviewDialog } from '../../components/SubmissionReviewDialog';
import { formatEventLocation } from '../../utils/eventLocation';
import { downloadPhoto } from '../../utils/download';
import { VisibilityOff, Visibility as VisibilityIcon } from '@mui/icons-material';

export function EventDetailPage() {
  const eventId = Number(useParams().eventId);
  const [event, setEvent] = useState<EventCard | null>(null);
  const [publicPhotos, setPublicPhotos] = useState<Photo[]>([]);
  const [privatePhotos, setPrivatePhotos] = useState<Photo[]>([]);
  const [publicPage, setPublicPage] = useState(1);
  const [privatePage, setPrivatePage] = useState(1);
  const [publicTotalPages, setPublicTotalPages] = useState(1);
  const [privateTotalPages, setPrivateTotalPages] = useState(1);
  const [publicTotal, setPublicTotal] = useState(0);
  const [privateTotal, setPrivateTotal] = useState(0);
  const [publicPageSize, setPublicPageSize] = useState(() => {
    if (typeof window === 'undefined') return 0;
    return Number(window.localStorage.getItem('fluffcatch:gallery-public-page-size') || 0);
  });
  const [privatePageSize, setPrivatePageSize] = useState(() => {
    if (typeof window === 'undefined') return 0;
    return Number(window.localStorage.getItem('fluffcatch:gallery-private-page-size') || 0);
  });
  const [effectivePublicPageSize, setEffectivePublicPageSize] = useState(publicPageSize || 24);
  const [effectivePrivatePageSize, setEffectivePrivatePageSize] = useState(privatePageSize || 24);
  const [previewIndex, setPreviewIndex] = useState(-1);
  const [submitOpen, setSubmitOpen] = useState(false);
  const [error, setError] = useState('');
  const [message, setMessage] = useState('');
  const [loading, setLoading] = useState(true);
  const [publicLoading, setPublicLoading] = useState(false);
  const [privateLoading, setPrivateLoading] = useState(false);
  const [authenticated, setAuthenticated] = useState(() => getCachedMe().authenticated);
  const [likeError, setLikeError] = useState('');
  const [shareOpen, setShareOpen] = useState(false);
  const [reviewOpen, setReviewOpen] = useState(false);
  const [editorOpen, setEditorOpen] = useState(false);
  const [manageMode, setManageMode] = useState(false);
  const [selectedIds, setSelectedIds] = useState<number[]>([]);
  const [editingPhoto, setEditingPhoto] = useState<Photo | null>(null);
  const [photoForm, setPhotoForm] = useState({ photographerName: '', tags: '', visibility: 'public' as Photo['visibility'] });
  const [copyMessage, setCopyMessage] = useState('');
  const [deleteConfirm, setDeleteConfirm] = useState<{ type: 'batch' } | { type: 'single'; photo: Photo } | null>(null);
  const [downloadId, setDownloadId] = useState<number | null>(null);
  const [batchDownloadOpen, setBatchDownloadOpen] = useState(false);
  const [privateAccessPassword, setPrivateAccessPassword] = useState('');
  const [privateAccessUnlocked, setPrivateAccessUnlocked] = useState(() => (
    typeof window !== 'undefined' && Number.isFinite(eventId)
      ? window.sessionStorage.getItem(privateAccessCacheKey(eventId)) === '1'
      : false
  ));
  const [passwordDialogOpen, setPasswordDialogOpen] = useState(false);
  const [passwordError, setPasswordError] = useState('');
  const [unlockingPassword, setUnlockingPassword] = useState(false);
  const [pendingProtectedPhoto, setPendingProtectedPhoto] = useState<Photo | null>(null);

  const photos = useMemo(() => [...publicPhotos, ...privatePhotos], [publicPhotos, privatePhotos]);

  function load(options: { publicPage?: number; privatePage?: number; publicPageSize?: number; privatePageSize?: number; refreshEvent?: boolean } = {}) {
    const targetPublicPage = options.publicPage ?? publicPage;
    const targetPrivatePage = options.privatePage ?? privatePage;
    const targetPublicPageSize = (options.publicPageSize ?? publicPageSize) || undefined;
    const targetPrivatePageSize = (options.privatePageSize ?? privatePageSize) || undefined;
    setLoading(true);
    setError('');
    const eventPromise = options.refreshEvent === false && event ? Promise.resolve(event) : getEvent(eventId);
    return Promise.all([
      eventPromise,
      getPhotos(eventId, authenticated, targetPublicPage, targetPublicPageSize, 'public'),
      getPhotos(eventId, authenticated, targetPrivatePage, targetPrivatePageSize, 'private')
    ])
      .then(([eventData, publicData, privateData]) => {
        setEvent(eventData);
        setPublicPhotos(publicData.photos);
        setPrivatePhotos(privateData.photos);
        setPublicPage(publicData.page);
        setPrivatePage(privateData.page);
        setPublicTotalPages(publicData.totalPages || 1);
        setPrivateTotalPages(privateData.totalPages || 1);
        setPublicTotal(publicData.total);
        setPrivateTotal(privateData.total);
        setEffectivePublicPageSize(publicData.pageSize || targetPublicPageSize || 24);
        setEffectivePrivatePageSize(privateData.pageSize || targetPrivatePageSize || 24);
        return [...publicData.photos, ...privateData.photos];
      })
      .catch((err: unknown) => {
        setError(err instanceof Error ? err.message : '加载失败');
        return [];
      })
      .finally(() => setLoading(false));
  }

  function applyPublicPhotoPage(photoData: Awaited<ReturnType<typeof getPhotos>>) {
    setPublicPhotos(photoData.photos);
    setPublicPage(photoData.page);
    setPublicTotalPages(photoData.totalPages || 1);
    setPublicTotal(photoData.total);
    setEffectivePublicPageSize(photoData.pageSize || 24);
    return photoData.photos;
  }

  function applyPrivatePhotoPage(photoData: Awaited<ReturnType<typeof getPhotos>>) {
    setPrivatePhotos(photoData.photos);
    setPrivatePage(photoData.page);
    setPrivateTotalPages(photoData.totalPages || 1);
    setPrivateTotal(photoData.total);
    setEffectivePrivatePageSize(photoData.pageSize || 24);
    return photoData.photos;
  }

  function loadPublicPhotos(options: { page?: number; pageSize?: number } = {}) {
    const targetPage = options.page ?? publicPage;
    const targetPageSize = (options.pageSize ?? publicPageSize) || undefined;
    setPublicLoading(true);
    setError('');
    return getPhotos(eventId, authenticated, targetPage, targetPageSize, 'public')
      .then(applyPublicPhotoPage)
      .catch((err: unknown) => {
        setError(err instanceof Error ? err.message : '公开返图加载失败');
        return [];
      })
      .finally(() => setPublicLoading(false));
  }

  function loadPrivatePhotos(options: { page?: number; pageSize?: number } = {}) {
    const targetPage = options.page ?? privatePage;
    const targetPageSize = (options.pageSize ?? privatePageSize) || undefined;
    setPrivateLoading(true);
    setError('');
    return getPhotos(eventId, authenticated, targetPage, targetPageSize, 'private')
      .then(applyPrivatePhotoPage)
      .catch((err: unknown) => {
        setError(err instanceof Error ? err.message : '非公开返图加载失败');
        return [];
      })
      .finally(() => setPrivateLoading(false));
  }

  function refreshPhotoSections() {
    setError('');
    return Promise.all([loadPublicPhotos(), loadPrivatePhotos()]).then(([nextPublic, nextPrivate]) => [...nextPublic, ...nextPrivate]);
  }

  function handlePublicPageChange(_: React.ChangeEvent<unknown>, value: number) {
    setPublicPage(value);
    setSelectedIds([]);
    void loadPublicPhotos({ page: value });
  }

  function handlePrivatePageChange(_: React.ChangeEvent<unknown>, value: number) {
    setPrivatePage(value);
    setSelectedIds([]);
    void loadPrivatePhotos({ page: value });
  }

  function handlePublicPageSizeChange(event: SelectChangeEvent) {
    const nextPageSize = Number(event.target.value) || 24;
    setPublicPageSize(nextPageSize);
    setEffectivePublicPageSize(nextPageSize);
    setPublicPage(1);
    if (typeof window !== 'undefined') {
      window.localStorage.setItem('fluffcatch:gallery-public-page-size', String(nextPageSize));
    }
    void loadPublicPhotos({ page: 1, pageSize: nextPageSize });
  }

  function handlePrivatePageSizeChange(event: SelectChangeEvent) {
    const nextPageSize = Number(event.target.value) || 24;
    setPrivatePageSize(nextPageSize);
    setEffectivePrivatePageSize(nextPageSize);
    setPrivatePage(1);
    if (typeof window !== 'undefined') {
      window.localStorage.setItem('fluffcatch:gallery-private-page-size', String(nextPageSize));
    }
    void loadPrivatePhotos({ page: 1, pageSize: nextPageSize });
  }

  useEffect(() => {
    setPublicPage(1);
    setPrivatePage(1);
    void load({ publicPage: 1, privatePage: 1 });
  }, [eventId, authenticated]);

  useEffect(() => {
    if (typeof window === 'undefined' || !eventId) return;
    setPrivateAccessUnlocked(window.sessionStorage.getItem(privateAccessCacheKey(eventId)) === '1');
  }, [eventId]);

  function photoURL(photo: Photo, variant: 'original' | 'thumbnail' = 'original') {
    return variant === 'thumbnail' ? photo.thumbnailUrl || photo.url : photo.url;
  }

  function isVideoPhoto(photo: Photo) {
    return photo.contentType.toLowerCase().startsWith('video/');
  }

  const restrictedPhotos = privatePhotos;

  useEffect(() => {
    const unsubscribe = subscribeAuthState((payload) => setAuthenticated(payload.authenticated));
    refreshMe().catch(() => undefined);
    return unsubscribe;
  }, []);

  const previewItems = useMemo(
    () =>
      photos.map((photo) => ({
        contentType: photo.contentType,
        src: photoURL(photo),
        subtitle: [
          photo.photographerName ? `摄影师：${photo.photographerName}` : '匿名投稿',
          `${formatContentType(photo.contentType)} · ${formatBytes(photo.sizeBytes)}`,
          `${photo.likeCount || 0} 个喜欢`
        ].join(' · '),
        title: event?.title || '媒体预览'
      })),
    [event?.title, photos]
  );

  async function handleLike(photo: Photo) {
    setLikeError('');
    updatePhotoInLists(photo.id, (item) => ({ ...item, liked: true, likeCount: (item.likeCount || 0) + (item.liked ? 0 : 1) }));
    try {
      const result = await likePhoto(photo.id);
      updatePhotoInLists(photo.id, (item) => ({ ...item, liked: result.liked, likeCount: result.likeCount }));
    } catch (err) {
      setLikeError(err instanceof Error ? err.message : '点赞失败');
      updatePhotoInLists(photo.id, () => photo);
    }
  }

  async function unlockProtectedPhoto() {
    if (!pendingProtectedPhoto || !privateAccessPassword.trim()) return;
    setPasswordError('');
    setUnlockingPassword(true);
    const targetID = pendingProtectedPhoto.id;
    try {
      await unlockEventPrivatePhotos(eventId, privateAccessPassword.trim());
      if (typeof window !== 'undefined') {
        window.sessionStorage.setItem(privateAccessCacheKey(eventId), '1');
      }
      setPrivateAccessUnlocked(true);
      setPrivateAccessPassword('');
      const nextPhotos = await loadPrivatePhotos();
      const nextIndex = nextPhotos.findIndex((photo) => photo.id === targetID && photo.accessGranted);
      if (nextIndex < 0) {
        setPasswordError('已验证口令，但图片仍未解锁，请刷新后重试。');
        return;
      }
      setPasswordDialogOpen(false);
      setPendingProtectedPhoto(null);
      setPreviewIndex(nextIndex);
    } catch {
      setPasswordError('访问口令不正确，请检查后重试。');
    } finally {
      setUnlockingPassword(false);
    }
  }

  function openPhoto(photo: Photo, index: number) {
    if (photo.visibility === 'private' && !authenticated && !photo.accessGranted) {
      setPendingProtectedPhoto(photo);
      setPasswordError('');
      setPasswordDialogOpen(true);
      return;
    }
    setPreviewIndex(index);
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

  async function handleSingleDownload(photo: Photo) {
    setDownloadId(photo.id);
    try {
      await downloadPhoto(photoURL(photo), downloadFilename(event?.title ?? 'fluffcatch', photo));
    } catch (err) {
      setError(err instanceof Error ? err.message : '下载失败');
    } finally {
      setDownloadId(null);
    }
  }

  function handleBatchDownload() {
    if (!selectedIds.length || !event) return;
    setBatchDownloadOpen(true);
  }

  const batchDownloadItems = useMemo(() => {
    if (!event) return [];
    return selectedIds
      .map((id) => photos.find((p) => p.id === id))
      .filter((p): p is Photo => !!p)
      .map((p) => ({ url: photoURL(p), filename: downloadFilename(event.title, p) }));
  }, [event, photos, selectedIds]);

  async function handleBatchDelete() {
    if (!selectedIds.length) return;
    setDeleteConfirm({ type: 'batch' });
  }

  async function confirmBatchDelete(headers: Record<string, string>) {
    setDeleteConfirm(null);
    setError('');
    try {
      await batchDeletePhotos(selectedIds, headers);
      setMessage(`已删除 ${selectedIds.length} 张图片。`);
      setSelectedIds([]);
      void refreshPhotoSections();
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
      removePhotoFromLists(photo.id);
    } catch (err) {
      setError(err instanceof Error ? err.message : '删除图片失败');
    }
  }

  function startEditPhoto(photo: Photo) {
    setEditingPhoto(photo);
    setPhotoForm({
      photographerName: photo.photographerName || '',
      tags: (photo.tags ?? []).map((tag) => tag.name).join(' '),
      visibility: photo.visibility
    });
  }

  function applyUpdatedPhoto(photo: Photo) {
    setPublicPhotos((prev) => {
      const without = prev.filter((item) => item.id !== photo.id);
      return photo.visibility === 'public' ? upsertPhoto(without, photo) : without;
    });
    setPrivatePhotos((prev) => {
      const without = prev.filter((item) => item.id !== photo.id);
      return photo.visibility === 'private' ? upsertPhoto(without, photo) : without;
    });
  }

  function removePhotoFromLists(photoId: number) {
    setPublicPhotos((prev) => prev.filter((item) => item.id !== photoId));
    setPrivatePhotos((prev) => prev.filter((item) => item.id !== photoId));
  }

  function updatePhotoInLists(photoId: number, updater: (photo: Photo) => Photo) {
    setPublicPhotos((prev) => prev.map((item) => (item.id === photoId ? updater(item) : item)));
    setPrivatePhotos((prev) => prev.map((item) => (item.id === photoId ? updater(item) : item)));
  }

  async function handlePhotoFormSubmit(formEvent: React.FormEvent<HTMLFormElement>) {
    formEvent.preventDefault();
    if (!editingPhoto) return;
    setError('');
    try {
      const result = await updatePhoto(editingPhoto.id, {
        photographerName: photoForm.photographerName,
        tags: photoForm.tags.split(/[\s,，]+/).filter(Boolean),
        visibility: photoForm.visibility
      });
      setEditingPhoto(null);
      setMessage('图片属性已更新。');
      applyUpdatedPhoto(result.photo);
    } catch (err) {
      setError(err instanceof Error ? err.message : '保存图片属性失败');
    }
  }

  function renderPhotoCard(photo: Photo, index: number, restricted: boolean) {
    const locked = restricted && photo.visibility === 'private' && !authenticated && !photo.accessGranted;
    const video = isVideoPhoto(photo);
    return (
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
          <CardActionArea onClick={() => manageMode && authenticated ? togglePhotoSelection(photo.id) : openPhoto(photo, index)} sx={{ display: 'block' }}>
            <Box sx={{ position: 'relative' }}>
              {video && !locked ? (
                <Box
                  component="video"
                  muted
                  preload="metadata"
                  src={photoURL(photo)}
                  sx={{
                    aspectRatio: '4 / 3',
                    bgcolor: 'common.black',
                    cursor: manageMode ? 'pointer' : 'zoom-in',
                    display: 'block',
                    objectFit: 'cover',
                    width: '100%'
                  }}
                />
              ) : (
                <Box
                  sx={{
                    alignItems: 'center',
                    aspectRatio: '4 / 3',
                    bgcolor: 'action.hover',
                    cursor: manageMode ? 'pointer' : locked ? 'pointer' : 'zoom-in',
                    display: 'flex',
                    justifyContent: 'center',
                    overflow: 'hidden',
                    width: '100%'
                  }}
                >
                  <CardMedia
                    component="img"
                    image={photoURL(photo, 'thumbnail')}
                    sx={{
                      display: 'block',
                      filter: locked ? 'blur(12px) saturate(0.9)' : undefined,
                      height: '100%',
                      objectFit: 'cover',
                      objectPosition: 'center',
                      transform: locked ? 'scale(1.06)' : undefined,
                      width: '100%'
                    }}
                  />
                </Box>
              )}
              {video && (
                <Box sx={{ alignItems: 'center', bgcolor: 'rgba(15,23,42,0.62)', borderRadius: 999, color: 'white', display: 'flex', left: 12, p: 0.75, position: 'absolute', top: 12 }}>
                  <PlayCircle fontSize="small" />
                </Box>
              )}
              {restricted && (
                <Box sx={{ bgcolor: 'rgba(15,23,42,0.55)', bottom: 0, color: 'white', left: 0, px: 1.25, py: 0.75, position: 'absolute', right: 0 }}>
                  <Typography sx={{ fontWeight: 800 }} variant="body2">
                    {!authenticated && photo.accessGranted ? '已解锁' : video ? '私密视频' : '私密图片'}
                  </Typography>
                </Box>
              )}
            </Box>
          </CardActionArea>
          <CardContent sx={{ p: 1.75 }}>
            <Stack sx={{ gap: 1.25 }}>
              <Stack direction="row" sx={{ alignItems: 'center', gap: 1, minWidth: 0 }}>
                <CameraAlt color="primary" fontSize="small" />
                <Typography noWrap sx={{ flex: 1, fontWeight: 700 }}>
                  {photo.photographerName || '匿名投稿'}
                </Typography>
                {!manageMode && photo.visibility === 'public' && (
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
              {!manageMode && !locked && (
                <Button
                  disabled={downloadId === photo.id}
                  onClick={(clickEvent) => { clickEvent.stopPropagation(); void handleSingleDownload(photo); }}
                  size="small"
                  startIcon={downloadId === photo.id ? <CircularProgress size={16} /> : <CloudDownload />}
                  variant="outlined"
                >
                  下载原文件
                </Button>
              )}
              {locked && (
                <Button onClick={() => openPhoto(photo, index)} size="small" variant="outlined">
                  输入密码查看
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
    );
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
        sx={(theme) => ({
          bgcolor: 'background.paper',
          border: '1px solid',
          borderColor: theme.palette.mode === 'dark' ? 'rgba(226, 232, 240, 0.18)' : 'divider',
          borderRadius: 2,
          boxShadow: 'none',
          overflow: 'hidden',
          p: { xs: 2.5, md: 3 }
        })}
      >
        <Stack direction={{ xs: 'column', md: 'row' }} sx={{ gap: 3 }}>
          <Box
            sx={{
              bgcolor: 'action.hover',
              borderRadius: 3,
              flexShrink: 0,
              overflow: 'hidden',
              width: { xs: '100%', md: 360 }
            }}
          >
            {event.coverUrl ? (
              <Box component="img" src={event.coverUrl} sx={{ aspectRatio: '16 / 10', display: 'block', objectFit: 'cover', width: '100%' }} />
            ) : (
              <Box
                sx={{
                  aspectRatio: '16 / 10',
                  bgcolor: 'action.hover'
                }}
              />
            )}
          </Box>
          <Stack sx={{ flex: 1, gap: 2, justifyContent: 'center', minWidth: 0 }}>
            <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 1 }}>
              <Chip color={event.isPublic ? 'success' : 'default'} label={event.isPublic ? '公开画廊' : '隐藏'} size="small" />
              <Chip color={event.submissionEnabled ? 'primary' : 'default'} label={event.submissionEnabled ? '开放投稿' : '投稿关闭'} size="small" />
              <Chip label={`${event.photoCount || publicTotal + privateTotal || photos.length} 张图片`} size="small" />
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
                    {manageMode ? '退出管理' : '管理媒体'}
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
            <Box sx={{ bgcolor: '#ffffff', border: '1px solid', borderColor: 'divider', borderRadius: 2, display: 'inline-flex', p: 1.5, width: 'fit-content' }}>
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
        <Paper sx={{ borderRadius: 3, p: { xs: 1.5, sm: 2 } }} variant="outlined">
          <Stack direction="row" sx={{ alignItems: 'center', flexWrap: 'wrap', gap: 1 }}>
            <Button
              disabled={!publicPhotos.length}
              onClick={() => {
                const pubIds = publicPhotos.map((p) => p.id);
                setSelectedIds((prev) => pubIds.every((id) => prev.includes(id)) ? prev.filter((id) => !pubIds.includes(id)) : Array.from(new Set([...prev, ...pubIds])));
              }}
              size="small"
              variant="outlined"
            >
              全选公开
            </Button>
            <Button
              disabled={!restrictedPhotos.length}
              onClick={() => {
                const privIds = restrictedPhotos.map((p) => p.id);
                setSelectedIds((prev) => privIds.every((id) => prev.includes(id)) ? prev.filter((id) => !privIds.includes(id)) : Array.from(new Set([...prev, ...privIds])));
              }}
              size="small"
              variant="outlined"
            >
              全选私密
            </Button>
            <Button disabled={!selectedIds.length} onClick={() => setSelectedIds([])} size="small">
              取消
            </Button>
            <Typography color="text.secondary" sx={{ mx: 0.5 }} variant="body2">
              已选 {selectedIds.length} 张
            </Typography>
            <Box sx={{ flex: 1 }} />
            <Button
              disabled={!selectedIds.length}
              onClick={() => {
                setError('');
                batchUpdatePhotos(selectedIds, 'public')
                  .then((result) => { setMessage(result.message); setSelectedIds([]); void refreshPhotoSections(); })
                  .catch((err: unknown) => setError(err instanceof Error ? err.message : '批量设置失败'));
              }}
              size="small"
              variant="outlined"
            >
              设为公开
            </Button>
            <Button
              disabled={!selectedIds.length}
              onClick={() => {
                setError('');
                batchUpdatePhotos(selectedIds, 'private')
                  .then((result) => { setMessage(result.message); setSelectedIds([]); void refreshPhotoSections(); })
                  .catch((err: unknown) => setError(err instanceof Error ? err.message : '批量设置失败'));
              }}
              size="small"
              variant="outlined"
            >
              设为私密
            </Button>
            <Button
              disabled={!selectedIds.length}
              onClick={handleBatchDownload}
              size="small"
              startIcon={<CloudDownload />}
              variant="outlined"
            >
              下载{selectedIds.length ? ` ${selectedIds.length}` : ''}
            </Button>
            <Button
              color="error"
              disabled={!selectedIds.length}
              onClick={handleBatchDelete}
              size="small"
              startIcon={<Delete />}
              variant="outlined"
            >
              删除{selectedIds.length ? ` ${selectedIds.length}` : ''}
            </Button>
          </Stack>
        </Paper>
      )}

      <PhotoSection title="公开返图" subtitle={`${publicTotal} 个公开媒体文件`}>
        <PhotoSectionBody loading={publicLoading}>
          {publicPhotos.length ? (
            <Grid container spacing={2}>
              {publicPhotos.map((photo) => renderPhotoCard(photo, photos.findIndex((item) => item.id === photo.id), false))}
            </Grid>
          ) : (
            <Alert severity="info">这里暂时还没有公开返图。</Alert>
          )}
        </PhotoSectionBody>
        <SectionPagination count={publicTotalPages} effectivePageSize={effectivePublicPageSize} onChange={handlePublicPageChange} onPageSizeChange={handlePublicPageSizeChange} page={publicPage} />
      </PhotoSection>

      {(authenticated || privateTotal > 0) && (
        <PhotoSection title="非公开返图" subtitle={authenticated ? `${privateTotal} 个私密媒体，管理员可直接查看` : privateAccessUnlocked ? `${privateTotal} 个私密媒体，已在当前浏览器会话中解锁` : `${privateTotal} 个私密媒体，需要输入这个兽聚的私密口令后查看`}>
          <PhotoSectionBody loading={privateLoading}>
            <Grid container spacing={2}>
              {restrictedPhotos.map((photo) => renderPhotoCard(photo, photos.findIndex((item) => item.id === photo.id), true))}
            </Grid>
            {!restrictedPhotos.length && <Alert severity="info">这里暂时还没有非公开返图。</Alert>}
          </PhotoSectionBody>
          <SectionPagination count={privateTotalPages} effectivePageSize={effectivePrivatePageSize} onChange={handlePrivatePageChange} onPageSizeChange={handlePrivatePageSizeChange} page={privatePage} />
        </PhotoSection>
      )}

      <ImagePreviewDialog
        images={previewItems}
        index={previewIndex < 0 ? 0 : previewIndex}
        onClose={() => setPreviewIndex(-1)}
        onIndexChange={setPreviewIndex}
        open={previewIndex >= 0}
      />
      <SubmissionDialog event={event} onClose={() => setSubmitOpen(false)} onSubmitted={() => load({ publicPage: 1, privatePage: 1 })} open={submitOpen} />
      <SubmissionReviewDialog event={event} onChanged={refreshPhotoSections} onClose={() => setReviewOpen(false)} open={reviewOpen} />
      <EventEditorDialog event={event} mode="edit" onClose={() => setEditorOpen(false)} onSaved={handleEventSaved} open={editorOpen} />

      <Dialog fullWidth maxWidth="xs" onClose={() => setPasswordDialogOpen(false)} open={passwordDialogOpen}>
        <DialogTitle>输入访问口令</DialogTitle>
        <DialogContent>
          <Stack component="form" id="gallery-password-form" onSubmit={(submitEvent) => { submitEvent.preventDefault(); void unlockProtectedPhoto(); }} sx={{ gap: 2, pt: 1 }}>
            <Typography color="text.secondary" variant="body2">
              这个兽聚的私密返图需要访问口令。验证通过后，本次浏览器会话内会保持解锁。
            </Typography>
            <TextField
              autoFocus
              error={Boolean(passwordError)}
              fullWidth
              helperText={passwordError || ''}
              label="访问口令"
              onChange={(inputEvent) => {
                setPrivateAccessPassword(inputEvent.target.value);
                setPasswordError('');
              }}
              type="password"
              value={privateAccessPassword}
            />
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button disabled={unlockingPassword} onClick={() => setPasswordDialogOpen(false)}>取消</Button>
          <Button disabled={!privateAccessPassword.trim() || unlockingPassword} form="gallery-password-form" type="submit" variant="contained">
            {unlockingPassword ? '验证中...' : '解锁查看'}
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog fullWidth maxWidth="sm" onClose={() => setEditingPhoto(null)} open={Boolean(editingPhoto)}>
        <DialogTitle>媒体属性 #{editingPhoto?.id}</DialogTitle>
        <DialogContent dividers>
          <Stack component="form" id="photo-property-form" onSubmit={handlePhotoFormSubmit} sx={{ gap: 2, pt: 1 }}>
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
                <MenuItem value="private">私密</MenuItem>
              </Select>
            </FormControl>
            <TextField
              fullWidth
              helperText="空格或逗号分隔，例如 #合照 #舞台"
              label="标签"
              onChange={(e) => setPhotoForm((prev) => ({ ...prev, tags: e.target.value }))}
              value={photoForm.tags}
            />
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setEditingPhoto(null)}>取消</Button>
          <Button form="photo-property-form" type="submit" variant="contained">保存</Button>
        </DialogActions>
      </Dialog>

      <BatchDownloadDialog
        items={batchDownloadItems}
        onClose={() => setBatchDownloadOpen(false)}
        open={batchDownloadOpen}
        zipName={event ? `${event.title}-fluffcatch.zip` : 'fluffcatch.zip'}
      />

      <ConfirmDialog
        onCancel={() => setDeleteConfirm(null)}
        onConfirm={deleteConfirm?.type === 'batch' ? confirmBatchDelete : confirmDeletePhoto}
        open={Boolean(deleteConfirm)}
        subtitle={
          deleteConfirm?.type === 'batch'
            ? `确定批量删除选中的 ${selectedIds.length} 个媒体文件吗？`
            : deleteConfirm?.type === 'single'
              ? `确定删除图片 #${deleteConfirm.photo.id} 吗？`
              : ''
        }
        title="删除媒体"
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

function PhotoSection({ children, subtitle, title }: { children: React.ReactNode; subtitle: string; title: string }) {
  return (
    <Paper sx={{ borderRadius: 3, p: { xs: 2, sm: 2.5 } }} variant="outlined">
      <Stack sx={{ gap: 2 }}>
        <Box>
          <Typography sx={{ fontWeight: 900 }} variant="h6">{title}</Typography>
          <Typography color="text.secondary" variant="body2">{subtitle}</Typography>
        </Box>
        {children}
      </Stack>
    </Paper>
  );
}

function PhotoSectionBody({ children, loading }: { children: React.ReactNode; loading: boolean }) {
  return (
    <Box sx={{ minHeight: 120, position: 'relative' }}>
      {children}
      {loading && (
        <Box
          sx={(theme) => ({
            alignItems: 'center',
            bgcolor: theme.palette.mode === 'dark' ? 'rgba(2, 6, 23, 0.46)' : 'rgba(255, 255, 255, 0.62)',
            borderRadius: 1.5,
            display: 'flex',
            inset: 0,
            justifyContent: 'center',
            position: 'absolute',
            zIndex: 3
          })}
        >
          <CircularProgress size={30} />
        </Box>
      )}
    </Box>
  );
}

function SectionPagination({
  count,
  effectivePageSize,
  onChange,
  onPageSizeChange,
  page
}: {
  count: number;
  effectivePageSize: number;
  onChange: (event: React.ChangeEvent<unknown>, value: number) => void;
  onPageSizeChange: (event: SelectChangeEvent) => void;
  page: number;
}) {
  return (
    <Stack
      direction={{ xs: 'column', sm: 'row' }}
      sx={(theme) => ({
        alignItems: 'center',
        bgcolor: theme.palette.mode === 'dark' ? 'rgba(15, 23, 42, 0.9)' : 'rgba(255, 255, 255, 0.92)',
        border: '1px solid',
        borderColor: theme.palette.mode === 'dark' ? 'rgba(226, 232, 240, 0.22)' : 'divider',
        borderRadius: 2,
        boxShadow: theme.palette.mode === 'dark' ? '0 10px 28px rgba(0,0,0,0.32)' : '0 10px 28px rgba(15,23,42,0.08)',
        gap: 1.5,
        justifyContent: 'center',
        mt: 1,
        px: 1.5,
        py: 1.25
      })}
    >
      {count > 1 && (
        <Pagination
          color="primary"
          count={count}
          onChange={onChange}
          page={Math.min(page, count)}
          showFirstButton
          showLastButton
          size="large"
        />
      )}
      <FormControl size="small" sx={{ minWidth: 132 }}>
        <InputLabel>每页数量</InputLabel>
        <Select label="每页数量" onChange={onPageSizeChange} value={String(effectivePageSize)}>
          {[12, 24, 36, 48, 72, 100].map((size) => (
            <MenuItem key={size} value={String(size)}>{size}</MenuItem>
          ))}
        </Select>
      </FormControl>
    </Stack>
  );
}

function upsertPhoto(photos: Photo[], photo: Photo) {
  if (photos.some((item) => item.id === photo.id)) {
    return photos.map((item) => (item.id === photo.id ? photo : item));
  }
  return [photo, ...photos];
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
    case 'video/mp4':
      return 'mp4';
    case 'video/webm':
      return 'webm';
    case 'video/ogg':
      return 'ogv';
    case 'video/quicktime':
      return 'mov';
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

function privateAccessCacheKey(eventID: number) {
  return `fluffcatch:private-access:${eventID}`;
}

function formatDateRange(start?: string, end?: string) {
  if (!start) return '时间待定';
  const formatter = new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'short' });
  const startText = formatter.format(new Date(start));
  if (!end) return startText;
  return `${startText} - ${formatter.format(new Date(end))}`;
}
