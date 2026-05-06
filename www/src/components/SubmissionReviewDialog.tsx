import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  CardMedia,
  Checkbox,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Grid,
  Paper,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  ToggleButton,
  ToggleButtonGroup,
  Typography
} from '@mui/material';
import { useEffect, useMemo, useState } from 'react';
import { approveSubmissions, deleteSubmissions, getEventPendingSubmissions, type EventCard, type Submission } from '../api/client';
import { usePersistentState } from '../hooks/usePersistentState';
import { ImagePreviewDialog, type ImagePreviewItem } from './ImagePreviewDialog';

type ViewMode = 'card' | 'list';
const viewModes = ['card', 'list'] as const;

type SubmissionReviewDialogProps = {
  event: EventCard | null;
  onChanged?: () => void;
  onClose: () => void;
  open: boolean;
};

export function SubmissionReviewDialog({ event, onChanged, onClose, open }: SubmissionReviewDialogProps) {
  const [submissions, setSubmissions] = useState<Submission[]>([]);
  const [selected, setSelected] = useState<number[]>([]);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [previewIndex, setPreviewIndex] = useState<number | null>(null);
  const [viewMode, setViewMode] = usePersistentState<ViewMode>('fluffcatch.event.submissions.viewMode', 'card', viewModes);

  const previewItems = useMemo<ImagePreviewItem[]>(
    () =>
      submissions.map((submission) => ({
        src: submission.url,
        subtitle: submission.photographerName ? `摄影师：${submission.photographerName}` : '匿名投稿',
        title: `投稿 #${submission.id}`
      })),
    [submissions]
  );

  function refresh() {
    if (!event) return;
    setLoading(true);
    setError('');
    getEventPendingSubmissions(event.id)
      .then((items) => {
        setSubmissions(items);
        setSelected((prev) => prev.filter((id) => items.some((item) => item.id === id)));
      })
      .catch((err) => setError(err instanceof Error ? err.message : '加载投稿失败'))
      .finally(() => setLoading(false));
  }

  useEffect(() => {
    if (open) refresh();
  }, [event?.id, open]);

  function toggleSubmission(id: number, checked: boolean) {
    setSelected((prev) => (checked ? Array.from(new Set([...prev, id])) : prev.filter((item) => item !== id)));
  }

  function toggleAll(checked: boolean) {
    setSelected(checked ? submissions.map((submission) => submission.id) : []);
  }

  async function batch(action: 'approve' | 'delete') {
    if (!selected.length) return;
    setError('');
    try {
      if (action === 'approve') await approveSubmissions(selected);
      else await deleteSubmissions(selected);
      setSelected([]);
      refresh();
      onChanged?.();
    } catch (err) {
      setError(err instanceof Error ? err.message : '操作失败');
    }
  }

  const allSelected = submissions.length > 0 && selected.length === submissions.length;
  const someSelected = selected.length > 0 && selected.length < submissions.length;

  return (
    <Dialog fullWidth maxWidth="lg" onClose={onClose} open={open}>
      <DialogTitle>
        <Stack direction={{ xs: 'column', sm: 'row' }} sx={{ alignItems: { xs: 'stretch', sm: 'center' }, justifyContent: 'space-between', gap: 2 }}>
          <Box>
            <Typography sx={{ fontWeight: 900 }} variant="h6">
              审核返图
            </Typography>
            <Typography color="text.secondary" variant="body2">
              {event?.title || '当前兽聚'} / 当前待审核 {submissions.length} 张
            </Typography>
          </Box>
          <Stack direction="row" sx={{ gap: 1 }}>
            <ToggleButtonGroup exclusive onChange={(_, value: ViewMode | null) => value && setViewMode(value)} size="small" value={viewMode}>
              <ToggleButton value="card">卡片</ToggleButton>
              <ToggleButton value="list">列表</ToggleButton>
            </ToggleButtonGroup>
            <Button disabled={!selected.length} onClick={() => void batch('approve')} variant="contained">
              批量通过
            </Button>
            <Button color="error" disabled={!selected.length} onClick={() => void batch('delete')} variant="outlined">
              批量删除
            </Button>
          </Stack>
        </Stack>
      </DialogTitle>
      <DialogContent dividers>
        {error && <Alert severity="error" sx={{ mb: 2 }}>{error}</Alert>}
        {loading ? (
          <Typography color="text.secondary" sx={{ py: 4, textAlign: 'center' }}>
            投稿加载中...
          </Typography>
        ) : viewMode === 'card' ? (
          <Grid container spacing={1.5}>
            {submissions.map((submission, index) => (
              <Grid key={submission.id} size={{ xs: 6, sm: 4, md: 2.4 }}>
                <Card>
                  <CardMedia
                    component="img"
                    image={submission.thumbnailUrl || submission.url}
                    onClick={() => setPreviewIndex(index)}
                    sx={{ aspectRatio: '1 / 1', cursor: 'zoom-in', objectFit: 'cover' }}
                  />
                  <CardContent sx={{ p: 1 }}>
                    <Stack direction="row" sx={{ alignItems: 'center', gap: 0.5 }}>
                      <Checkbox checked={selected.includes(submission.id)} onChange={(event) => toggleSubmission(submission.id, event.target.checked)} size="small" />
                      <Box sx={{ minWidth: 0 }}>
                        <Typography noWrap sx={{ fontWeight: 800 }} variant="body2">
                          #{submission.id}
                        </Typography>
                        <Typography color="text.secondary" noWrap variant="caption">
                          {submission.photographerName || '匿名'} / {submission.contentHash.slice(0, 8)}
                        </Typography>
                      </Box>
                    </Stack>
                  </CardContent>
                </Card>
              </Grid>
            ))}
            {!submissions.length && (
              <Grid size={{ xs: 12 }}>
                <Box sx={{ p: 4, textAlign: 'center' }}>
                  <Typography color="text.secondary">当前兽聚暂无待审核投稿。</Typography>
                </Box>
              </Grid>
            )}
          </Grid>
        ) : (
          <TableContainer component={Paper}>
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell padding="checkbox">
                    <Checkbox checked={allSelected} indeterminate={someSelected} onChange={(event) => toggleAll(event.target.checked)} />
                  </TableCell>
                  <TableCell>图片</TableCell>
                  <TableCell>投稿</TableCell>
                  <TableCell>摄影师</TableCell>
                  <TableCell>Hash</TableCell>
                  <TableCell>时间</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {submissions.map((submission, index) => (
                  <TableRow hover key={submission.id} selected={selected.includes(submission.id)}>
                    <TableCell padding="checkbox">
                      <Checkbox checked={selected.includes(submission.id)} onChange={(event) => toggleSubmission(submission.id, event.target.checked)} />
                    </TableCell>
                    <TableCell>
                      <Box
                        component="img"
                        onClick={() => setPreviewIndex(index)}
                        src={submission.thumbnailUrl || submission.url}
                        sx={{ borderRadius: 1, cursor: 'zoom-in', height: 56, objectFit: 'cover', width: 76 }}
                      />
                    </TableCell>
                    <TableCell>#{submission.id}</TableCell>
                    <TableCell>{submission.photographerName || '匿名'}</TableCell>
                    <TableCell>{submission.contentHash.slice(0, 16)}</TableCell>
                    <TableCell>{new Intl.DateTimeFormat('zh-CN', { dateStyle: 'short', timeStyle: 'short' }).format(new Date(submission.createdAt))}</TableCell>
                  </TableRow>
                ))}
                {!submissions.length && (
                  <TableRow>
                    <TableCell align="center" colSpan={6}>
                      <Typography color="text.secondary" sx={{ py: 4 }}>
                        当前兽聚暂无待审核投稿。
                      </Typography>
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </TableContainer>
        )}
      </DialogContent>
      <DialogActions sx={{ justifyContent: 'space-between', px: 3 }}>
        <Typography color="text.secondary" variant="body2">
          已选择 {selected.length} 张
        </Typography>
        <Button onClick={onClose}>关闭</Button>
      </DialogActions>
      <ImagePreviewDialog
        images={previewItems}
        index={previewIndex ?? 0}
        onClose={() => setPreviewIndex(null)}
        onIndexChange={setPreviewIndex}
        open={previewIndex !== null}
      />
    </Dialog>
  );
}
