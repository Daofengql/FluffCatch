import MDEditor from '@uiw/react-md-editor/nohighlight';
import '@uiw/react-markdown-preview/markdown.css';
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Collapse,
  FormControl,
  Grid,
  InputLabel,
  MenuItem,
  Paper,
  Select,
  Stack,
  TextField,
  Typography
} from '@mui/material';
import type { SelectChangeEvent } from '@mui/material/Select';
import { useColorScheme } from '@mui/material/styles';
import { useEffect, useMemo, useState } from 'react';
import { useLocation } from 'react-router-dom';
import { getEventPage, getSiteSettings, type EventCard as EventCardData, type EventListFilters, type SiteSettings } from '../../api/client';
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
  contactWidgetEnabled: false,
  contactWidgetTitle: '联系我',
  contactWidgetHtml: '',
  footerSections: [
    {
      title: '关于站点',
      html: `<p>兽聚返图收集与画廊</p><p>© ${new Date().getFullYear()} FluffCatch. All rights reserved.</p>`
    },
    {
      title: '快速入口',
      html: '<ul><li><a href="/">首页</a></li><li><a href="/submit">返图入口</a></li></ul>'
    },
    {
      title: '站点信息',
      html: '<p>公开画廊、限时投稿和活动返图都会在这里汇总。</p>'
    }
  ]
};

export function HomePage() {
  const { colorScheme } = useColorScheme();
  const location = useLocation();
  const [events, setEvents] = useState<EventCardData[]>([]);
  const [eventPage, setEventPage] = useState({ page: 1, pageSize: 12, total: 0, totalPages: 0 });
  const [site, setSite] = useState<SiteSettings>(fallbackSite);
  const urlQuery = useMemo(() => new URLSearchParams(location.search).get('q') ?? '', [location.search]);
  const [filters, setFilters] = useState<EventListFilters>({ page: 1, pageSize: 12, sort: 'start_desc' });
  const [filterDraft, setFilterDraft] = useState({
    endDate: '',
    region: { cityCode: '', cityName: '', provinceCode: '', provinceName: '' } as CityValue,
    sort: 'start_desc' as NonNullable<EventListFilters['sort']>,
    startDate: ''
  });
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  const [filtersOpen, setFiltersOpen] = useState(false);

  function loadEvents(nextFilters: EventListFilters) {
    setLoading(true);
    setError('');
    getEventPage(nextFilters)
      .then((payload) => {
        setEvents(payload.events);
        setEventPage({ page: payload.page, pageSize: payload.pageSize, total: payload.total, totalPages: payload.totalPages });
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '加载失败'))
      .finally(() => setLoading(false));
  }

  useEffect(() => {
    getSiteSettings()
      .then((payload) => setSite({ ...fallbackSite, ...payload }))
      .catch(() => setSite(fallbackSite));
  }, []);

  useEffect(() => {
    if (urlQuery.trim() === (filters.query || '')) return;
    setFilters((prev) => ({ ...prev, page: 1, query: urlQuery.trim() }));
  }, [urlQuery]);

  useEffect(() => {
    loadEvents(filters);
  }, [filters]);

  function applyFilters() {
    setFilters((prev) => ({
      ...prev,
      cityCode: filterDraft.region.cityCode || undefined,
      endDate: filterDraft.endDate || undefined,
      page: 1,
      provinceCode: filterDraft.region.provinceCode || undefined,
      sort: filterDraft.sort,
      startDate: filterDraft.startDate || undefined
    }));
  }

  function clearFilters() {
    const emptyRegion = { cityCode: '', cityName: '', provinceCode: '', provinceName: '' };
    setFilterDraft({ endDate: '', region: emptyRegion, sort: 'start_desc', startDate: '' });
    setFilters((prev) => ({ page: 1, pageSize: prev.pageSize, query: prev.query, sort: 'start_desc' }));
  }

  function changeSort(event: SelectChangeEvent) {
    setFilterDraft((prev) => ({ ...prev, sort: event.target.value as NonNullable<EventListFilters['sort']> }));
  }

  function changePage(nextPage: number) {
    setFilters((prev) => ({ ...prev, page: nextPage }));
  }

  const hasSubmittedFilters = Boolean(filters.provinceCode || filters.cityCode || filters.startDate || filters.endDate || (filters.sort && filters.sort !== 'start_desc'));
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
        <Stack direction={{ xs: 'column', md: 'row' }} sx={{ alignItems: { xs: 'stretch', md: 'center' }, justifyContent: 'space-between', gap: 2 }}>
          <Box>
            <Typography sx={{ fontWeight: 800 }}>兽聚筛选</Typography>
            <Typography color="text.secondary" variant="body2">按地区、时间找到想看的兽聚。</Typography>
          </Box>
          <Button onClick={() => setFiltersOpen((prev) => !prev)} sx={{ minHeight: 40, whiteSpace: 'nowrap' }} variant="outlined">
            {filtersOpen ? '收起筛选' : '展开筛选'}
          </Button>
        </Stack>
        <Collapse in={filtersOpen}>
          <Grid container spacing={2} sx={{ mt: 2 }}>
            <Grid size={{ xs: 12, md: 4 }}>
              <CityCascader onChange={(value) => setFilterDraft((prev) => ({ ...prev, region: value }))} value={filterDraft.region} />
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 2 }}>
              <TextField fullWidth label="开始日期" onChange={(event) => setFilterDraft((prev) => ({ ...prev, startDate: event.target.value }))} slotProps={{ inputLabel: { shrink: true } }} type="date" value={filterDraft.startDate} />
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 2 }}>
              <TextField fullWidth label="结束日期" onChange={(event) => setFilterDraft((prev) => ({ ...prev, endDate: event.target.value }))} slotProps={{ inputLabel: { shrink: true } }} type="date" value={filterDraft.endDate} />
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 2 }}>
              <FormControl fullWidth>
                <InputLabel>时间排序</InputLabel>
                <Select label="时间排序" onChange={changeSort} value={filterDraft.sort}>
                  <MenuItem value="start_desc">时间倒序</MenuItem>
                  <MenuItem value="start_asc">时间正序</MenuItem>
                </Select>
              </FormControl>
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 1 }}>
              <Button fullWidth onClick={applyFilters} sx={{ height: '100%' }} variant="contained">
                应用
              </Button>
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 1 }}>
              <Button fullWidth onClick={clearFilters} sx={{ height: '100%' }}>
                清空
              </Button>
            </Grid>
          </Grid>
        </Collapse>
      </Paper>
      {loading && (
        <Box sx={{ display: 'flex', justifyContent: 'center', py: 8 }}>
          <CircularProgress />
        </Box>
      )}
      {error && (
        <Alert
          action={
            <Button color="inherit" onClick={() => loadEvents(filters)} size="small">
              重试
            </Button>
          }
          severity="error"
        >
          {error}
        </Alert>
      )}
      {!loading && !error && (
        <Stack sx={{ gap: 2.5 }}>
          <Stack direction={{ xs: 'column', sm: 'row' }} sx={{ alignItems: { xs: 'flex-start', sm: 'center' }, gap: 1, justifyContent: 'space-between' }}>
            <Typography color="text.secondary" variant="body2">
              共 {eventPage.total} 个公开兽聚{hasSubmittedFilters || urlQuery.trim() ? '符合当前条件' : ''}
            </Typography>
            {eventPage.totalPages > 1 && (
              <Typography color="text.secondary" variant="body2">
                第 {eventPage.page} / {eventPage.totalPages} 页
              </Typography>
            )}
          </Stack>
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
                    {urlQuery.trim() || hasSubmittedFilters ? '没有匹配的兽聚' : '还没有公开兽聚'}
                  </Typography>
                  <Typography color="text.secondary" sx={{ mt: 1 }}>
                    {urlQuery.trim() || hasSubmittedFilters ? '可以调整名称、地区、时间或排序后再试一次。' : '管理员创建公开兽聚后，这里会显示卡片式入口。'}
                  </Typography>
                </Paper>
              </Grid>
            )}
          </Grid>
          {eventPage.totalPages > 1 && (
            <Stack direction="row" sx={{ alignItems: 'center', gap: 1, justifyContent: 'center' }}>
              <Button disabled={eventPage.page <= 1 || loading} onClick={() => changePage(eventPage.page - 1)} variant="outlined">
                上一页
              </Button>
              <Button disabled={eventPage.page >= eventPage.totalPages || loading} onClick={() => changePage(eventPage.page + 1)} variant="outlined">
                下一页
              </Button>
            </Stack>
          )}
        </Stack>
      )}
    </Stack>
  );
}
