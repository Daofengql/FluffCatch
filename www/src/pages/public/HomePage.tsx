import MDEditor from '@uiw/react-md-editor';
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Grid,
  Paper,
  Stack,
  TextField,
  Typography
} from '@mui/material';
import { useColorScheme } from '@mui/material/styles';
import { useEffect, useMemo, useState } from 'react';
import { getEvents, getSiteSettings, type EventCard as EventCardData, type SiteSettings } from '../../api/client';
import { CityCascader, type CityValue } from '../../components/common/CityCascader';
import { EventCard } from '../../components/EventCard';

const fallbackSite: SiteSettings = {
  name: 'FluffCatch',
  subtitle: '兽聚返图收集与画廊',
  logoUrl: '',
  homeMarkdown: '',
  themeMode: 'system',
  themePreset: 'blue',
  themePrimaryColor: '#2563eb',
  publicBackgroundDesktopUrl: '',
  publicBackgroundMobileUrl: '',
  footerText: `© ${new Date().getFullYear()} FluffCatch. All rights reserved.`,
  icpNumber: '',
  policeRecordNumber: '',
  policeRecordUrl: '',
  contactText: '',
  contactEmail: '',
  contactUrl: ''
};

export function HomePage() {
  const { colorScheme } = useColorScheme();
  const [events, setEvents] = useState<EventCardData[]>([]);
  const [site, setSite] = useState<SiteSettings>(fallbackSite);
  const [query, setQuery] = useState('');
  const [regionFilter, setRegionFilter] = useState<CityValue>({ cityCode: '', cityName: '', provinceCode: '', provinceName: '' });
  const [startDate, setStartDate] = useState('');
  const [endDate, setEndDate] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);

  function loadEvents() {
    setLoading(true);
    setError('');
    getEvents()
      .then((items) => setEvents(Array.isArray(items) ? items : []))
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '加载失败'))
      .finally(() => setLoading(false));
  }

  useEffect(() => {
    loadEvents();
    getSiteSettings()
      .then((payload) => setSite({ ...fallbackSite, ...payload }))
      .catch(() => setSite(fallbackSite));
  }, []);

  const filteredEvents = useMemo(() => {
    const keyword = query.trim().toLowerCase();
    const filterStart = startDate ? new Date(`${startDate}T00:00:00`) : null;
    const filterEnd = endDate ? new Date(`${endDate}T23:59:59`) : null;

    return events.filter((event) => {
      if (keyword && !event.title.toLowerCase().includes(keyword)) {
        return false;
      }
      if (regionFilter.cityCode && event.cityCode !== regionFilter.cityCode) {
        return false;
      }
      if (!regionFilter.cityCode && regionFilter.provinceCode && event.provinceCode !== regionFilter.provinceCode) {
        return false;
      }
      if (!filterStart && !filterEnd) {
        return true;
      }

      const eventStart = event.startTime ? new Date(event.startTime) : null;
      const eventEnd = event.endTime ? new Date(event.endTime) : eventStart;
      if (!eventStart && !eventEnd) {
        return false;
      }
      const rangeStart = eventStart ?? eventEnd;
      const rangeEnd = eventEnd ?? eventStart;
      if (filterStart && rangeEnd && rangeEnd < filterStart) {
        return false;
      }
      if (filterEnd && rangeStart && rangeStart > filterEnd) {
        return false;
      }
      return true;
    });
  }, [endDate, events, query, regionFilter.cityCode, regionFilter.provinceCode, startDate]);
  const markdownColorMode = colorScheme === 'dark' ? 'dark' : 'light';

  return (
    <Stack sx={{ gap: 3 }}>
      {site.homeMarkdown.trim() && (
        <Paper
          data-color-mode={markdownColorMode}
          sx={(theme) => ({
            bgcolor: 'background.paper',
            border: '1px solid',
            borderColor: theme.palette.mode === 'dark' ? 'rgba(226, 232, 240, 0.18)' : 'divider',
            borderRadius: 2,
            overflow: 'hidden',
            p: { xs: 3, md: 4 }
          })}
        >
          <Box
            sx={(theme) => ({
              bgcolor: 'transparent',
              '& .wmde-markdown': { bgcolor: 'transparent', color: theme.palette.text.primary },
              '& .wmde-markdown a': { color: theme.palette.primary.main },
              '& .wmde-markdown blockquote': { borderLeftColor: theme.palette.primary.main, color: theme.palette.text.secondary },
              '& .wmde-markdown h1': { fontSize: { xs: '2rem', md: '2.6rem' }, lineHeight: 1.15 },
              '& .wmde-markdown h1, & .wmde-markdown h2, & .wmde-markdown h3': { color: theme.palette.text.primary },
              '& .wmde-markdown hr': { borderColor: theme.palette.divider },
              '& .wmde-markdown pre, & .wmde-markdown code': {
                backgroundColor: theme.palette.mode === 'dark' ? 'rgba(226, 232, 240, 0.1)' : 'rgba(15, 23, 42, 0.06)'
              },
              '& .wmde-markdown table tr': { backgroundColor: 'transparent', borderColor: theme.palette.divider },
              '& .wmde-markdown table tr:nth-of-type(2n)': { backgroundColor: theme.palette.action.hover },
              '& .wmde-markdown table th, & .wmde-markdown table td': { borderColor: theme.palette.divider }
            })}
          >
            <MDEditor.Markdown source={site.homeMarkdown} skipHtml />
          </Box>
        </Paper>
      )}
      <Paper sx={{ borderRadius: 3, p: 2.5 }}>
        <Grid container spacing={2}>
          <Grid size={{ xs: 12, md: 4 }}>
            <TextField fullWidth label="按名称搜索" onChange={(event) => setQuery(event.target.value)} placeholder="输入兽聚名称关键词" value={query} />
          </Grid>
          <Grid size={{ xs: 12, md: 3 }}>
            <CityCascader helperText="可只选省份，也可继续选到城市。" onChange={setRegionFilter} value={regionFilter} />
          </Grid>
          <Grid size={{ xs: 12, sm: 6, md: 2 }}>
            <TextField fullWidth label="开始日期" onChange={(event) => setStartDate(event.target.value)} slotProps={{ inputLabel: { shrink: true } }} type="date" value={startDate} />
          </Grid>
          <Grid size={{ xs: 12, sm: 6, md: 2 }}>
            <TextField fullWidth label="结束日期" onChange={(event) => setEndDate(event.target.value)} slotProps={{ inputLabel: { shrink: true } }} type="date" value={endDate} />
          </Grid>
          <Grid size={{ xs: 12, sm: 6, md: 1 }}>
            <Button fullWidth onClick={() => { setQuery(''); setRegionFilter({ cityCode: '', cityName: '', provinceCode: '', provinceName: '' }); setStartDate(''); setEndDate(''); }} sx={{ height: '100%' }}>
              清空
            </Button>
          </Grid>
        </Grid>
      </Paper>
      {loading && (
        <Box sx={{ display: 'flex', justifyContent: 'center', py: 8 }}>
          <CircularProgress />
        </Box>
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
        <Grid container spacing={3}>
          {filteredEvents.map((event) => (
            <Grid key={event.id} size={{ xs: 12, md: 6, lg: 4 }}>
              <EventCard event={event} />
            </Grid>
          ))}
          {!filteredEvents.length && (
            <Grid size={{ xs: 12 }}>
              <Paper sx={{ p: 4, textAlign: 'center' }}>
                <Typography sx={{ fontWeight: 800 }} variant="h6">
                  {events.length ? '没有匹配的兽聚' : '还没有公开兽聚'}
                </Typography>
                <Typography color="text.secondary" sx={{ mt: 1 }}>
                  {events.length ? '可以调整搜索词、地区或时间范围再试一次。' : '管理员创建公开兽聚后，这里会显示卡片式入口。'}
                </Typography>
              </Paper>
            </Grid>
          )}
        </Grid>
      )}
    </Stack>
  );
}
