import { Alert, Button, Card, CardContent, Checkbox, Grid, Stack, Typography } from '@mui/material';
import { useEffect, useState } from 'react';
import { approveSubmissions, deleteSubmissions, getPendingSubmissions, type Submission } from '../../api/client';
import { PageHeader } from '../../components/common/PageHeader';

export function AdminSubmissionsPage() {
  const [submissions, setSubmissions] = useState<Submission[]>([]);
  const [selected, setSelected] = useState<number[]>([]);
  const [error, setError] = useState('');

  function refresh() {
    getPendingSubmissions().then(setSubmissions).catch((err) => setError(err instanceof Error ? err.message : '加载失败'));
  }
  useEffect(refresh, []);

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
        actions={<><Button disabled={!selected.length} onClick={() => batch('approve')} variant="contained">批量通过</Button><Button color="error" disabled={!selected.length} onClick={() => batch('delete')} variant="outlined">批量删除</Button></>}
        subtitle="把投稿转入正式画廊"
        title="投稿审核"
      />
      {error && <Alert severity="error">{error}</Alert>}
      <Grid container spacing={2}>
        {submissions.map((submission) => (
          <Grid key={submission.id} size={{ xs: 12, md: 6 }}>
            <Card>
              <CardContent>
                <Stack direction="row" sx={{ alignItems: 'center', gap: 1 }}>
                  <Checkbox checked={selected.includes(submission.id)} onChange={(event) => setSelected((prev) => event.target.checked ? [...prev, submission.id] : prev.filter((id) => id !== submission.id))} />
                  <Stack>
                    <Typography sx={{ fontWeight: 800 }}>{submission.originalFilename}</Typography>
                    <Typography color="text.secondary">摄影师：{submission.photographerName || '匿名'}</Typography>
                    <Typography color="text.secondary">标签：{submission.tags.join(' ') || '无'}</Typography>
                  </Stack>
                </Stack>
              </CardContent>
            </Card>
          </Grid>
        ))}
      </Grid>
    </Stack>
  );
}
