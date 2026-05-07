import {
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
  Grid,
  Stack,
  Switch,
  TextField,
  Typography
} from '@mui/material';
import { type ChangeEvent, type FormEvent, useEffect, useState } from 'react';
import {
  deleteEvent,
  saveEvent,
  uploadEventCover,
  type EventCard
} from '../api/client';
import { ConfirmDialog } from './ConfirmDialog';
import { CityCascader, type CityValue } from './common/CityCascader';

type EditorMode = 'create' | 'edit';

type EventEditorDialogProps = {
  event?: EventCard | null;
  mode: EditorMode | null;
  onClose: () => void;
  onSaved: (eventId: number) => void;
  open: boolean;
};

const emptyEvent: Partial<EventCard> = {
  title: '',
  description: '',
  location: '',
  provinceCode: '',
  provinceName: '',
  cityCode: '',
  cityName: '',
  startTime: '',
  endTime: '',
  isPublic: true,
  submissionEnabled: true
};

export function EventEditorDialog({ event, mode, onClose, onSaved, open }: EventEditorDialogProps) {
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);
  const [coverFile, setCoverFile] = useState<File | null>(null);
  const [coverPreviewUrl, setCoverPreviewUrl] = useState('');
  const [region, setRegion] = useState<CityValue>({ cityCode: '', cityName: '', provinceCode: '', provinceName: '' });
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);

  const editorEvent = mode === 'edit' && event ? event : emptyEvent;
  const activeCoverUrl = coverPreviewUrl || (mode === 'edit' ? event?.coverUrl ?? '' : '');

  useEffect(() => {
    if (open && mode === 'edit' && event) {
      setRegion({
        cityCode: event.cityCode || '',
        cityName: event.cityName || '',
        provinceCode: event.provinceCode || '',
        provinceName: event.provinceName || ''
      });
    } else if (open && mode === 'create') {
      setRegion({ cityCode: '', cityName: '', provinceCode: '', provinceName: '' });
    }
    if (open) {
      setError('');
    }
  }, [open, mode, event]);

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

  function handleClose() {
    clearCoverSelection();
    onClose();
  }

  function handleCoverSelect(changeEvent: ChangeEvent<HTMLInputElement>) {
    const file = changeEvent.target.files?.[0] ?? null;
    if (coverPreviewUrl) {
      URL.revokeObjectURL(coverPreviewUrl);
    }
    setCoverFile(file);
    setCoverPreviewUrl(file ? URL.createObjectURL(file) : '');
    changeEvent.target.value = '';
  }

  async function handleSubmit(formEvent: FormEvent<HTMLFormElement>) {
    formEvent.preventDefault();
    if (saving) return;
    setError('');
    setSaving(true);
    const formData = new FormData(formEvent.currentTarget);
    const selectedCoverFile = coverFile;
    const currentMode = mode;

    try {
      const result = await saveEvent({
        id: currentMode === 'edit' ? event?.id : undefined,
        title: String(formData.get('title') || ''),
        description: String(formData.get('description') || ''),
        location: String(formData.get('location') || ''),
        provinceCode: region.provinceCode,
        provinceName: region.provinceName,
        cityCode: region.cityCode,
        cityName: region.cityName,
        startTime: String(formData.get('startTime') || ''),
        endTime: String(formData.get('endTime') || ''),
        isPublic: formData.get('isPublic') === 'on',
        submissionEnabled: formData.get('submissionEnabled') === 'on',
        submissionPassword: String(formData.get('submissionPassword') || ''),
        privatePassword: String(formData.get('privatePassword') || '')
      });

      if (selectedCoverFile && selectedCoverFile.size > 0) {
        await uploadEventCover(result.event.id, selectedCoverFile);
      }

      clearCoverSelection();
      onSaved(result.event.id);
    } catch (err) {
      setError(err instanceof Error ? err.message : '保存失败');
    } finally {
      setSaving(false);
    }
  }

  async function handleDeleteConfirm(headers: Record<string, string>) {
    if (!event) return;
    setDeleteConfirmOpen(false);
    setError('');
    try {
      await deleteEvent(event.id, headers);
      handleClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : '删除失败');
    }
  }

  return (
    <>
    <Dialog fullWidth maxWidth="md" onClose={handleClose} open={open}>
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
                bgcolor: 'action.hover',
                borderBottom: '1px dashed',
                borderColor: 'divider',
                color: 'text.secondary',
                cursor: 'pointer',
                display: 'flex',
                justifyContent: 'center',
                px: 2,
                textAlign: 'center',
                transition: 'background-color 160ms ease, color 160ms ease',
                '&:hover': { bgcolor: 'action.selected', color: 'primary.main' }
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
            <Stack component="form" id="event-editor-form" key={`${mode}-${event?.id ?? 'new'}`} onSubmit={handleSubmit} sx={{ gap: 2 }}>
              <TextField defaultValue={editorEvent.title ?? ''} fullWidth label="标题" name="title" required />
              <CityCascader helperText="行政区选择到市一级，用于首页筛选。" onChange={setRegion} value={region} />
              <TextField defaultValue={editorEvent.location ?? ''} fullWidth helperText="例如：会展中心、酒店名、具体场馆；可留空。" label="详细地点" name="location" />
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
              <TextField fullWidth helperText={mode === 'edit' ? '留空则不修改当前私密口令' : '用于解锁这个兽聚中的私密图片'} label="私密图片访问口令" name="privatePassword" />

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
        {error && (
          <Typography color="error" sx={{ mt: 2 }} variant="body2">{error}</Typography>
        )}
      </DialogContent>
      <DialogActions sx={{ justifyContent: 'space-between', px: 3 }}>
        <Box>
          {mode === 'edit' && (
            <Button color="error" onClick={() => setDeleteConfirmOpen(true)} variant="outlined">
              删除兽聚
            </Button>
          )}
        </Box>
        <Stack direction="row" sx={{ gap: 1 }}>
          <Button onClick={handleClose}>取消</Button>
          <Button disabled={saving} form="event-editor-form" type="submit" variant="contained">
            {saving ? '保存中...' : mode === 'edit' ? '保存修改' : '创建兽聚'}
          </Button>
        </Stack>
      </DialogActions>
    </Dialog>

      <ConfirmDialog
        onCancel={() => setDeleteConfirmOpen(false)}
        onConfirm={handleDeleteConfirm}
        open={deleteConfirmOpen}
        subtitle={event ? `确定删除「${event.title}」吗？相关投稿和图片记录会一起删除。` : ''}
        title="删除兽聚"
      />
    </>
  );
}

function toDatetimeLocal(value?: string) {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  const offsetDate = new Date(date.getTime() - date.getTimezoneOffset() * 60000);
  return offsetDate.toISOString().slice(0, 16);
}
