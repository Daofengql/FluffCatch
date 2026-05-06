import { Card, CardContent, Grid, Stack, Typography } from '@mui/material';
import { useEffect, useState } from 'react';
import { getAdminDashboard } from '../../api/client';
import { PageHeader } from '../../components/common/PageHeader';

export function AdminDashboardPage() {
  const [stats, setStats] = useState<Record<string, number>>({});

  useEffect(() => {
    getAdminDashboard().then((payload) => setStats(payload.stats)).catch(() => setStats({}));
  }, []);

  return (
    <Stack sx={{ gap: 3 }}>
      <PageHeader subtitle="返图系统当前数据概览" title="系统概览" />
      <Grid container spacing={2}>
        {[
          ['兽聚数量', stats.events || 0],
          ['正式图片', stats.photos || 0],
          ['待审投稿', stats.pendingSubmissions || 0],
          ['图片容量', formatBytes(stats.photoBytes || 0)]
        ].map(([label, value]) => (
          <Grid key={label} size={{ xs: 12, md: 3 }}>
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
    </Stack>
  );
}

function formatBytes(bytes: number) {
  if (!bytes) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  return `${(bytes / 1024 ** index).toFixed(1)} ${units[index]}`;
}
