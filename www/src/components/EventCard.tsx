import { CalendarMonth, Collections, LocationOn, PhotoLibrary } from '@mui/icons-material';
import {
  Box,
  Card,
  CardActionArea,
  CardContent,
  CardMedia,
  Chip,
  Stack,
  Typography
} from '@mui/material';
import { useNavigate } from 'react-router-dom';
import type { EventCard as EventCardData } from '../api/client';
import { formatEventLocation } from '../utils/eventLocation';

type EventCardProps = {
  event: EventCardData;
};

export function EventCard({ event }: EventCardProps) {
  const navigate = useNavigate();

  return (
    <Card sx={{ display: 'flex', flexDirection: 'column', height: '100%', overflow: 'hidden' }}>
      <CardActionArea onClick={() => navigate(`/events/${event.id}`)} sx={{ display: 'flex', flexDirection: 'column', flexGrow: 1, height: '100%', alignItems: 'stretch' }}>
        {event.coverUrl ? (
          <CardMedia component="img" height="180" image={event.coverUrl} />
        ) : (
          <Box
            sx={(theme) => ({
              alignItems: 'center',
              bgcolor: theme.palette.action.hover,
              color: 'primary.main',
              display: 'flex',
              height: 180,
              justifyContent: 'center'
            })}
          >
            <PhotoLibrary sx={{ fontSize: 64 }} />
          </Box>
        )}
        <CardContent sx={{ flexGrow: 1, width: '100%' }}>
          <Stack direction="row" sx={{ gap: 1, mb: 2 }}>
            <Chip color={event.isPublic ? 'success' : 'default'} label={event.isPublic ? '公开' : '隐藏'} size="small" />
            <Chip color={event.submissionEnabled ? 'primary' : 'default'} label={event.submissionEnabled ? '可投稿' : '已关闭投稿'} size="small" />
            <Chip icon={<Collections />} label={`${event.photoCount || 0} 张返图`} size="small" />
          </Stack>
          <Typography gutterBottom sx={{ fontWeight: 800 }} variant="h5">
            {event.title}
          </Typography>
          <Typography color="text.secondary" sx={{ mb: 2 }}>
            {event.description}
          </Typography>
          <Stack color="text.secondary" sx={{ gap: 1 }}>
            <Stack direction="row" sx={{ alignItems: 'center', gap: 1 }}>
              <LocationOn fontSize="small" />
              <span>{formatEventLocation(event)}</span>
            </Stack>
            <Stack direction="row" sx={{ alignItems: 'center', gap: 1 }}>
              <CalendarMonth fontSize="small" />
              <span>{formatDateRange(event.startTime, event.endTime)}</span>
            </Stack>
          </Stack>
        </CardContent>
      </CardActionArea>
    </Card>
  );
}

function formatDateRange(start?: string, end?: string) {
  if (!start) {
    return '时间待定';
  }

  const startText = new Intl.DateTimeFormat('zh-CN', {
    dateStyle: 'medium',
    timeStyle: 'short'
  }).format(new Date(start));

  if (!end) {
    return startText;
  }

  const endText = new Intl.DateTimeFormat('zh-CN', {
    dateStyle: 'medium',
    timeStyle: 'short'
  }).format(new Date(end));

  return `${startText} - ${endText}`;
}
