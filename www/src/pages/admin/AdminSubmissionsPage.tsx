import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  CardMedia,
  Checkbox,
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
import { useLocation } from 'react-router-dom';
import { approveSubmissions, deleteSubmissions, getPendingSubmissions, type Submission } from '../../api/client';
import { PageHeader } from '../../components/common/PageHeader';
import { ImagePreviewDialog, type ImagePreviewItem } from '../../components/ImagePreviewDialog';

type ViewMode = 'card' | 'list';

export function AdminSubmissionsPage() {
  const location = useLocation();
  const [submissions, setSubmissions] = useState<Submission[]>([]);
  const [selected, setSelected] = useState<number[]>([]);
  const [error, setError] = useState('');
  const [previewIndex, setPreviewIndex] = useState<number | null>(null);
  const [viewMode, setViewMode] = useState<ViewMode>('card');
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
    setError('');
    getPendingSubmissions()
      .then((items) => {
        setSubmissions(items);
        setSelected((prev) => prev.filter((id) => items.some((item) => item.id === id)));
      })
      .catch((err) => setError(err instanceof Error ? err.message : '加载失败'));
  }

  useEffect(refresh, [location.key]);

  async function batch(action: 'approve' | 'delete') {
    try {
      if (action === 'approve') await approveSubmissions(selected);
      else await deleteSubmissions(selected);
      setSelected([]);
      refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : '操作失败');
    }
  }

  return (
    <Stack sx={{ gap: 3 }}>
      <PageHeader
        actions={
          <>
            <ToggleButtonGroup
              exclusive
              onChange={(_, value: ViewMode | null) => value && setViewMode(value)}
              size="small"
              value={viewMode}
            >
              <ToggleButton value="card">卡片</ToggleButton>
              <ToggleButton value="list">列表</ToggleButton>
            </ToggleButtonGroup>
            <Button disabled={!selected.length} onClick={() => batch('approve')} variant="contained">批量通过</Button>
            <Button color="error" disabled={!selected.length} onClick={() => batch('delete')} variant="outlined">批量删除</Button>
          </>
        }
        subtitle="把投稿转入正式画廊"
        title="投稿审核"
      />
      {error && <Alert severity="error">{error}</Alert>}
      {viewMode === 'card' ? (
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
                    <Checkbox
                      checked={selected.includes(submission.id)}
                      onChange={(event) => setSelected((prev) => event.target.checked ? [...prev, submission.id] : prev.filter((id) => id !== submission.id))}
                      size="small"
                    />
                    <Box sx={{ minWidth: 0 }}>
                      <Typography noWrap sx={{ fontWeight: 800 }} variant="body2">#{submission.id}</Typography>
                      <Typography color="text.secondary" noWrap variant="caption">{submission.photographerName || '匿名'} / {submission.contentHash.slice(0, 8)}</Typography>
                    </Box>
                  </Stack>
                </CardContent>
              </Card>
            </Grid>
          ))}
          {!submissions.length && (
            <Grid size={{ xs: 12 }}>
              <Box sx={{ p: 4, textAlign: 'center' }}>
                <Typography color="text.secondary">暂无待审核投稿。</Typography>
              </Box>
            </Grid>
          )}
        </Grid>
      ) : (
        <TableContainer component={Paper}>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell padding="checkbox" />
                <TableCell>图片</TableCell>
                <TableCell>投稿</TableCell>
                <TableCell>摄影师</TableCell>
                <TableCell>Hash</TableCell>
                <TableCell>时间</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {submissions.map((submission, index) => (
                <TableRow hover key={submission.id}>
                  <TableCell padding="checkbox">
                    <Checkbox checked={selected.includes(submission.id)} onChange={(event) => setSelected((prev) => event.target.checked ? [...prev, submission.id] : prev.filter((id) => id !== submission.id))} />
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
                    <Typography color="text.secondary" sx={{ py: 4 }}>暂无待审核投稿。</Typography>
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </TableContainer>
      )}
      <ImagePreviewDialog
        images={previewItems}
        index={previewIndex ?? 0}
        onClose={() => setPreviewIndex(null)}
        onIndexChange={setPreviewIndex}
        open={previewIndex !== null}
      />
    </Stack>
  );
}
