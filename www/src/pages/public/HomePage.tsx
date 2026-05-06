import { Alert, Box, CircularProgress, Grid, Paper, Stack, Typography } from '@mui/material';
import { useEffect, useState } from 'react';
import { getEvents, type EventCard as EventCardData } from '../../api/client';
import { EventCard } from '../../components/EventCard';

export function HomePage() {
  const [events, setEvents] = useState<EventCardData[]>([]);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    getEvents()
      .then(setEvents)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '加载失败'))
      .finally(() => setLoading(false));
  }, []);

  return (
    <Stack sx={{ gap: 3 }}>
      <Box>
        <Typography sx={{ fontWeight: 800 }} variant="h4">
          兽聚返图画廊
        </Typography>
        <Typography color="text.secondary" sx={{ mt: 1 }}>
          每场兽聚独立收图、审核、归档，不再到处翻返图。
        </Typography>
      </Box>
      {loading && (
        <Box sx={{ display: 'flex', justifyContent: 'center', py: 8 }}>
          <CircularProgress />
        </Box>
      )}
      {error && <Alert severity="error">{error}</Alert>}
      {!loading && !error && (
        <Grid container spacing={3}>
          {events.map((event) => (
            <Grid key={event.id} size={{ xs: 12, md: 6, lg: 4 }}>
              <EventCard event={event} />
            </Grid>
          ))}
          {!events.length && (
            <Grid size={{ xs: 12 }}>
              <Paper sx={{ p: 4, textAlign: 'center' }}>
                <Typography sx={{ fontWeight: 800 }} variant="h6">
                  还没有公开兽聚
                </Typography>
                <Typography color="text.secondary" sx={{ mt: 1 }}>
                  管理员创建公开兽聚后，这里会显示卡片式入口。
                </Typography>
              </Paper>
            </Grid>
          )}
        </Grid>
      )}
    </Stack>
  );
}
