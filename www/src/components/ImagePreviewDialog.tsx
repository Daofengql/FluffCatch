import { ChevronLeft, ChevronRight, Close, CloudDownload, Fullscreen, FullscreenExit, RestartAlt, RotateRight, ZoomIn, ZoomOut } from '@mui/icons-material';
import { Box, Button, CircularProgress, Dialog, IconButton, Slider, Stack, Tooltip, Typography } from '@mui/material';
import { type PointerEvent as ReactPointerEvent, type WheelEvent, useEffect, useMemo, useRef, useState } from 'react';
import { downloadPhoto } from '../utils/download';

type ImagePreviewDialogProps = {
  open: boolean;
  src?: string;
  title?: string;
  subtitle?: string;
  images?: ImagePreviewItem[];
  index?: number;
  onIndexChange?: (index: number) => void;
  onClose: () => void;
};

export type ImagePreviewItem = {
  src: string;
  contentType?: string;
  downloadFilename?: string;
  downloadUrl?: string;
  previewSrc?: string;
  title?: string;
  subtitle?: string;
};

type Point = {
  x: number;
  y: number;
};

type PinchState = {
  center: Point;
  distance: number;
  offset: Point;
  zoom: number;
};

export function ImagePreviewDialog({ images, index = 0, onClose, onIndexChange, open, src, subtitle, title }: ImagePreviewDialogProps) {
  const [zoom, setZoom] = useState(1);
  const [rotation, setRotation] = useState(0);
  const [fullscreen, setFullscreen] = useState(false);
  const [offset, setOffset] = useState<Point>({ x: 0, y: 0 });
  const [dragging, setDragging] = useState(false);
  const [downloading, setDownloading] = useState(false);
  const [downloadError, setDownloadError] = useState('');
  const [pinchDistance, setPinchDistance] = useState(0);
  const [loadedOriginalSrc, setLoadedOriginalSrc] = useState('');
  const [loadedOriginals, setLoadedOriginals] = useState<Set<string>>(() => new Set());
  const [trackOffset, setTrackOffset] = useState(0);
  const [trackAnimating, setTrackAnimating] = useState(false);
  const pointersRef = useRef<Map<number, Point>>(new Map());
  const dragStartRef = useRef<Point | null>(null);
  const stageRef = useRef<HTMLDivElement | null>(null);
  const swipeStartRef = useRef<Point | null>(null);
  const swipeLockRef = useRef<'vertical' | 'horizontal' | null>(null);
  const slideTimerRef = useRef<number | null>(null);
  const stageWidthRef = useRef(0);
  const lastTapRef = useRef(0);
  const [activeIndex, setActiveIndex] = useState(index);
  const pinchRef = useRef<PinchState | null>(null);
  const offsetRef = useRef<Point>({ x: 0, y: 0 });
  const zoomRef = useRef(1);
  const slides = useMemo<ImagePreviewItem[]>(() => images?.length ? images : [{ src: src || '', title, subtitle }], [images, src, subtitle, title]);
  const requestedIndex = Math.min(Math.max(index, 0), Math.max(slides.length - 1, 0));
  const currentIndex = Math.min(Math.max(activeIndex, 0), Math.max(slides.length - 1, 0));
  const current = slides[currentIndex] ?? { src: '', title, subtitle };
  const hasMany = slides.length > 1;
  const isVideo = current.contentType?.toLowerCase().startsWith('video/') || false;
  const displaySrc = isVideo ? current.src : displaySourceFor(current, true);

  useEffect(() => {
    if (open) {
      setActiveIndex(requestedIndex);
    }
  }, [open]);

  useEffect(() => {
    if (!open) return;
    resetView();
  }, [open, currentIndex]);

  useEffect(() => {
    if (!open || isVideo || !current.src) {
      setLoadedOriginalSrc('');
      return undefined;
    }
    if (loadedOriginals.has(current.src)) {
      setLoadedOriginalSrc(current.src);
      return undefined;
    }
    if (!current.previewSrc || current.previewSrc === current.src) {
      setLoadedOriginalSrc(current.src);
      setLoadedOriginals((prev) => new Set(prev).add(current.src));
      return undefined;
    }

    let cancelled = false;
    setLoadedOriginalSrc('');
    const image = new Image();
    image.onload = () => {
      if (!cancelled) {
        setLoadedOriginalSrc(current.src);
        setLoadedOriginals((prev) => new Set(prev).add(current.src));
      }
    };
    image.onerror = () => {
      if (!cancelled) {
        setLoadedOriginalSrc(current.previewSrc || current.src);
      }
    };
    image.src = current.src;
    return () => {
      cancelled = true;
    };
  }, [current.previewSrc, current.src, isVideo, loadedOriginals, open]);

  useEffect(() => {
    if (open) {
      setFullscreen(false);
    }
  }, [open]);

  useEffect(() => () => {
    if (slideTimerRef.current !== null) {
      window.clearTimeout(slideTimerRef.current);
    }
  }, []);

  useEffect(() => {
    if (!open) return undefined;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'ArrowLeft') go(currentIndex - 1);
      if (event.key === 'ArrowRight') go(currentIndex + 1);
      if (event.key === '0') resetView();
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [currentIndex, open, slides.length]);

  function resetView() {
    zoomRef.current = 1;
    offsetRef.current = { x: 0, y: 0 };
    pinchRef.current = null;
    setZoom(1);
    setRotation(0);
    setOffset({ x: 0, y: 0 });
    setDragging(false);
    setTrackAnimating(false);
    setTrackOffset(0);
    setPinchDistance(0);
    pointersRef.current.clear();
    dragStartRef.current = null;
    swipeStartRef.current = null;
    swipeLockRef.current = null;
    if (slideTimerRef.current !== null) {
      window.clearTimeout(slideTimerRef.current);
      slideTimerRef.current = null;
    }
  }

  function go(nextIndex: number) {
    if (!slides.length) return;
    const wrapped = (nextIndex + slides.length) % slides.length;
    if (wrapped === currentIndex) return;
    animateToIndex(wrapped, nextIndex > currentIndex ? 1 : -1);
  }

  async function handleDownloadCurrent() {
    const url = current.downloadUrl || current.src;
    if (!url || downloading) return;
    setDownloadError('');
    setDownloading(true);
    try {
      await downloadPhoto(url, current.downloadFilename || fallbackDownloadFilename(current.title, current.contentType));
    } catch (err) {
      setDownloadError(err instanceof Error ? err.message : '下载失败');
    } finally {
      setDownloading(false);
    }
  }

  function updateZoom(nextZoom: number, origin?: Point) {
    const currentZoom = zoomRef.current;
    const clamped = clampZoom(nextZoom);
    let nextOffset = offsetRef.current;
    if (origin && currentZoom !== clamped) {
      const ratio = clamped / currentZoom;
      nextOffset = {
        x: origin.x - (origin.x - offsetRef.current.x) * ratio,
        y: origin.y - (origin.y - offsetRef.current.y) * ratio
      };
    }
    if (clamped === 1) {
      nextOffset = { x: 0, y: 0 };
    }
    zoomRef.current = clamped;
    offsetRef.current = nextOffset;
    setZoom(clamped);
    setOffset(nextOffset);
  }

  function handleWheel(event: WheelEvent<HTMLDivElement>) {
    if (isVideo) return;
    event.preventDefault();
    const rect = event.currentTarget.getBoundingClientRect();
    updateZoom(zoomRef.current + (event.deltaY < 0 ? 0.18 : -0.18), {
      x: event.clientX - rect.left - rect.width / 2,
      y: event.clientY - rect.top - rect.height / 2
    });
  }

  function handlePointerDown(event: ReactPointerEvent<HTMLDivElement>) {
    if (isVideo || trackAnimating) return;
    event.currentTarget.setPointerCapture(event.pointerId);
    stageWidthRef.current = event.currentTarget.getBoundingClientRect().width || window.innerWidth || 1;
    pointersRef.current.set(event.pointerId, { x: event.clientX, y: event.clientY });

    const now = Date.now();
    if (event.pointerType === 'touch' && now - lastTapRef.current < 260 && pointersRef.current.size === 1) {
      updateZoom(zoomRef.current > 1 ? 1 : 2, pointerCenter(event.currentTarget));
      swipeStartRef.current = null;
      swipeLockRef.current = null;
      lastTapRef.current = 0;
      return;
    }
    lastTapRef.current = now;

    if (pointersRef.current.size === 2) {
      const distance = pointerDistance();
      pinchRef.current = {
        center: pointerCenter(event.currentTarget),
        distance,
        offset: offsetRef.current,
        zoom: zoomRef.current
      };
      setPinchDistance(distance);
      swipeStartRef.current = null;
      swipeLockRef.current = null;
      dragStartRef.current = null;
      setDragging(false);
      setTrackOffset(0);
      return;
    }
    if (event.pointerType === 'touch' && hasMany && zoomRef.current <= 1.05) {
      swipeStartRef.current = { x: event.clientX, y: event.clientY };
      swipeLockRef.current = null;
    }
    if (zoomRef.current > 1) {
      setDragging(true);
      dragStartRef.current = { x: event.clientX - offsetRef.current.x, y: event.clientY - offsetRef.current.y };
    }
  }

  function handlePointerMove(event: ReactPointerEvent<HTMLDivElement>) {
    if (isVideo || trackAnimating || !pointersRef.current.has(event.pointerId)) return;
    pointersRef.current.set(event.pointerId, { x: event.clientX, y: event.clientY });
    if (pointersRef.current.size === 2 && pinchRef.current) {
      const nextDistance = pointerDistance();
      if (nextDistance <= 0 || pinchRef.current.distance <= 0) return;
      const nextZoom = clampZoom(pinchRef.current.zoom * (nextDistance / pinchRef.current.distance));
      const ratio = nextZoom / pinchRef.current.zoom;
      const center = pointerCenter(event.currentTarget);
      const nextOffset = nextZoom === 1 ? { x: 0, y: 0 } : {
        x: center.x - (pinchRef.current.center.x - pinchRef.current.offset.x) * ratio,
        y: center.y - (pinchRef.current.center.y - pinchRef.current.offset.y) * ratio
      };
      zoomRef.current = nextZoom;
      offsetRef.current = nextOffset;
      setZoom(nextZoom);
      setOffset(nextOffset);
      setPinchDistance(nextDistance);
      setTrackOffset(0);
      return;
    }
    if (event.pointerType === 'touch' && hasMany && zoomRef.current <= 1.05 && swipeStartRef.current) {
      const deltaX = event.clientX - swipeStartRef.current.x;
      const deltaY = event.clientY - swipeStartRef.current.y;
      const distance = Math.hypot(deltaX, deltaY);
      if (!swipeLockRef.current && distance > 8) {
        swipeLockRef.current = Math.abs(deltaX) > Math.abs(deltaY) * 1.15 ? 'horizontal' : 'vertical';
      }
      if (swipeLockRef.current === 'horizontal') {
        const width = currentStageWidth();
        setTrackOffset(Math.max(-width, Math.min(width, deltaX)));
        return;
      }
    }
    if (dragging && dragStartRef.current) {
      const nextOffset = { x: event.clientX - dragStartRef.current.x, y: event.clientY - dragStartRef.current.y };
      offsetRef.current = nextOffset;
      setOffset(nextOffset);
    }
  }

  function handlePointerUp(event: ReactPointerEvent<HTMLDivElement>) {
    const wasHorizontalSwipe = event.pointerType === 'touch' && hasMany && zoomRef.current <= 1.05 && swipeStartRef.current && swipeLockRef.current === 'horizontal' && pointersRef.current.size <= 1;
    if (event.pointerType === 'touch' && hasMany && zoomRef.current <= 1.05 && swipeStartRef.current && swipeLockRef.current === 'horizontal' && pointersRef.current.size <= 1) {
      const deltaX = event.clientX - swipeStartRef.current.x;
      const deltaY = event.clientY - swipeStartRef.current.y;
      const width = currentStageWidth();
      const threshold = Math.min(110, Math.max(72, width * 0.22));
      if (Math.abs(deltaX) > threshold && Math.abs(deltaX) > Math.abs(deltaY) * 1.15) {
        animateToIndex(wrapIndex(currentIndex + (deltaX < 0 ? 1 : -1)), deltaX < 0 ? 1 : -1);
      } else {
        snapTrackBack();
      }
    }
    if (!wasHorizontalSwipe) {
      setTrackOffset(0);
    }
    swipeStartRef.current = null;
    swipeLockRef.current = null;
    pointersRef.current.delete(event.pointerId);
    if (pointersRef.current.size < 2) {
      pinchRef.current = null;
      setPinchDistance(0);
    }
    if (pointersRef.current.size === 1 && zoomRef.current > 1) {
      const remaining = Array.from(pointersRef.current.values())[0];
      setDragging(true);
      dragStartRef.current = { x: remaining.x - offsetRef.current.x, y: remaining.y - offsetRef.current.y };
    } else if (!pointersRef.current.size) {
      setDragging(false);
      dragStartRef.current = null;
    }
  }

  function pointerDistance() {
    const points = Array.from(pointersRef.current.values());
    if (points.length < 2) return 0;
    return Math.hypot(points[0].x - points[1].x, points[0].y - points[1].y);
  }

  function pointerCenter(target?: HTMLDivElement | null) {
    const points = Array.from(pointersRef.current.values());
    const center = points.reduce(
      (sum, point) => ({ x: sum.x + point.x / points.length, y: sum.y + point.y / points.length }),
      { x: 0, y: 0 }
    );
    const rect = target?.getBoundingClientRect() || stageRef.current?.getBoundingClientRect();
    if (!rect) {
      return { x: center.x - window.innerWidth / 2, y: center.y - window.innerHeight / 2 };
    }
    return { x: center.x - rect.left - rect.width / 2, y: center.y - rect.top - rect.height / 2 };
  }

  function currentStageWidth() {
    const width = stageRef.current?.getBoundingClientRect().width || stageWidthRef.current || window.innerWidth || 1;
    stageWidthRef.current = width;
    return width;
  }

  function wrapIndex(nextIndex: number) {
    return (nextIndex + slides.length) % slides.length;
  }

  function animateToIndex(nextIndex: number, direction: 1 | -1) {
    if (!hasMany || trackAnimating) return;
    const wrapped = wrapIndex(nextIndex);
    if (wrapped === currentIndex) return;
    if (slideTimerRef.current !== null) {
      window.clearTimeout(slideTimerRef.current);
    }
    const width = currentStageWidth();
    setTrackAnimating(true);
    setTrackOffset(direction > 0 ? -width : width);
    slideTimerRef.current = window.setTimeout(() => {
      slideTimerRef.current = null;
      setActiveIndex(wrapped);
      setTrackOffset(0);
      setTrackAnimating(false);
      onIndexChange?.(wrapped);
    }, 270);
  }

  function snapTrackBack() {
    if (slideTimerRef.current !== null) {
      window.clearTimeout(slideTimerRef.current);
    }
    setTrackAnimating(true);
    setTrackOffset(0);
    slideTimerRef.current = window.setTimeout(() => {
      slideTimerRef.current = null;
      setTrackAnimating(false);
    }, 180);
  }

  function displaySourceFor(item: ImagePreviewItem, active: boolean) {
    if (item.contentType?.toLowerCase().startsWith('video/')) return item.src;
    if (loadedOriginals.has(item.src)) return item.src;
    if (active && loadedOriginalSrc === item.src) return item.src;
    return item.previewSrc || item.src;
  }

  function markOriginalLoaded(value: string) {
    if (!value) return;
    setLoadedOriginalSrc((prev) => prev || (value === current.src ? value : prev));
    setLoadedOriginals((prev) => {
      if (prev.has(value)) return prev;
      const next = new Set(prev);
      next.add(value);
      return next;
    });
  }

  function renderVisibleSlides() {
    if (!slides.length) return null;
    if (!hasMany) {
      return renderSlide(currentIndex, 0);
    }
    if (slides.length === 2) {
      const otherIndex = wrapIndex(currentIndex + 1);
      const side = trackOffset > 0 ? -1 : 1;
      return (
        <>
          {renderSlide(currentIndex, 0)}
          {renderSlide(otherIndex, side)}
        </>
      );
    }
    return (
      <>
        {renderSlide(wrapIndex(currentIndex - 1), -1)}
        {renderSlide(currentIndex, 0)}
        {renderSlide(wrapIndex(currentIndex + 1), 1)}
      </>
    );
  }

  function renderSlide(itemIndex: number, slot: -1 | 0 | 1) {
    const item = slides[itemIndex];
    if (!item) return null;
    const active = slot === 0;
    const itemIsVideo = item.contentType?.toLowerCase().startsWith('video/') || false;
    const itemSrc = displaySourceFor(item, active);
    const isLoadedOriginal = itemIsVideo || loadedOriginals.has(item.src) || (active && loadedOriginalSrc === item.src);
    const loadingOriginal = active && !itemIsVideo && Boolean(item.previewSrc && item.previewSrc !== item.src) && !isLoadedOriginal;
    if (!itemSrc) return null;
    return (
      <Box
        key={`${itemIndex}-${item.src}`}
        sx={{
          alignItems: 'center',
          display: 'flex',
          inset: 0,
          justifyContent: 'center',
          overflow: 'hidden',
          position: 'absolute',
          transform: `translate3d(calc(${slot * 100}% + ${trackOffset}px), 0, 0)`,
          transition: trackAnimating ? 'transform 260ms cubic-bezier(0.2, 0, 0, 1)' : 'none',
          width: '100%',
          willChange: 'transform'
        }}
      >
        {itemIsVideo ? (
          <Box
            component="video"
            controls={active}
            muted={!active}
            preload="metadata"
            src={itemSrc}
            sx={{
              bgcolor: 'common.black',
              display: 'block',
              maxHeight: '100%',
              maxWidth: '100%',
              objectFit: 'contain',
              pointerEvents: active ? 'auto' : 'none',
              width: '100%'
            }}
          />
        ) : (
          <Box
            sx={{
              alignItems: 'center',
              display: 'flex',
              height: '100%',
              justifyContent: 'center',
              maxHeight: '100%',
              maxWidth: '100%',
              position: 'relative',
              transform: active ? `translate3d(${offset.x}px, ${offset.y}px, 0) scale(${zoom}) rotate(${rotation}deg)` : 'none',
              transformOrigin: 'center center',
              transition: active && !dragging && !pinchDistance && !trackAnimating ? 'transform 100ms ease, opacity 100ms ease' : 'none',
              width: '100%',
              willChange: 'transform'
            }}
          >
            {item.previewSrc && item.previewSrc !== item.src ? (
              <>
                <Box
                  alt={item.title || title || 'preview'}
                  component="img"
                  draggable={false}
                  src={item.previewSrc}
                  sx={{
                    display: 'block',
                    filter: isLoadedOriginal ? undefined : 'blur(1.5px)',
                    height: '100%',
                    maxHeight: '100%',
                    maxWidth: '100%',
                    objectFit: 'contain',
                    opacity: isLoadedOriginal ? 0 : 1,
                    transition: 'opacity 180ms ease, filter 180ms ease',
                    width: '100%'
                  }}
                />
                <Box
                  alt={item.title || title || 'preview'}
                  component="img"
                  draggable={false}
                  onLoad={() => markOriginalLoaded(item.src)}
                  src={item.src}
                  sx={{
                    display: 'block',
                    height: '100%',
                    inset: 0,
                    maxHeight: '100%',
                    maxWidth: '100%',
                    objectFit: 'contain',
                    opacity: isLoadedOriginal ? 1 : 0,
                    position: 'absolute',
                    transition: 'opacity 180ms ease',
                    width: '100%'
                  }}
                />
                {loadingOriginal && (
                  <Stack
                    direction="row"
                    sx={{
                      alignItems: 'center',
                      bgcolor: 'rgba(15,23,42,0.72)',
                      border: '1px solid rgba(255,255,255,0.14)',
                      borderRadius: 999,
                      color: 'white',
                      gap: 1,
                      left: '50%',
                      px: 1.25,
                      py: 0.75,
                      pointerEvents: 'none',
                      position: 'absolute',
                      top: { xs: 14, sm: 18 },
                      transform: 'translateX(-50%)',
                      zIndex: 2
                    }}
                  >
                    <CircularProgress color="inherit" size={16} />
                    <Typography sx={{ fontWeight: 700 }} variant="caption">加载原图</Typography>
                  </Stack>
                )}
              </>
            ) : (
              <Box
                alt={item.title || title || 'preview'}
                component="img"
                draggable={false}
                onLoad={() => markOriginalLoaded(item.src)}
                src={itemSrc}
                sx={{
                  display: 'block',
                  height: '100%',
                  maxHeight: '100%',
                  maxWidth: '100%',
                  objectFit: 'contain',
                  width: '100%'
                }}
              />
            )}
          </Box>
        )}
      </Box>
    );
  }

  return (
    <Dialog
      fullScreen={fullscreen}
      maxWidth={false}
      onClose={onClose}
      open={open}
      slotProps={{
        backdrop: { sx: { bgcolor: 'rgba(2, 6, 23, 0.78)' } },
        paper: {
          sx: {
            bgcolor: '#0f172a',
            border: { xs: 0, sm: '1px solid rgba(255,255,255,0.12)' },
            borderRadius: { xs: 0, sm: fullscreen ? 0 : 3 },
            boxShadow: fullscreen ? 'none' : '0 16px 40px rgba(2, 6, 23, 0.45)',
            color: 'white',
            height: { xs: '100dvh', sm: fullscreen ? '100vh' : '86vh' },
            m: { xs: 0, sm: fullscreen ? 0 : 4 },
            maxHeight: { xs: '100dvh', sm: fullscreen ? '100vh' : '86vh' },
            maxWidth: { xs: '100vw', sm: fullscreen ? '100vw' : '90vw' },
            overflow: 'hidden',
            width: { xs: '100vw', sm: fullscreen ? '100vw' : '90vw' }
          }
        }
      }}
    >
      <Stack sx={{ height: '100%', overflow: 'hidden' }}>
        <Stack
          direction="row"
          sx={{
            alignItems: 'center',
            borderBottom: '1px solid rgba(255,255,255,0.08)',
            flexWrap: { xs: 'wrap', sm: 'nowrap' },
            gap: { xs: 0.75, sm: 1 },
            p: { xs: 1, sm: 1.5 }
          }}
        >
          <Box sx={{ flex: 1, minWidth: { xs: 'calc(100% - 48px)', sm: 0 }, order: { xs: 1, sm: 0 } }}>
            <Typography noWrap sx={{ fontWeight: 800 }}>
              {current.title || title || '图片预览'}
            </Typography>
            {(current.subtitle || subtitle || hasMany) && (
              <Typography color="grey.300" noWrap variant="body2">
                {[current.subtitle || subtitle, hasMany ? `${currentIndex + 1} / ${slides.length}` : ''].filter(Boolean).join(' · ')}
              </Typography>
            )}
            {downloadError && (
              <Typography color="error.light" noWrap variant="caption">
                {downloadError}
              </Typography>
            )}
          </Box>
          <Stack direction="row" sx={{ alignItems: 'center', display: { xs: 'none', md: 'flex' }, gap: 1, minWidth: 220, order: { xs: 3, sm: 0 } }}>
            <Typography variant="caption">{Math.round(zoom * 100)}%</Typography>
            <Slider
              disabled={isVideo}
              max={4}
              min={0.5}
              onChange={(_, value) => updateZoom(Array.isArray(value) ? value[0] : value)}
              size="small"
              step={0.1}
              value={zoom}
            />
          </Stack>
          <Stack direction="row" sx={{ gap: 0.25, order: { xs: 2, sm: 0 } }}>
            <IconButton color="inherit" disabled={isVideo} onClick={() => updateZoom(zoom - 0.2)} size="small">
              <ZoomOut />
            </IconButton>
            <IconButton color="inherit" disabled={isVideo} onClick={() => updateZoom(zoom + 0.2)} size="small">
              <ZoomIn />
            </IconButton>
            <IconButton color="inherit" disabled={isVideo} onClick={() => setRotation((value) => (value + 90) % 360)} size="small">
              <RotateRight />
            </IconButton>
            <IconButton color="inherit" disabled={isVideo} onClick={resetView} size="small">
              <RestartAlt />
            </IconButton>
            <Tooltip title="下载当前媒体">
              <span>
                <IconButton color="inherit" disabled={downloading || !current.src} onClick={() => void handleDownloadCurrent()} size="small">
                  {downloading ? <CircularProgress color="inherit" size={20} /> : <CloudDownload />}
                </IconButton>
              </span>
            </Tooltip>
            <IconButton color="inherit" onClick={() => setFullscreen((value) => !value)} size="small" sx={{ display: { xs: 'none', sm: 'inline-flex' } }}>
              {fullscreen ? <FullscreenExit /> : <Fullscreen />}
            </IconButton>
            <IconButton color="inherit" onClick={onClose} size="small">
              <Close />
            </IconButton>
          </Stack>
        </Stack>
        <Box
          ref={stageRef}
          onPointerCancel={handlePointerUp}
          onPointerDown={handlePointerDown}
          onPointerLeave={handlePointerUp}
          onPointerMove={handlePointerMove}
          onPointerUp={handlePointerUp}
          onWheel={handleWheel}
          sx={{
            alignItems: 'center',
            cursor: isVideo ? 'default' : zoom > 1 ? dragging ? 'grabbing' : 'grab' : 'zoom-in',
            display: 'flex',
            flex: 1,
            justifyContent: 'center',
            overflow: 'hidden',
            position: 'relative',
            touchAction: isVideo ? 'auto' : 'none',
            userSelect: 'none'
          }}
        >
          {hasMany && (
            <IconButton
              color="inherit"
              onClick={() => go(currentIndex - 1)}
              sx={{ bgcolor: 'rgba(15,23,42,0.55)', display: { xs: 'none', sm: 'inline-flex' }, left: 16, position: 'absolute', top: '50%', zIndex: 2 }}
            >
              <ChevronLeft />
            </IconButton>
          )}
          <Box sx={{ inset: 0, overflow: 'hidden', position: 'absolute' }}>
            {renderVisibleSlides()}
          </Box>
          {hasMany && (
            <IconButton
              color="inherit"
              onClick={() => go(currentIndex + 1)}
              sx={{ bgcolor: 'rgba(15,23,42,0.55)', display: { xs: 'none', sm: 'inline-flex' }, position: 'absolute', right: 16, top: '50%', zIndex: 2 }}
            >
              <ChevronRight />
            </IconButton>
          )}
        </Box>
        {hasMany && (
          <Stack direction="row" sx={{ borderTop: '1px solid rgba(255,255,255,0.08)', gap: 1, justifyContent: 'center', p: { xs: 1, sm: 1.5 } }}>
            <Button color="inherit" fullWidth onClick={() => go(currentIndex - 1)} startIcon={<ChevronLeft />} variant="outlined">
              上一张
            </Button>
            <Button color="inherit" endIcon={<ChevronRight />} fullWidth onClick={() => go(currentIndex + 1)} variant="outlined">
              下一张
            </Button>
          </Stack>
        )}
      </Stack>
    </Dialog>
  );
}

function clampZoom(value: number) {
  return Math.max(0.5, Math.min(4, Number(value.toFixed(2))));
}

function fallbackDownloadFilename(title?: string, contentType?: string) {
  const extension = contentType?.includes('png') ? 'png' : contentType?.startsWith('video/') ? 'mp4' : 'jpg';
  const name = (title || 'fluffcatch-media').replace(/[\\/:*?"<>|]+/g, '_').trim() || 'fluffcatch-media';
  return `${name}.${extension}`;
}
