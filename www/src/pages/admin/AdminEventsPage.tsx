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
  Grid,
  Paper,
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
import { type FormEvent, useEffect, useMemo, useState } from 'react';
import { deleteEvent, getEvents, saveEvent, uploadEventCover, type EventCard } from '../../api/client';
import { PageHeader } from '../../components/common/PageHeader';

type EditorMode = 'create' | 'edit' | null;

const emptyEvent: Partial<EventCard> = {
  slug: '',
  title: '',
  description: '',
  location: '',
  startTime: '',
  endTime: '',
  isPublic: true,
  submissionEnabled: true
};

export function AdminEventsPage() {
  const [events, setEvents] = useState<EventCard[]>([]);
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [mode, setMode] = useState<EditorMode>(null);
  const [error, setError] = useState('');
  const [message, setMessage] = useState('');

  const selectedEvent = useMemo(() => events.find((event) => event.id === selectedId) ?? null, [events, selectedId]);
  const editorEvent = mode === 'edit' && selectedEvent ? selectedEvent : emptyEvent;
  const editorOpen = mode !== null;

  function refresh(nextSelectedId = selectedId) {
    getEvents(true)
      .then((items) => {
      setEvents(items);
      if (nextSelectedId && items.some((item) => item.id === nextSelectedId)) {
        setSelectedId(nextSelectedId);
        setMode('edit');
      } else if (mode === 'edit') {
        setSelectedId(null);
        setMode(null);
      }
      })
      .catch((err) => setError(err instanceof Error ? err.message : '加载失败'));
  }

  useEffect(() => {
    refresh(null);
  }, []);

  function startCreate() {
    setMode('create');
    setSelectedId(null);
    setError('');
    setMessage('');
  }

  function startEdit(event: EventCard) {
    setMode('edit');
    setSelectedId(event.id);
    setError('');
    setMessage('');
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError('');
    setMessage('');
    const formData = new FormData(event.currentTarget);
    const cover = formData.get('cover');

    try {
      const result = await saveEvent({
        id: mode === 'edit' ? selectedEvent?.id : undefined,
        slug: String(formData.get('slug') || ''),
        title: String(formData.get('title') || ''),
        description: String(formData.get('description') || ''),
        location: String(formData.get('location') || ''),
        startTime: String(formData.get('startTime') || ''),
        endTime: String(formData.get('endTime') || ''),
        isPublic: formData.get('isPublic') === 'on',
        submissionEnabled: formData.get('submissionEnabled') === 'on',
        submissionPassword: String(formData.get('submissionPassword') || '')
      });

      if (cover instanceof File && cover.size > 0) {
        await uploadEventCover(result.event.id, cover);
      }

      setMessage(mode === 'edit' ? '兽聚已更新。' : '兽聚已创建。');
      event.currentTarget.reset();
      setMode(null);
      setSelectedId(null);
      refresh(result.event.id);
    } catch (err) {
      setError(err instanceof Error ? err.message : '保存失败');
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
      setMode(null);
      refresh(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : '删除失败');
    }
  }

  async function handleCoverUpload(eventId: number, file: File | null) {
    if (!file) return;
    setError('');
    setMessage('');
    try {
      await uploadEventCover(eventId, file);
      setMessage('海报已更新。');
      refresh(eventId);
    } catch (err) {
      setError(err instanceof Error ? err.message : '海报上传失败');
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
                  <TableCell>URL 标识</TableCell>
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
                    <TableCell>{event.slug}</TableCell>
                    <TableCell>{event.location || '未填写'}</TableCell>
                    <TableCell>
                      <Stack direction="row" sx={{ gap: 1 }}>
                        <Chip color={event.isPublic ? 'success' : 'default'} label={event.isPublic ? '公开' : '隐藏'} size="small" />
                        <Chip color={event.submissionEnabled ? 'primary' : 'default'} label={event.submissionEnabled ? '可投稿' : '关闭投稿'} size="small" />
                      </Stack>
                    </TableCell>
                    <TableCell align="right">
                      <Button onClick={() => startEdit(event)} size="small" variant="outlined">
                        编辑
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
                {!events.length && (
                  <TableRow>
                    <TableCell align="center" colSpan={5}>
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

      <Dialog fullWidth maxWidth="md" onClose={() => setMode(null)} open={editorOpen}>
        <DialogTitle>{mode === 'edit' ? '编辑兽聚' : '新建兽聚'}</DialogTitle>
        <DialogContent dividers>
          <Card elevation={0} sx={{ border: '1px solid', borderColor: 'divider' }}>
            {mode === 'edit' && selectedEvent?.coverUrl && (
              <CardMedia component="img" image={selectedEvent.coverUrl} sx={{ aspectRatio: '16 / 7', objectFit: 'cover' }} />
            )}
            <CardContent>
              <Stack component="form" id="event-editor-form" key={`${mode}-${selectedEvent?.id ?? 'new'}`} onSubmit={handleSubmit} sx={{ gap: 2 }}>
                <Box>
                  <Typography color="text.secondary" variant="body2">
                    URL 标识可留空；重复时后端会自动追加后缀。
                  </Typography>
                </Box>

                <TextField defaultValue={editorEvent.slug ?? ''} fullWidth helperText="用于公开链接和内部标识" label="URL 标识" name="slug" />
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

                <Button component="label" variant="outlined">
                  {mode === 'edit' ? '选择新海报缩略图' : '选择海报缩略图'}
                  <input accept="image/*" hidden name="cover" type="file" />
                </Button>

                {mode === 'edit' && selectedEvent && (
                  <Button component="label" variant="outlined">
                    仅更换海报
                    <input
                      accept="image/*"
                      hidden
                      type="file"
                      onChange={(changeEvent) => {
                        const file = changeEvent.target.files?.[0] ?? null;
                        void handleCoverUpload(selectedEvent.id, file);
                        changeEvent.target.value = '';
                      }}
                    />
                  </Button>
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
            <Button onClick={() => setMode(null)}>取消</Button>
            <Button form="event-editor-form" type="submit" variant="contained">
              {mode === 'edit' ? '保存修改' : '创建兽聚'}
            </Button>
          </Stack>
        </DialogActions>
      </Dialog>
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
