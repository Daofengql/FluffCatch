import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  CardMedia,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  Grid,
  InputLabel,
  MenuItem,
  Paper,
  Select,
  Stack,
  Switch,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Typography
} from '@mui/material';
import type { SelectChangeEvent } from '@mui/material/Select';
import { type ChangeEvent, type FormEvent, useEffect, useMemo, useState } from 'react';
import { useLocation } from 'react-router-dom';
import { deleteEvent, deletePhoto, getEvents, getPhotos, saveEvent, updatePhoto, uploadEventCover, type EventCard, type Photo, type PhotoPage } from '../../api/client';
import { PageHeader } from '../../components/common/PageHeader';
import { ImagePreviewDialog } from '../../components/ImagePreviewDialog';

type EditorMode = 'create' | 'edit' | null;
type ViewMode = 'card' | 'list';

const emptyEvent: Partial<EventCard> = {
  title: '',
  description: '',
  location: '',
  startTime: '',
  endTime: '',
  isPublic: true,
  submissionEnabled: true
};

export function AdminEventsPage() {
  const location = useLocation();
  const [events, setEvents] = useState<EventCard[]>([]);
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [mode, setMode] = useState<EditorMode>(null);
  const [error, setError] = useState('');
  const [message, setMessage] = useState('');
  const [coverFile, setCoverFile] = useState<File | null>(null);
  const [coverPreviewUrl, setCoverPreviewUrl] = useState('');
  const [saving, setSaving] = useState(false);
  const [photosEvent, setPhotosEvent] = useState<EventCard | null>(null);
  const [photoPage, setPhotoPage] = useState<PhotoPage>({ page: 1, pageSize: 12, photos: [], total: 0, totalPages: 0 });
  const [photoLoading, setPhotoLoading] = useState(false);
  const [previewPhotoIndex, setPreviewPhotoIndex] = useState<number | null>(null);
  const [photoViewMode, setPhotoViewMode] = useState<ViewMode>('card');
  const [editingPhoto, setEditingPhoto] = useState<Photo | null>(null);
  const [photoForm, setPhotoForm] = useState({ accessPassword: '', photographerName: '', tags: '', visibility: 'public' as Photo['visibility'] });

  const selectedEvent = useMemo(() => events.find((event) => event.id === selectedId) ?? null, [events, selectedId]);
  const editorEvent = mode === 'edit' && selectedEvent ? selectedEvent : emptyEvent;
  const editorOpen = mode !== null;
  const activeCoverUrl = coverPreviewUrl || (mode === 'edit' ? selectedEvent?.coverUrl ?? '' : '');
  const photoPreviewItems = useMemo(
    () =>
      photoPage.photos.map((photo) => ({
        src: photo.url,
        subtitle: photo.photographerName ? `摄影师：${photo.photographerName}` : photo.visibility,
        title: `${photosEvent?.title || '图片'} #${photo.id}`
      })),
    [photoPage.photos, photosEvent?.title]
  );

  function refresh(nextSelectedId = selectedId) {
    getEvents(true)
      .then((items) => {
        setEvents(items);
        if (nextSelectedId && items.some((item) => item.id === nextSelectedId)) {
          setSelectedId(nextSelectedId);
        } else if (mode === 'edit') {
          setSelectedId(null);
          setMode(null);
        }
      })
      .catch((err) => setError(err instanceof Error ? err.message : '加载失败'));
  }

  useEffect(() => {
    refresh(null);
  }, [location.key]);

  useEffect(() => {
    return () => {
      if (coverPreviewUrl) {
        URL.revokeObjectURL(coverPreviewUrl);
      }
    };
  }, [coverPreviewUrl]);

  function clearCoverSelection() {
    if (coverPreviewUrl) {
      URL.revokeObjectURL(coverPreviewUrl);
    }
    setCoverFile(null);
    setCoverPreviewUrl('');
  }

  function closeEditor() {
    setMode(null);
    clearCoverSelection();
  }

  function startCreate() {
    setMode('create');
    setSelectedId(null);
    clearCoverSelection();
    setError('');
    setMessage('');
  }

  function startEdit(event: EventCard) {
    setMode('edit');
    setSelectedId(event.id);
    clearCoverSelection();
    setError('');
    setMessage('');
  }

  function openPhotos(event: EventCard, page = 1) {
    setPhotosEvent(event);
    setPreviewPhotoIndex(null);
    setPhotoLoading(true);
    getPhotos(event.id, true, page, 12)
      .then(setPhotoPage)
      .catch((err) => setError(err instanceof Error ? err.message : '加载图片失败'))
      .finally(() => setPhotoLoading(false));
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

  async function handlePhotoFormSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!editingPhoto || !photosEvent) return;
    setError('');
    try {
      await updatePhoto(editingPhoto.id, {
        accessPassword: photoForm.accessPassword,
        photographerName: photoForm.photographerName,
        tags: photoForm.tags.split(/[\s,，]+/).filter(Boolean),
        visibility: photoForm.visibility
      });
      setEditingPhoto(null);
      openPhotos(photosEvent, photoPage.page);
    } catch (err) {
      setError(err instanceof Error ? err.message : '保存图片属性失败');
    }
  }

  async function handleDeletePhoto(photo: Photo) {
    if (!photosEvent) return;
    if (!window.confirm(`确定删除图片 #${photo.id} 吗？这个操作会删除原图和缩略图。`)) return;
    setError('');
    try {
      await deletePhoto(photo.id);
      openPhotos(photosEvent, photoPage.page);
    } catch (err) {
      setError(err instanceof Error ? err.message : '删除图片失败');
    }
  }

  function handleCoverSelect(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0] ?? null;
    if (coverPreviewUrl) {
      URL.revokeObjectURL(coverPreviewUrl);
    }
    setCoverFile(file);
    setCoverPreviewUrl(file ? URL.createObjectURL(file) : '');
    event.target.value = '';
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (saving) return;
    setError('');
    setMessage('');
    setSaving(true);
    const formData = new FormData(event.currentTarget);
    const selectedCoverFile = coverFile;
    const currentMode = mode;

    try {
      const result = await saveEvent({
        id: currentMode === 'edit' ? selectedEvent?.id : undefined,
        title: String(formData.get('title') || ''),
        description: String(formData.get('description') || ''),
        location: String(formData.get('location') || ''),
        startTime: String(formData.get('startTime') || ''),
        endTime: String(formData.get('endTime') || ''),
        isPublic: formData.get('isPublic') === 'on',
        submissionEnabled: formData.get('submissionEnabled') === 'on',
        submissionPassword: String(formData.get('submissionPassword') || '')
      });

      if (selectedCoverFile && selectedCoverFile.size > 0) {
        await uploadEventCover(result.event.id, selectedCoverFile);
      }

      setMessage(currentMode === 'edit' ? '兽聚已更新。' : '兽聚已创建。');
      clearCoverSelection();
      setMode(null);
      setSelectedId(null);
      refresh(result.event.id);
    } catch (err) {
      setError(err instanceof Error ? err.message : '保存失败');
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete() {
    if (!selectedEvent) return;
    if (!window.confirm(`确定删除「${selectedEvent.title}」吗？相关投稿和图片记录会一起删除。`)) {
      return;
    }

    setError('');
    setMessage('');
    try {
      await deleteEvent(selectedEvent.id);
      setMessage('兽聚已删除。');
      setSelectedId(null);
      closeEditor();
      refresh(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : '删除失败');
    }
  }

  return (
    <Stack sx={{ gap: 3 }}>
      <PageHeader
        actions={<Button onClick={startCreate} variant="contained">新建兽聚</Button>}
        subtitle="默认只显示列表；点击新建或编辑后弹出编辑卡片"
        title="兽聚管理"
      />
      {message && <Alert severity="success">{message}</Alert>}
      {error && <Alert severity="error">{error}</Alert>}

      <Grid container spacing={3}>
        <Grid size={{ xs: 12 }}>
          <TableContainer component={Paper}>
            <Table>
              <TableHead>
                <TableRow>
                  <TableCell>标题</TableCell>
                  <TableCell>地点</TableCell>
                  <TableCell>状态</TableCell>
                  <TableCell align="right">操作</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {events.map((event) => (
                  <TableRow hover key={event.id} selected={event.id === selectedId}>
                    <TableCell>
                      <Stack direction="row" sx={{ alignItems: 'center', gap: 1.5 }}>
                        {event.coverUrl && (
                          <Box component="img" src={event.coverUrl} sx={{ borderRadius: 1, height: 44, objectFit: 'cover', width: 72 }} />
                        )}
                        <Box>
                          <Typography sx={{ fontWeight: 800 }}>{event.title}</Typography>
                          <Typography color="text.secondary" variant="caption">
                            {formatDateRange(event.startTime, event.endTime)}
                          </Typography>
                        </Box>
                      </Stack>
                    </TableCell>
                    <TableCell>{event.location || '未填写'}</TableCell>
                    <TableCell>
                      <Stack direction="row" sx={{ gap: 1 }}>
                        <Chip color={event.isPublic ? 'success' : 'default'} label={event.isPublic ? '公开' : '隐藏'} size="small" />
                        <Chip color={event.submissionEnabled ? 'primary' : 'default'} label={event.submissionEnabled ? '可投稿' : '关闭投稿'} size="small" />
                      </Stack>
                    </TableCell>
                    <TableCell align="right">
                      <Stack direction="row" sx={{ gap: 1, justifyContent: 'flex-end' }}>
                        <Button onClick={() => openPhotos(event)} size="small" variant="outlined">
                          图片管理
                        </Button>
                        <Button onClick={() => startEdit(event)} size="small" variant="outlined">
                          编辑
                        </Button>
                      </Stack>
                    </TableCell>
                  </TableRow>
                ))}
                {!events.length && (
                  <TableRow>
                    <TableCell align="center" colSpan={4}>
                      <Typography color="text.secondary" sx={{ py: 4 }}>
                        还没有兽聚，点击右上角新建一个吧。
                      </Typography>
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </TableContainer>
        </Grid>

      </Grid>

      <Dialog fullWidth maxWidth="md" onClose={closeEditor} open={editorOpen}>
        <DialogTitle>{mode === 'edit' ? '编辑兽聚' : '新建兽聚'}</DialogTitle>
        <DialogContent dividers>
          <Card elevation={0} sx={{ border: '1px solid', borderColor: 'divider' }}>
            {activeCoverUrl ? (
              <Box sx={{ position: 'relative' }}>
                <CardMedia component="img" image={activeCoverUrl} sx={{ aspectRatio: '16 / 7', objectFit: 'cover' }} />
                <Chip
                  color={coverFile ? 'primary' : 'default'}
                  label={coverFile ? `已选择：${coverFile.name}` : '当前海报'}
                  size="small"
                  sx={{
                    '& .MuiChip-label': { overflow: 'hidden', textOverflow: 'ellipsis' },
                    bgcolor: coverFile ? undefined : 'rgba(255,255,255,0.9)',
                    bottom: 12,
                    maxWidth: { xs: 220, sm: 360 },
                    position: 'absolute',
                    right: 12
                  }}
                />
                <Button
                  component="label"
                  sx={{
                    backdropFilter: 'blur(8px)',
                    bgcolor: 'rgba(255,255,255,0.92)',
                    bottom: 12,
                    left: 12,
                    position: 'absolute',
                    '&:hover': { bgcolor: 'rgba(255,255,255,1)' }
                  }}
                  variant="outlined"
                >
                  {coverFile ? '重新选择海报' : '更换海报'}
                  <input accept="image/*" hidden onChange={handleCoverSelect} type="file" />
                </Button>
              </Box>
            ) : (
              <Box
                component="label"
                sx={{
                  alignItems: 'center',
                  aspectRatio: '16 / 7',
                  bgcolor: 'grey.100',
                  borderBottom: '1px dashed',
                  borderColor: 'divider',
                  color: 'text.secondary',
                  cursor: 'pointer',
                  display: 'flex',
                  justifyContent: 'center',
                  px: 2,
                  textAlign: 'center',
                  transition: 'background-color 160ms ease, color 160ms ease',
                  '&:hover': { bgcolor: 'grey.200', color: 'primary.main' }
                }}
              >
                <Stack sx={{ alignItems: 'center', gap: 1 }}>
                  <Button component="span" variant="outlined">
                    选择海报缩略图
                  </Button>
                  <Typography variant="body2">点击这里选择图片，选择后会立即预览</Typography>
                </Stack>
                <input accept="image/*" hidden onChange={handleCoverSelect} type="file" />
              </Box>
            )}
            <CardContent>
              <Stack component="form" id="event-editor-form" key={`${mode}-${selectedEvent?.id ?? 'new'}`} onSubmit={handleSubmit} sx={{ gap: 2 }}>
                <TextField defaultValue={editorEvent.title ?? ''} fullWidth label="标题" name="title" required />
                <TextField defaultValue={editorEvent.location ?? ''} fullWidth label="地点" name="location" />
                <TextField defaultValue={editorEvent.description ?? ''} fullWidth label="简介" multiline name="description" rows={3} />
                <Grid container spacing={2}>
                  <Grid size={{ xs: 12, sm: 6 }}>
                    <TextField defaultValue={toDatetimeLocal(editorEvent.startTime)} fullWidth label="开始时间" name="startTime" slotProps={{ inputLabel: { shrink: true } }} type="datetime-local" />
                  </Grid>
                  <Grid size={{ xs: 12, sm: 6 }}>
                    <TextField defaultValue={toDatetimeLocal(editorEvent.endTime)} fullWidth label="结束时间" name="endTime" slotProps={{ inputLabel: { shrink: true } }} type="datetime-local" />
                  </Grid>
                </Grid>
                <TextField fullWidth helperText={mode === 'edit' ? '留空则不修改当前投稿口令' : '可留空，留空表示不需要口令'} label="投稿口令" name="submissionPassword" />

                <Grid container spacing={2}>
                  <Grid size={{ xs: 12, sm: 6 }}>
                    <Stack direction="row" sx={{ alignItems: 'center', justifyContent: 'space-between' }}>
                      <Typography>公开展示</Typography>
                      <Switch defaultChecked={Boolean(editorEvent.isPublic)} name="isPublic" />
                    </Stack>
                  </Grid>
                  <Grid size={{ xs: 12, sm: 6 }}>
                    <Stack direction="row" sx={{ alignItems: 'center', justifyContent: 'space-between' }}>
                      <Typography>允许投稿</Typography>
                      <Switch defaultChecked={editorEvent.submissionEnabled !== false} name="submissionEnabled" />
                    </Stack>
                  </Grid>
                </Grid>

                {coverFile && (
                  <Typography color="text.secondary" variant="caption">
                    海报会在点击「{mode === 'edit' ? '保存修改' : '创建兽聚'}」时上传。
                  </Typography>
                )}
              </Stack>
            </CardContent>
          </Card>
        </DialogContent>
        <DialogActions sx={{ justifyContent: 'space-between', px: 3 }}>
          <Box>
            {mode === 'edit' && (
              <Button color="error" onClick={() => void handleDelete()} variant="outlined">
                删除兽聚
              </Button>
            )}
          </Box>
          <Stack direction="row" sx={{ gap: 1 }}>
            <Button onClick={closeEditor}>取消</Button>
            <Button disabled={saving} form="event-editor-form" type="submit" variant="contained">
              {saving ? '保存中...' : mode === 'edit' ? '保存修改' : '创建兽聚'}
            </Button>
          </Stack>
        </DialogActions>
      </Dialog>

      <Dialog fullWidth maxWidth="lg" onClose={() => setPhotosEvent(null)} open={Boolean(photosEvent)}>
        <DialogTitle>
          <Stack direction="row" sx={{ alignItems: 'center', justifyContent: 'space-between', gap: 2 }}>
            <span>{photosEvent?.title} / 图片管理</span>
            <Stack direction="row" sx={{ gap: 1 }}>
              <Button onClick={() => setPhotoViewMode('card')} size="small" variant={photoViewMode === 'card' ? 'contained' : 'outlined'}>
                卡片
              </Button>
              <Button onClick={() => setPhotoViewMode('list')} size="small" variant={photoViewMode === 'list' ? 'contained' : 'outlined'}>
                列表
              </Button>
            </Stack>
          </Stack>
        </DialogTitle>
        <DialogContent dividers>
          {photoLoading ? (
            <Typography color="text.secondary">图片加载中...</Typography>
          ) : photoViewMode === 'list' ? (
            <TableContainer component={Paper}>
              <Table size="small">
                <TableHead>
                  <TableRow>
                    <TableCell>图片</TableCell>
                    <TableCell>ID</TableCell>
                    <TableCell>摄影师</TableCell>
                    <TableCell>可见性</TableCell>
                    <TableCell>标签</TableCell>
                    <TableCell>Hash</TableCell>
                    <TableCell align="right">操作</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {photoPage.photos.map((photo, index) => (
                    <TableRow hover key={photo.id}>
                      <TableCell>
                        <Box
                          component="img"
                          onClick={() => setPreviewPhotoIndex(index)}
                          src={photo.thumbnailUrl || photo.url}
                          sx={{ borderRadius: 1, cursor: 'zoom-in', height: 56, objectFit: 'cover', width: 76 }}
                        />
                      </TableCell>
                      <TableCell>#{photo.id}</TableCell>
                      <TableCell>{photo.photographerName || '匿名'}</TableCell>
                      <TableCell>{photo.visibility}</TableCell>
                      <TableCell>{(photo.tags ?? []).map((tag) => tag.name).join(' ') || '无'}</TableCell>
                      <TableCell>{photo.contentHash.slice(0, 16)}</TableCell>
                      <TableCell align="right">
                        <Stack direction="row" sx={{ gap: 1, justifyContent: 'flex-end' }}>
                          <Button onClick={() => startEditPhoto(photo)} size="small">属性</Button>
                          <Button color="error" onClick={() => void handleDeletePhoto(photo)} size="small">删除</Button>
                        </Stack>
                      </TableCell>
                    </TableRow>
                  ))}
                  {!photoPage.photos.length && (
                    <TableRow>
                      <TableCell align="center" colSpan={7}>
                        <Typography color="text.secondary" sx={{ py: 4 }}>
                          这个兽聚还没有正式图片。
                        </Typography>
                      </TableCell>
                    </TableRow>
                  )}
                </TableBody>
              </Table>
            </TableContainer>
          ) : (
            <Grid container spacing={2}>
              {photoPage.photos.map((photo, index) => (
                <Grid key={photo.id} size={{ xs: 6, md: 3 }}>
                  <Card>
                    <CardMedia
                      component="img"
                      image={photo.thumbnailUrl || photo.url}
                      onClick={() => setPreviewPhotoIndex(index)}
                      sx={{ aspectRatio: '1 / 1', cursor: 'zoom-in', objectFit: 'cover' }}
                    />
                    <CardContent sx={{ p: 1.5 }}>
                      <Typography noWrap sx={{ fontWeight: 800 }} variant="body2">
                        #{photo.id}
                      </Typography>
                      <Typography color="text.secondary" noWrap variant="caption">
                        {photo.visibility} / {photo.contentHash.slice(0, 10)}
                      </Typography>
                      <Stack direction="row" sx={{ gap: 1, mt: 1 }}>
                        <Button onClick={() => startEditPhoto(photo)} size="small">属性</Button>
                        <Button color="error" onClick={() => void handleDeletePhoto(photo)} size="small">删除</Button>
                      </Stack>
                    </CardContent>
                  </Card>
                </Grid>
              ))}
              {!photoPage.photos.length && (
                <Grid size={{ xs: 12 }}>
                  <Typography color="text.secondary" sx={{ py: 4, textAlign: 'center' }}>
                    这个兽聚还没有正式图片。
                  </Typography>
                </Grid>
              )}
            </Grid>
          )}
        </DialogContent>
        <DialogActions sx={{ justifyContent: 'space-between', px: 3 }}>
          <Typography color="text.secondary" variant="body2">
            共 {photoPage.total} 张，当前第 {photoPage.page} / {photoPage.totalPages || 1} 页
          </Typography>
          <Stack direction="row" sx={{ gap: 1 }}>
            <Button disabled={photoPage.page <= 1 || !photosEvent} onClick={() => photosEvent && openPhotos(photosEvent, photoPage.page - 1)}>
              上一页
            </Button>
            <Button disabled={!photosEvent || photoPage.page >= photoPage.totalPages} onClick={() => photosEvent && openPhotos(photosEvent, photoPage.page + 1)}>
              下一页
            </Button>
            <Button onClick={() => setPhotosEvent(null)}>关闭</Button>
          </Stack>
        </DialogActions>
      </Dialog>

      <Dialog fullWidth maxWidth="sm" onClose={() => setEditingPhoto(null)} open={Boolean(editingPhoto)}>
        <DialogTitle>图片属性 #{editingPhoto?.id}</DialogTitle>
        <DialogContent dividers>
          <Stack component="form" id="photo-editor-form" onSubmit={handlePhotoFormSubmit} sx={{ gap: 2, pt: 1 }}>
            <TextField
              fullWidth
              label="摄影师署名"
              onChange={(event) => setPhotoForm((prev) => ({ ...prev, photographerName: event.target.value }))}
              value={photoForm.photographerName}
            />
            <FormControl fullWidth>
              <InputLabel>可见性</InputLabel>
              <Select
                label="可见性"
                onChange={(event: SelectChangeEvent) => setPhotoForm((prev) => ({ ...prev, visibility: event.target.value as Photo['visibility'] }))}
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
              onChange={(event) => setPhotoForm((prev) => ({ ...prev, accessPassword: event.target.value }))}
              type="password"
              value={photoForm.accessPassword}
            />
            <TextField
              fullWidth
              helperText="空格或逗号分隔，例如 #合照 #舞台"
              label="标签"
              onChange={(event) => setPhotoForm((prev) => ({ ...prev, tags: event.target.value }))}
              value={photoForm.tags}
            />
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setEditingPhoto(null)}>取消</Button>
          <Button form="photo-editor-form" type="submit" variant="contained">保存</Button>
        </DialogActions>
      </Dialog>

      <ImagePreviewDialog
        images={photoPreviewItems}
        index={previewPhotoIndex ?? 0}
        onClose={() => setPreviewPhotoIndex(null)}
        onIndexChange={setPreviewPhotoIndex}
        open={previewPhotoIndex !== null}
      />
    </Stack>
  );
}

function toDatetimeLocal(value?: string) {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  const offsetDate = new Date(date.getTime() - date.getTimezoneOffset() * 60000);
  return offsetDate.toISOString().slice(0, 16);
}

function formatDateRange(start?: string, end?: string) {
  if (!start) return '时间待定';
  const formatter = new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'short' });
  const startText = formatter.format(new Date(start));
  return end ? `${startText} - ${formatter.format(new Date(end))}` : startText;
}
