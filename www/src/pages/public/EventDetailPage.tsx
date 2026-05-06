import { Alert, Button, Card, CardMedia, CircularProgress, Grid, Stack, Typography } from '@mui/material';
import { useEffect, useState } from 'react';
import { Link as RouterLink, useParams } from 'react-router-dom';
import { getEvent, getPhotos, type EventCard, type Photo } from '../../api/client';

export function EventDetailPage() {
  const eventId = Number(useParams().eventId);
  const [event, setEvent] = useState<EventCard | null>(null);
  const [photos, setPhotos] = useState<Photo[]>([]);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    Promise.all([getEvent(eventId), getPhotos(eventId)])
      .then(([eventData, photoData]) => {
        setEvent(eventData);
        setPhotos(photoData);
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '加载失败'))
      .finally(() => setLoading(false));
  }, [eventId]);

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
      <Stack direction={{ xs: 'column', md: 'row' }} sx={{ alignItems: 'flex-start', gap: 3 }}>
        {event.coverUrl && <CardMedia component="img" image={event.coverUrl} sx={{ borderRadius: 2, maxWidth: 360 }} />}
        <Stack sx={{ gap: 1 }}>
          <Typography sx={{ fontWeight: 800 }} variant="h4">
            {event.title}
          </Typography>
          <Typography color="text.secondary">{event.description}</Typography>
          <Typography color="text.secondary">地点：{event.location || '未填写'}</Typography>
          <Button component={RouterLink} sx={{ width: 'fit-content' }} to={`/events/${event.id}/submit`} variant="contained">
            上传返图
          </Button>
        </Stack>
      </Stack>
      <Grid container spacing={2}>
        {photos.map((photo) => (
          <Grid key={photo.id} size={{ xs: 6, md: 3 }}>
            <Card>
              <CardMedia component="img" image={photo.thumbnailUrl || photo.url} sx={{ aspectRatio: '1 / 1', objectFit: 'cover' }} />
            </Card>
          </Grid>
        ))}
        {!photos.length && (
          <Grid size={{ xs: 12 }}>
            <Alert severity="info">这里暂时还没有公开图片。</Alert>
          </Grid>
        )}
      </Grid>
    </Stack>
  );
}
