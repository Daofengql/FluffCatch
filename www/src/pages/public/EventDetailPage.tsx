import { CalendarMonth, CameraAlt, CloudDownload, CloudUpload, Delete, Edit, Favorite, FavoriteBorder, InfoOutlined, Link as LinkIcon, LocalOffer, LocationOn, PhotoLibrary, PlayCircle, Settings as SettingsIcon, Storage } from '@mui/icons-material';
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
  Divider,
  FormControl,
  Grid,
  IconButton,
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
import { useEffect, useMemo, useRef, useState } from 'react';
import { useOutletContext, useParams } from 'react-router-dom';
import { batchDeletePhotos, batchUpdatePhotos, deletePhoto, getEvent, getPhotos, likePhoto, setEventCoverFromPhoto, unlockEventPrivatePhotos, updatePhoto, type EventCard, type GalleryFilters, type Photo } from '../../api/client';
import { getCachedMe, refreshMe, subscribeAuthState } from '../../api/authState';
import { BatchDownloadDialog } from '../../components/BatchDownloadDialog';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { ImagePreviewDialog } from '../../components/ImagePreviewDialog';
import { EventEditorDialog } from '../../components/EventEditorDialog';
import { SubmissionDialog } from '../../components/SubmissionDialog';
import { SubmissionLinkDialog } from '../../components/SubmissionLinkDialog';
import { SubmissionReviewDialog } from '../../components/SubmissionReviewDialog';
import { formatEventLocation } from '../../utils/eventLocation';
import { downloadPhoto } from '../../utils/download';

export function EventDetailPage() {
  const eventId = Number(useParams().eventId);
  const { setPageTitleOverride } = useOutletContext<{ setPageTitleOverride?: (value: string) => void }>();
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
  const [reviewOpen, setReviewOpen] = useState(false);
  const [linkDialogOpen, setLinkDialogOpen] = useState(false);
  const [editorOpen, setEditorOpen] = useState(false);
  const [manageMode, setManageMode] = useState(false);
  const [selectedIds, setSelectedIds] = useState<number[]>([]);
  const [editingPhoto, setEditingPhoto] = useState<Photo | null>(null);
  const [photoForm, setPhotoForm] = useState({ photographerName: '', takenAt: '', visibility: 'public' as Photo['visibility'] });
  const [galleryFilters, setGalleryFilters] = useState<GalleryFilters>({ mediaType: 'all', sort: 'latest' });
  const [galleryFilterDraft, setGalleryFilterDraft] = useState<GalleryFilters>({ mediaType: 'all', sort: 'latest' });
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [deleteConfirm, setDeleteConfirm] = useState<{ type: 'batch' } | { type: 'single'; photo: Photo } | null>(null);
  const [cardMenu, setCardMenu] = useState<{ mouseX: number; mouseY: number; photo: Photo } | null>(null);
  const [propertyPhoto, setPropertyPhoto] = useState<Photo | null>(null);
  const [quickTagPhoto, setQuickTagPhoto] = useState<Photo | null>(null);
  const [quickTags, setQuickTags] = useState<string[]>([]);
  const [quickTagInput, setQuickTagInput] = useState('');
  const [quickTagSaving, setQuickTagSaving] = useState(false);
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
  const didApplyInitialFiltersRef = useRef(false);
  const didInitialLoadRef = useRef(false);
  const longPressTimerRef = useRef<number | null>(null);
  const longPressPointRef = useRef<{ x: number; y: number } | null>(null);
  const suppressNextCardClickRef = useRef(false);
  const submissionChangedRef = useRef(false);

  const photos = useMemo(() => [...publicPhotos, ...privatePhotos], [publicPhotos, privatePhotos]);

  useEffect(() => {
    setPageTitleOverride?.(event?.title || '');
    return () => setPageTitleOverride?.('');
  }, [event?.title, setPageTitleOverride]);

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
      getPhotos(eventId, authenticated, targetPublicPage, targetPublicPageSize, 'public', galleryFilters),
      getPhotos(eventId, authenticated, targetPrivatePage, targetPrivatePageSize, 'private', galleryFilters)
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
    return getPhotos(eventId, authenticated, targetPage, targetPageSize, 'public', galleryFilters)
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
    return getPhotos(eventId, authenticated, targetPage, targetPageSize, 'private', galleryFilters)
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

  function refreshEventOnly() {
    return getEvent(eventId)
      .then(setEvent)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '兽聚信息刷新失败'));
  }

  function handleSubmissionUploaded() {
    submissionChangedRef.current = true;
  }

  function closeSubmissionDialog() {
    setSubmitOpen(false);
    if (!submissionChangedRef.current) return;
    submissionChangedRef.current = false;
    setPublicPage(1);
    setPrivatePage(1);
    setSelectedIds([]);
    void Promise.all([refreshEventOnly(), loadPublicPhotos({ page: 1 }), loadPrivatePhotos({ page: 1 })]);
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
    if (!didInitialLoadRef.current || !event) {
      didInitialLoadRef.current = true;
      void load({ publicPage: 1, privatePage: 1 });
      return;
    }
    void Promise.all([refreshEventOnly(), loadPublicPhotos({ page: 1 }), loadPrivatePhotos({ page: 1 })]);
  }, [eventId, authenticated]);

  useEffect(() => {
    if (!didApplyInitialFiltersRef.current) {
      didApplyInitialFiltersRef.current = true;
      return;
    }
    setPublicPage(1);
    setPrivatePage(1);
    setSelectedIds([]);
    void Promise.all([loadPublicPhotos({ page: 1 }), loadPrivatePhotos({ page: 1 })]);
  }, [galleryFilters.tag, galleryFilters.photographer, galleryFilters.mediaType, galleryFilters.sort]);

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

  useEffect(() => () => {
    clearLongPressTimer();
  }, []);

  useEffect(() => {
    if (!cardMenu) return undefined;
    const close = () => closeCardMenu();
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') closeCardMenu();
    };
    window.addEventListener('click', close);
    window.addEventListener('scroll', close, { passive: true });
    window.addEventListener('keydown', closeOnEscape);
    return () => {
      window.removeEventListener('click', close);
      window.removeEventListener('scroll', close);
      window.removeEventListener('keydown', closeOnEscape);
    };
  }, [cardMenu]);

  const previewItems = useMemo(
    () =>
      photos.map((photo) => ({
        contentType: photo.contentType,
        downloadFilename: downloadFilename(event?.title ?? 'fluffcatch', photo),
        downloadUrl: photoURL(photo),
        previewSrc: isVideoPhoto(photo) ? undefined : photoURL(photo, 'thumbnail'),
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

  function handleEventSaved() {
    setEditorOpen(false);
    setMessage('兽聚已更新。');
    void Promise.all([refreshEventOnly(), refreshPhotoSections()]);
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

  function openCardMenu(photo: Photo, point: { x: number; y: number }) {
    if (!authenticated) return;
    setCardMenu({
      mouseX: point.x + 2,
      mouseY: point.y - 6,
      photo
    });
  }

  function closeCardMenu() {
    setCardMenu(null);
  }

  function handleCardContextMenu(event: React.MouseEvent, photo: Photo) {
    if (!authenticated) return;
    event.preventDefault();
    event.stopPropagation();
    openCardMenu(photo, { x: event.clientX, y: event.clientY });
  }

  function handleCardPointerDown(event: React.PointerEvent, photo: Photo) {
    if (!authenticated || event.pointerType !== 'touch') return;
    clearLongPressTimer();
    longPressPointRef.current = { x: event.clientX, y: event.clientY };
    longPressTimerRef.current = window.setTimeout(() => {
      suppressNextCardClickRef.current = true;
      openCardMenu(photo, longPressPointRef.current || { x: event.clientX, y: event.clientY });
    }, 520);
  }

  function handleCardPointerMove(event: React.PointerEvent) {
    if (!longPressPointRef.current) return;
    const distance = Math.hypot(event.clientX - longPressPointRef.current.x, event.clientY - longPressPointRef.current.y);
    if (distance > 12) {
      clearLongPressTimer();
    }
  }

  function handleCardPointerEnd() {
    clearLongPressTimer();
  }

  function clearLongPressTimer() {
    if (longPressTimerRef.current !== null) {
      window.clearTimeout(longPressTimerRef.current);
      longPressTimerRef.current = null;
    }
    longPressPointRef.current = null;
  }

  function handleCardActionClick(event: React.MouseEvent, photo: Photo, index: number) {
    if (suppressNextCardClickRef.current) {
      suppressNextCardClickRef.current = false;
      event.preventDefault();
      event.stopPropagation();
      return;
    }
    if (manageMode && authenticated) {
      togglePhotoSelection(photo.id);
      return;
    }
    openPhoto(photo, index);
  }

  function handleCardMenuDownload() {
    const photo = cardMenu?.photo;
    closeCardMenu();
    if (photo) void handleSingleDownload(photo);
  }

  function handleCardMenuSettings() {
    const photo = cardMenu?.photo;
    closeCardMenu();
    if (photo) startEditPhoto(photo);
  }

  function handleCardMenuDelete() {
    const photo = cardMenu?.photo;
    closeCardMenu();
    if (photo) requestDeletePhoto(photo);
  }

  function handleCardMenuProperties() {
    const photo = cardMenu?.photo;
    closeCardMenu();
    if (photo) setPropertyPhoto(photo);
  }

  function handleCardMenuQuickTags() {
    const photo = cardMenu?.photo;
    closeCardMenu();
    if (!photo) return;
    setQuickTagPhoto(photo);
    setQuickTags((photo.tags ?? []).map((tag) => tag.name));
    setQuickTagInput('');
  }

  function applyTagFilter(tagName: string) {
    setFiltersOpen(true);
    const nextFilters = { ...galleryFilters, tag: tagName };
    setGalleryFilterDraft(nextFilters);
    setGalleryFilters(nextFilters);
  }

  function applyGalleryFilters() {
    setPublicPage(1);
    setPrivatePage(1);
    setSelectedIds([]);
    setGalleryFilters({
      tag: galleryFilterDraft.tag?.trim() || undefined,
      photographer: galleryFilterDraft.photographer?.trim() || undefined,
      mediaType: galleryFilterDraft.mediaType || 'all',
      sort: galleryFilterDraft.sort || 'latest'
    });
  }

  function clearGalleryFilters() {
    const nextFilters: GalleryFilters = { mediaType: 'all', sort: 'latest' };
    setGalleryFilterDraft(nextFilters);
    setGalleryFilters(nextFilters);
  }

  function addQuickTag(value = quickTagInput) {
    const normalized = normalizeTagInput(value);
    if (!normalized) return;
    setQuickTags((prev) => (prev.includes(normalized) ? prev : [...prev, normalized]));
    setQuickTagInput('');
  }

  function removeQuickTag(tagName: string) {
    setQuickTags((prev) => prev.filter((tag) => tag !== tagName));
  }

  async function saveQuickTags() {
    if (!quickTagPhoto) return;
    setQuickTagSaving(true);
    setError('');
    try {
      const result = await updatePhoto(quickTagPhoto.id, {
        photographerName: quickTagPhoto.photographerName || '',
        tags: quickTags,
        takenAt: toDatetimeLocal(quickTagPhoto.takenAt || ''),
        visibility: quickTagPhoto.visibility
      });
      setQuickTagPhoto(null);
      setMessage('标签已更新。');
      applyUpdatedPhoto(result.photo);
    } catch (err) {
      setError(err instanceof Error ? err.message : '保存标签失败');
    } finally {
      setQuickTagSaving(false);
    }
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
      takenAt: toDatetimeLocal(photo.takenAt || ''),
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
        takenAt: photoForm.takenAt,
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
      <Grid key={photo.id} size={{ xs: 6, sm: 6, md: 4, lg: 3 }}>
        <Card
          onContextMenu={(contextEvent) => handleCardContextMenu(contextEvent, photo)}
          onPointerCancel={handleCardPointerEnd}
          onPointerDown={(pointerEvent) => handleCardPointerDown(pointerEvent, photo)}
          onPointerLeave={handleCardPointerEnd}
          onPointerMove={handleCardPointerMove}
          onPointerUp={handleCardPointerEnd}
          sx={{ borderRadius: { xs: 1.5, sm: 3 }, height: '100%', overflow: 'hidden', position: 'relative', border: manageMode && selectedIds.includes(photo.id) ? '2px solid' : undefined, borderColor: manageMode && selectedIds.includes(photo.id) ? 'primary.main' : undefined }}
        >
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
          <CardActionArea onClick={(clickEvent) => handleCardActionClick(clickEvent, photo, index)} sx={{ display: 'block' }}>
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
              {!manageMode && !locked && (
                <IconButton
                  aria-label="下载原文件"
                  disabled={downloadId === photo.id}
                  onClick={(clickEvent) => { clickEvent.preventDefault(); clickEvent.stopPropagation(); void handleSingleDownload(photo); }}
                  size="small"
                  sx={{
                    bgcolor: 'rgba(15,23,42,0.72)',
                    bottom: 8,
                    color: 'white',
                    display: { xs: 'inline-flex', sm: 'none' },
                    position: 'absolute',
                    right: 8,
                    zIndex: 3,
                    '&:hover': { bgcolor: 'rgba(15,23,42,0.86)' },
                    '&.Mui-disabled': { bgcolor: 'rgba(15,23,42,0.48)', color: 'rgba(255,255,255,0.7)' }
                  }}
                >
                  {downloadId === photo.id ? <CircularProgress color="inherit" size={18} /> : <CloudDownload fontSize="small" />}
                </IconButton>
              )}
            </Box>
          </CardActionArea>
          <CardContent sx={{ p: { xs: 1, sm: 1.75 } }}>
            <Stack sx={{ gap: { xs: 0.75, sm: 1.25 } }}>
              <Stack direction="row" sx={{ alignItems: 'center', gap: 1, minWidth: 0 }}>
                <CameraAlt color="primary" fontSize="small" sx={{ display: { xs: 'none', sm: 'inline-flex' } }} />
                <Typography noWrap sx={{ flex: 1, fontWeight: 700 }}>
                  {photo.photographerName || '匿名投稿'}
                </Typography>
                {!manageMode && photo.visibility === 'public' && (
                  <Button
                    color={photo.liked ? 'error' : 'inherit'}
                    onClick={() => void handleLike(photo)}
                    size="small"
                    startIcon={photo.liked ? <Favorite /> : <FavoriteBorder />}
                    sx={{ minWidth: 0, px: { xs: 0.75, sm: 1 } }}
                  >
                    {photo.likeCount || 0}
                  </Button>
                )}
              </Stack>
              {!manageMode && !locked && (
                <Button
                  aria-label="下载原文件"
                  disabled={downloadId === photo.id}
                  onClick={(clickEvent) => { clickEvent.stopPropagation(); void handleSingleDownload(photo); }}
                  size="small"
                  startIcon={downloadId === photo.id ? <CircularProgress size={16} /> : <CloudDownload />}
                  sx={{
                    display: { xs: 'none', sm: 'inline-flex' }
                  }}
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
                <Stack direction={{ xs: 'column', sm: 'row' }} sx={{ gap: 0.5 }}>
                  <Button onClick={() => startEditPhoto(photo)} size="small" startIcon={<Edit />} sx={{ justifyContent: 'flex-start' }}>属性</Button>
                  <Button color="error" onClick={() => requestDeletePhoto(photo)} size="small" startIcon={<Delete />} sx={{ justifyContent: 'flex-start' }}>删除</Button>
                </Stack>
              )}
              <Stack sx={{ color: 'text.secondary', gap: { xs: 0.35, sm: 0.75 } }}>
                <PhotoMeta icon={<Storage fontSize="inherit" />} text={`${formatContentType(photo.contentType)} · ${formatBytes(photo.sizeBytes)}`} />
                <PhotoMeta icon={<CalendarMonth fontSize="inherit" />} text={formatPhotoTime(photo.takenAt || photo.createdAt)} />
              </Stack>
              {!!(photo.tags ?? []).length && (
                <Stack direction="row" sx={{ flexWrap: 'wrap', gap: { xs: 0.4, sm: 0.75 } }}>
                  {(photo.tags ?? []).slice(0, 2).map((tag) => (
                    <Chip
                      clickable
                      icon={<LocalOffer sx={{ display: { xs: 'none', sm: 'inline-flex' } }} />}
                      key={tag.id}
                      label={tag.name}
                      onClick={(chipEvent) => {
                        chipEvent.preventDefault();
                        chipEvent.stopPropagation();
                        applyTagFilter(tag.name);
                      }}
                      size="small"
                      sx={{
                        '& .MuiChip-label': { px: { xs: 0.75, sm: 1 } },
                        fontSize: { xs: 11, sm: 13 },
                        height: { xs: 22, sm: 24 },
                        maxWidth: '100%'
                      }}
                    />
                  ))}
                  {(photo.tags ?? []).length > 2 && (
                    <Chip
                      label={`+${photo.tags.length - 2}`}
                      size="small"
                      sx={{ '& .MuiChip-label': { px: { xs: 0.75, sm: 1 } }, fontSize: { xs: 11, sm: 13 }, height: { xs: 22, sm: 24 } }}
                    />
                  )}
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
                  <Button onClick={() => setLinkDialogOpen(true)} startIcon={<LinkIcon />} sx={{ width: 'fit-content' }} variant="outlined">
                    限时投稿链接
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
                const first = selectedIds[0];
                if (!event || !first) return;
                setEventCoverFromPhoto(event.id, first)
                  .then(() => { setMessage('已使用选中返图设置封面。'); void refreshEventOnly(); })
                  .catch((err: unknown) => setError(err instanceof Error ? err.message : '设置封面失败'));
              }}
              size="small"
              variant="outlined"
            >
              设为封面
            </Button>
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

      <Paper sx={{ borderRadius: 3, p: { xs: 1.5, sm: 2 } }} variant="outlined">
        <Stack direction="row" sx={{ alignItems: 'center', justifyContent: 'space-between', gap: 1 }}>
          <Box>
            <Typography sx={{ fontWeight: 800 }}>画廊筛选</Typography>
            <Typography color="text.secondary" variant="body2">筛选只刷新返图区域。</Typography>
          </Box>
          <Button onClick={() => setFiltersOpen((prev) => !prev)} variant="outlined">
            {filtersOpen ? '收起' : '展开'}
          </Button>
        </Stack>
        <Collapse in={filtersOpen}>
          <Grid container spacing={1.5} sx={{ mt: 1.5 }}>
            <Grid size={{ xs: 12, sm: 6, md: 3 }}>
              <TextField
                fullWidth
                label="标签筛选"
                onChange={(e) => setGalleryFilterDraft((prev) => ({ ...prev, tag: e.target.value }))}
                onKeyDown={(keyEvent) => {
                  if (keyEvent.key === 'Enter') {
                    keyEvent.preventDefault();
                    applyGalleryFilters();
                  }
                }}
                placeholder="#舞台"
                size="small"
                value={galleryFilterDraft.tag || ''}
              />
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 3 }}>
              <TextField
                fullWidth
                label="摄影师筛选"
                onChange={(e) => setGalleryFilterDraft((prev) => ({ ...prev, photographer: e.target.value }))}
                onKeyDown={(keyEvent) => {
                  if (keyEvent.key === 'Enter') {
                    keyEvent.preventDefault();
                    applyGalleryFilters();
                  }
                }}
                size="small"
                value={galleryFilterDraft.photographer || ''}
              />
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 2 }}>
              <FormControl fullWidth size="small">
                <InputLabel>媒体类型</InputLabel>
                <Select label="媒体类型" onChange={(e: SelectChangeEvent) => setGalleryFilterDraft((prev) => ({ ...prev, mediaType: e.target.value as GalleryFilters['mediaType'] }))} value={galleryFilterDraft.mediaType || 'all'}>
                  <MenuItem value="all">全部</MenuItem>
                  <MenuItem value="image">图片</MenuItem>
                  <MenuItem value="video">视频</MenuItem>
                </Select>
              </FormControl>
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 2 }}>
              <FormControl fullWidth size="small">
                <InputLabel>排序</InputLabel>
                <Select label="排序" onChange={(e: SelectChangeEvent) => setGalleryFilterDraft((prev) => ({ ...prev, sort: e.target.value as GalleryFilters['sort'] }))} value={galleryFilterDraft.sort || 'latest'}>
                  <MenuItem value="latest">最新入库</MenuItem>
                  <MenuItem value="oldest">最早入库</MenuItem>
                  <MenuItem value="taken_desc">拍摄时间新</MenuItem>
                  <MenuItem value="taken_asc">拍摄时间旧</MenuItem>
                  <MenuItem value="likes">最多喜欢</MenuItem>
                </Select>
              </FormControl>
            </Grid>
            <Grid size={{ xs: 6, md: 1 }}>
              <Button fullWidth onClick={applyGalleryFilters} sx={{ height: '100%' }} variant="contained">
                应用
              </Button>
            </Grid>
            <Grid size={{ xs: 6, md: 1 }}>
              <Button fullWidth onClick={clearGalleryFilters} sx={{ height: '100%' }}>
                清空筛选
              </Button>
            </Grid>
          </Grid>
        </Collapse>
      </Paper>

      <PhotoSection title="公开返图" subtitle={`${publicTotal} 个公开媒体文件`}>
        <PhotoSectionBody loading={publicLoading}>
          {publicPhotos.length ? (
            <Grid container spacing={{ xs: 1, sm: 2 }}>
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
            <Grid container spacing={{ xs: 1, sm: 2 }}>
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
      <SubmissionDialog event={event} onClose={closeSubmissionDialog} onUploaded={handleSubmissionUploaded} open={submitOpen} />
      <SubmissionReviewDialog event={event} onChanged={refreshPhotoSections} onClose={() => setReviewOpen(false)} open={reviewOpen} />
      <SubmissionLinkDialog event={event} onClose={() => setLinkDialogOpen(false)} open={linkDialogOpen} />
      <EventEditorDialog event={event} mode="edit" onClose={() => setEditorOpen(false)} onSaved={handleEventSaved} open={editorOpen} />

      {cardMenu && (
        <Paper
          elevation={8}
          onClick={(clickEvent) => clickEvent.stopPropagation()}
          onContextMenu={(contextEvent) => contextEvent.preventDefault()}
          sx={{
            border: '1px solid',
            borderColor: 'divider',
            left: Math.min(cardMenu.mouseX, Math.max(8, window.innerWidth - 228)),
            overflow: 'hidden',
            position: 'fixed',
            py: 0.5,
            top: Math.min(cardMenu.mouseY, Math.max(8, window.innerHeight - 252)),
            width: 220,
            zIndex: (theme) => theme.zIndex.modal
          }}
        >
          <ContextMenuButton disabled={downloadId === cardMenu.photo.id} icon={downloadId === cardMenu.photo.id ? <CircularProgress size={18} /> : <CloudDownload fontSize="small" />} label="下载" onClick={handleCardMenuDownload} />
          <ContextMenuButton icon={<SettingsIcon fontSize="small" />} label="设置" onClick={handleCardMenuSettings} />
          <ContextMenuButton icon={<LocalOffer fontSize="small" />} label="修改标签" onClick={handleCardMenuQuickTags} />
          <ContextMenuButton icon={<InfoOutlined fontSize="small" />} label="属性" onClick={handleCardMenuProperties} />
          <Divider />
          <ContextMenuButton color="error.main" icon={<Delete fontSize="small" />} label="删除" onClick={handleCardMenuDelete} />
        </Paper>
      )}

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
              label="拍摄时间"
              onChange={(e) => setPhotoForm((prev) => ({ ...prev, takenAt: e.target.value }))}
              slotProps={{ inputLabel: { shrink: true } }}
              type="datetime-local"
              value={photoForm.takenAt}
            />
              {editingPhoto?.exif && formatExifRows(editingPhoto.exif).length > 0 && (
                <Paper sx={{ p: 1.5 }} variant="outlined">
                  <Typography sx={{ fontWeight: 800, mb: 0.75 }} variant="body2">EXIF</Typography>
                  <Stack sx={{ gap: 0.75 }}>
                    {formatExifRows(editingPhoto.exif).map((row) => (
                      <PropertyRow key={row.label} label={row.label} value={row.value} wrap />
                    ))}
                  </Stack>
                </Paper>
              )}
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setEditingPhoto(null)}>取消</Button>
          <Button form="photo-property-form" type="submit" variant="contained">保存</Button>
        </DialogActions>
      </Dialog>

      <Dialog fullWidth maxWidth="sm" onClose={() => setPropertyPhoto(null)} open={Boolean(propertyPhoto)}>
        <DialogTitle>媒体属性 #{propertyPhoto?.id}</DialogTitle>
        <DialogContent dividers>
          {propertyPhoto && (
            <Stack sx={{ gap: 2 }}>
              <Box
                sx={{
                  aspectRatio: '16 / 9',
                  bgcolor: 'action.hover',
                  borderRadius: 2,
                  overflow: 'hidden',
                  position: 'relative'
                }}
              >
                {isVideoPhoto(propertyPhoto) ? (
                  <Box
                    component="video"
                    controls
                    preload="metadata"
                    src={photoURL(propertyPhoto)}
                    sx={{ bgcolor: 'common.black', display: 'block', height: '100%', objectFit: 'contain', width: '100%' }}
                  />
                ) : (
                  <Box
                    alt={`媒体 #${propertyPhoto.id}`}
                    component="img"
                    src={photoURL(propertyPhoto, 'thumbnail')}
                    sx={{ display: 'block', height: '100%', objectFit: 'contain', width: '100%' }}
                  />
                )}
              </Box>
              <Paper sx={{ p: 1.5 }} variant="outlined">
                <Stack sx={{ gap: 1 }}>
                  <PropertyRow label="摄影师" value={propertyPhoto.photographerName || '匿名投稿'} />
                  <PropertyRow label="可见性" value={propertyPhoto.visibility === 'public' ? '公开' : '私密'} />
                  <PropertyRow label="媒体类型" value={formatContentType(propertyPhoto.contentType)} />
                  <PropertyRow label="文件大小" value={formatBytes(propertyPhoto.sizeBytes)} />
                  <PropertyRow label="拍摄时间" value={formatDateTime(propertyPhoto.takenAt)} />
                  <PropertyRow label="入库时间" value={formatDateTime(propertyPhoto.createdAt)} />
                  <PropertyRow label="更新时间" value={formatDateTime(propertyPhoto.updatedAt)} />
                  <PropertyRow label="喜欢数" value={`${propertyPhoto.likeCount || 0}`} />
                  <PropertyRow label="存储策略" value={propertyPhoto.storagePolicyId || '未知'} />
                  <PropertyRow label="对象键" value={propertyPhoto.objectKey || '未知'} wrap />
                  <PropertyRow label="内容哈希" value={propertyPhoto.contentHash || '未知'} wrap />
                </Stack>
              </Paper>
              {propertyPhoto.exif && formatExifRows(propertyPhoto.exif).length > 0 && (
                <Paper sx={{ p: 1.5 }} variant="outlined">
                  <Typography sx={{ fontWeight: 800, mb: 1 }} variant="body2">EXIF</Typography>
                  <Stack sx={{ gap: 0.75 }}>
                    {formatExifRows(propertyPhoto.exif).map((row) => (
                      <PropertyRow key={row.label} label={row.label} value={row.value} wrap />
                    ))}
                  </Stack>
                </Paper>
              )}
            </Stack>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setPropertyPhoto(null)}>关闭</Button>
          {propertyPhoto && (
            <Button
              onClick={() => {
                startEditPhoto(propertyPhoto);
                setPropertyPhoto(null);
              }}
              startIcon={<SettingsIcon />}
              variant="contained"
            >
              设置
            </Button>
          )}
        </DialogActions>
      </Dialog>

      <Dialog fullWidth maxWidth="xs" onClose={() => setQuickTagPhoto(null)} open={Boolean(quickTagPhoto)}>
        <DialogTitle>修改标签</DialogTitle>
        <DialogContent dividers>
          <Stack sx={{ gap: 2, pt: 0.5 }}>
            <Typography color="text.secondary" variant="body2">
              输入标签后按回车添加，点击标签旁边的 x 可以删除。
            </Typography>
            <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 0.75 }}>
              {quickTags.map((tagName) => (
                <Chip key={tagName} label={tagName} onDelete={() => removeQuickTag(tagName)} />
              ))}
              {!quickTags.length && (
                <Typography color="text.secondary" variant="body2">还没有标签。</Typography>
              )}
            </Stack>
            <TextField
              autoFocus
              fullWidth
              label="添加标签"
              onChange={(inputEvent) => setQuickTagInput(inputEvent.target.value)}
              onKeyDown={(keyEvent) => {
                if (keyEvent.key === 'Enter' || keyEvent.key === ',' || keyEvent.key === '，') {
                  keyEvent.preventDefault();
                  addQuickTag();
                }
                if (keyEvent.key === 'Backspace' && !quickTagInput && quickTags.length) {
                  removeQuickTag(quickTags[quickTags.length - 1]);
                }
              }}
              placeholder="例如 #舞台"
              value={quickTagInput}
            />
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button disabled={quickTagSaving} onClick={() => setQuickTagPhoto(null)}>取消</Button>
          <Button disabled={quickTagSaving} onClick={() => void saveQuickTags()} variant="contained">
            {quickTagSaving ? '保存中...' : '保存标签'}
          </Button>
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
        requireCaptcha={deleteConfirm?.type !== 'single'}
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

function ContextMenuButton({ color, disabled = false, icon, label, onClick }: { color?: string; disabled?: boolean; icon: React.ReactNode; label: string; onClick: () => void }) {
  return (
    <Button
      disabled={disabled}
      onClick={onClick}
      sx={{
        borderRadius: 0,
        color,
        justifyContent: 'flex-start',
        px: 1.5,
        py: 0.9,
        textAlign: 'left',
        width: '100%',
        '& .MuiButton-startIcon': { color }
      }}
      startIcon={icon}
    >
      {label}
    </Button>
  );
}

function PropertyRow({ label, value, wrap = false }: { label: string; value: string; wrap?: boolean }) {
  return (
    <Stack direction={{ xs: 'column', sm: 'row' }} sx={{ gap: { xs: 0.25, sm: 1.5 }, minWidth: 0 }}>
      <Typography color="text.secondary" sx={{ flexShrink: 0, width: { sm: 88 } }} variant="body2">
        {label}
      </Typography>
      <Typography
        sx={{
          flex: 1,
          fontFamily: wrap ? 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace' : undefined,
          minWidth: 0,
          overflowWrap: wrap ? 'anywhere' : undefined,
          wordBreak: wrap ? 'break-word' : undefined
        }}
        variant="body2"
      >
        {value}
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

function formatDateTime(value?: string) {
  if (!value) return '时间未知';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '时间未知';
  return new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'short' }).format(date);
}

function formatExifRows(exif?: Record<string, unknown>) {
  if (!exif) return [];
  const rows: { label: string; value: string }[] = [];
  const width = numberExifValue(exif.width);
  const height = numberExifValue(exif.height);
  if (width || height) rows.push({ label: '尺寸', value: width && height ? `${width} x ${height}` : `${width || height}` });
  const device = [stringExifValue(exif.cameraMake), stringExifValue(exif.cameraModel)].filter(Boolean).join(' ');
  if (device) rows.push({ label: '拍摄设备', value: device });
  const lens = [stringExifValue(exif.lensMake), stringExifValue(exif.lensModel)].filter(Boolean).join(' ');
  if (lens) rows.push({ label: '镜头', value: lens });
  const takenAt = stringExifValue(exif.takenAt);
  if (takenAt) rows.push({ label: '拍摄时间', value: formatDateTime(takenAt) });
  const exposureTime = stringExifValue(exif.exposureTime);
  if (exposureTime) rows.push({ label: '快门', value: exposureTime });
  const fNumber = stringExifValue(exif.fNumber);
  if (fNumber) rows.push({ label: '光圈', value: fNumber });
  const iso = numberExifValue(exif.iso);
  if (iso) rows.push({ label: 'ISO', value: `${iso}` });
  const exposureBias = stringExifValue(exif.exposureBias);
  if (exposureBias) rows.push({ label: '曝光补偿', value: exposureBias });
  const focalLength = stringExifValue(exif.focalLength);
  if (focalLength) rows.push({ label: '焦距', value: focalLength });
  return rows;
}

function stringExifValue(value: unknown) {
  return typeof value === 'string' ? value.trim() : '';
}

function numberExifValue(value: unknown) {
  if (typeof value === 'number' && Number.isFinite(value)) return value;
  if (typeof value === 'string') {
    const parsed = Number(value);
    if (Number.isFinite(parsed)) return parsed;
  }
  return 0;
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
    case 'image/heic':
      return 'heic';
    case 'image/heif':
      return 'heif';
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

function normalizeTagInput(value: string) {
  return value.trim().replace(/^[#＃]+/, '').trim();
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

function toDatetimeLocal(value?: string) {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  const offsetDate = new Date(date.getTime() - date.getTimezoneOffset() * 60000);
  return offsetDate.toISOString().slice(0, 16);
}
