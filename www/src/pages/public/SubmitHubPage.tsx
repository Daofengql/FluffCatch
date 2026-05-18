import { Alert, Box, Button, CircularProgress, FormControl, InputLabel, MenuItem, Paper, Select, Stack, Typography } from '@mui/material';
import { useEffect, useMemo, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { getEvents, resolveSubmissionToken, type EventCard, type SubmissionLink } from '../../api/client';
import { SubmissionForm } from '../../components/SubmissionForm';
import { formatEventLocation } from '../../utils/eventLocation';

export function SubmitHubPage() {
  const [searchParams] = useSearchParams();
  const [events, setEvents] = useState<EventCard[]>([]);
  const [selectedEventId, setSelectedEventId] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  const tokenFromUrl = searchParams.get('token') || searchParams.get('submissionToken') || '';
  const eventIdFromUrl = searchParams.get('eventId') || searchParams.get('event') || '';
  const [tokenPhotographerName, setTokenPhotographerName] = useState('');
  const [tokenLink, setTokenLink] = useState<SubmissionLink | null>(null);
  const [tokenError, setTokenError] = useState('');

  function loadEvents() {
    setLoading(true);
    setError('');
    getEvents()
      .then((items) => {
        const openEvents = Array.isArray(items) ? items.filter((event) => event.isPublic && event.submissionEnabled) : [];
        setEvents(openEvents);
        setSelectedEventId((prev) => {
          if (prev) return prev;
          if (eventIdFromUrl && openEvents.some((event) => String(event.id) === eventIdFromUrl)) return eventIdFromUrl;
          return String(openEvents[0]?.id || '');
        });
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '加载失败'))
      .finally(() => setLoading(false));
  }

  useEffect(loadEvents, []);

  const selectedEvent = useMemo(() => events.find((event) => String(event.id) === selectedEventId) ?? null, [events, selectedEventId]);

  useEffect(() => {
    setTokenPhotographerName('');
    setTokenLink(null);
    setTokenError('');
    if (!selectedEvent || !tokenFromUrl) return;
    resolveSubmissionToken(selectedEvent.id, tokenFromUrl)
      .then((result) => {
        if (!result.valid) {
          setTokenError('这个限时投稿链接无效、已过期或已达到使用次数。');
          return;
        }
        setTokenLink(result.link || null);
        setTokenPhotographerName(result.link?.photographerName || '');
      })
      .catch(() => setTokenError('投稿链接校验失败，请稍后重试。'));
  }, [selectedEvent?.id, tokenFromUrl]);

  return (
    <Box sx={{ alignItems: 'center', display: 'flex', justifyContent: 'center', width: '100%' }}>
      <Paper elevation={3} sx={{ maxWidth: 760, p: { xs: 3, sm: 4 }, width: '100%' }}>
        <Stack sx={{ gap: 2.5 }}>
          <Box>
            <Typography sx={{ fontWeight: 900 }} variant="h4">
              上传返图
            </Typography>
            <Typography color="text.secondary" sx={{ mt: 1 }}>
              通过管理员生成的限时投稿链接上传图片，系统会按队列逐张提交审核。
            </Typography>
          </Box>
          {loading && (
            <Stack sx={{ alignItems: 'center', py: 4 }}>
              <CircularProgress />
            </Stack>
          )}
          {error && (
            <Alert
              action={
                <Button color="inherit" onClick={loadEvents} size="small">
                  重试
                </Button>
              }
              severity="error"
            >
              {error}
            </Alert>
          )}
          {!loading && !error && (
            <>
              <FormControl fullWidth>
                <InputLabel>选择兽聚</InputLabel>
                <Select label="选择兽聚" onChange={(event) => setSelectedEventId(event.target.value)} value={selectedEventId}>
                  {events.map((event) => (
                    <MenuItem key={event.id} value={String(event.id)}>
                      {event.title} / {formatEventLocation(event)}
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
              {!events.length ? (
                <Alert severity="info">目前没有开放投稿的公开兽聚。</Alert>
              ) : !tokenFromUrl ? (
                <Alert severity="warning">请使用管理员生成的限时投稿链接进入上传页。</Alert>
              ) : tokenError ? (
                <Alert severity="warning">{tokenError}</Alert>
              ) : (
                <SubmissionForm
                  event={selectedEvent}
                  initialPhotographerName={tokenPhotographerName}
                  initialSubmissionLink={tokenLink}
                  initialSubmissionToken={tokenFromUrl}
                  lockPhotographerName={Boolean(tokenPhotographerName)}
                />
              )}
            </>
          )}
        </Stack>
      </Paper>
    </Box>
  );
}
