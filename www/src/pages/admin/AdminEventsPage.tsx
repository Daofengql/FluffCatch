import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  Grid,
  Paper,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography
} from '@mui/material';
import { useEffect, useState } from 'react';
import { useLocation } from 'react-router-dom';
import {
  deleteEvent,
  getAdminDashboard,
  getEvents,
  type EventCard
} from '../../api/client';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { PageHeader } from '../../components/common/PageHeader';
import { EventEditorDialog } from '../../components/EventEditorDialog';
import { formatEventLocation } from '../../utils/eventLocation';

type EditorMode = 'create' | 'edit' | null;
type DeleteTarget = { event: EventCard } | null;

export function AdminEventsPage() {
  const location = useLocation();
  const [events, setEvents] = useState<EventCard[]>([]);
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [mode, setMode] = useState<EditorMode>(null);
  const [error, setError] = useState('');
  const [message, setMessage] = useState('');
  const [stats, setStats] = useState<Record<string, number>>({});
  const [deleteTarget, setDeleteTarget] = useState<DeleteTarget>(null);

  const selectedEvent = events.find((event) => event.id === selectedId) ?? null;

  function refresh(nextSelectedId = selectedId) {
    getAdminDashboard().then((payload) => setStats(payload.stats)).catch(() => setStats({}));
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

  function handleEditorClose() {
    setMode(null);
    setSelectedId(null);
  }

  function handleEventSaved(eventId: number) {
    setMessage('兽聚已保存。');
    setMode(null);
    setSelectedId(null);
    refresh(eventId);
  }

  async function handleDeleteConfirm(headers: Record<string, string>) {
    if (!deleteTarget) return;
    setError('');
    setDeleteTarget(null);
    try {
      await deleteEvent(deleteTarget.event.id, headers);
      setMessage('兽聚已删除。');
      refresh(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : '删除失败');
    }
  }

  return (
    <Stack sx={{ gap: 3 }}>
      <PageHeader
        actions={<Button onClick={startCreate} variant="contained">新建兽聚</Button>}
        subtitle="后台负责系统级总览与兽聚列表；单个兽聚的内容管理可以进入前台页面完成"
        title="兽聚管理"
      />
      {message && <Alert onClose={() => setMessage('')} severity="success">{message}</Alert>}
      {error && <Alert onClose={() => setError('')} severity="error">{error}</Alert>}

      <Grid container spacing={2}>
        {[
          ['兽聚数量', stats.events || 0],
          ['正式图片', stats.photos || 0],
          ['待审投稿', stats.pendingSubmissions || 0],
          ['图片容量', formatBytes(stats.photoBytes || 0)]
        ].map(([label, value]) => (
          <Grid key={label} size={{ xs: 12, sm: 6, md: 3 }}>
            <Card>
              <CardContent>
                <Typography color="text.secondary">{label}</Typography>
                <Typography sx={{ fontWeight: 800, mt: 1 }} variant="h4">
                  {value}
                </Typography>
              </CardContent>
            </Card>
          </Grid>
        ))}
      </Grid>

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
                <TableCell>{formatEventLocation(event)}</TableCell>
                <TableCell>
                  <Stack direction="row" sx={{ gap: 1 }}>
                    <Chip color={event.isPublic ? 'success' : 'default'} label={event.isPublic ? '公开' : '隐藏'} size="small" />
                    <Chip color={event.submissionEnabled ? 'primary' : 'default'} label={event.submissionEnabled ? '可投稿' : '关闭投稿'} size="small" />
                  </Stack>
                </TableCell>
                <TableCell align="right">
                  <Stack direction="row" sx={{ gap: 1, justifyContent: 'flex-end' }}>
                    <Button onClick={() => startEdit(event)} size="small" variant="outlined">
                      编辑
                    </Button>
                    <Button onClick={() => setDeleteTarget({ event })} size="small" variant="outlined" color="error">
                      删除
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

      <EventEditorDialog
        event={selectedEvent}
        mode={mode}
        onClose={handleEditorClose}
        onSaved={handleEventSaved}
        open={mode !== null}
      />

      <ConfirmDialog
        onCancel={() => setDeleteTarget(null)}
        onConfirm={handleDeleteConfirm}
        open={Boolean(deleteTarget)}
        subtitle={deleteTarget ? `确定删除「${deleteTarget.event.title}」吗？相关投稿和图片记录会一起删除。` : ''}
        title="删除兽聚"
      />
    </Stack>
  );
}

function formatDateRange(start?: string, end?: string) {
  if (!start) return '时间待定';
  const formatter = new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'short' });
  const startText = formatter.format(new Date(start));
  return end ? `${startText} - ${formatter.format(new Date(end))}` : startText;
}

function formatBytes(bytes: number) {
  if (!bytes) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  return `${(bytes / 1024 ** index).toFixed(1)} ${units[index]}`;
}
