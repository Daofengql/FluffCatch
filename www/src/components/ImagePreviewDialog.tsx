import { ChevronLeft, ChevronRight, Close, Fullscreen, FullscreenExit, RotateRight, ZoomIn, ZoomOut } from '@mui/icons-material';
import { Box, Button, Dialog, IconButton, Slider, Stack, Typography } from '@mui/material';
import { useEffect, useMemo, useState } from 'react';

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
  title?: string;
  subtitle?: string;
};

export function ImagePreviewDialog({ images, index = 0, onClose, onIndexChange, open, src, subtitle, title }: ImagePreviewDialogProps) {
  const [zoom, setZoom] = useState(1);
  const [rotation, setRotation] = useState(0);
  const [fullscreen, setFullscreen] = useState(false);
  const slides = useMemo<ImagePreviewItem[]>(() => images?.length ? images : [{ src: src || '', title, subtitle }], [images, src, subtitle, title]);
  const currentIndex = Math.min(Math.max(index, 0), Math.max(slides.length - 1, 0));
  const current = slides[currentIndex] ?? { src: '', title, subtitle };
  const hasMany = slides.length > 1;
  const isVideo = current.contentType?.toLowerCase().startsWith('video/') || false;

  useEffect(() => {
    if (open) {
      setZoom(1);
      setRotation(0);
      setFullscreen(false);
    }
  }, [open, currentIndex]);

  function go(nextIndex: number) {
    if (!slides.length) return;
    const wrapped = (nextIndex + slides.length) % slides.length;
    onIndexChange?.(wrapped);
  }

  return (
    <Dialog
      fullScreen={fullscreen}
      maxWidth={false}
      onClose={onClose}
      open={open}
      slotProps={{
        backdrop: { sx: { bgcolor: 'rgba(2, 6, 23, 0.72)' } },
        paper: {
          sx: {
            bgcolor: '#0f172a',
            border: '1px solid rgba(255,255,255,0.12)',
            borderRadius: fullscreen ? 0 : 4,
            boxShadow: fullscreen ? 'none' : '0 16px 40px rgba(2, 6, 23, 0.45)',
            color: 'white',
            height: fullscreen ? '100vh' : '80vh',
            maxHeight: fullscreen ? '100vh' : '80vh',
            maxWidth: fullscreen ? '100vw' : '80vw',
            overflow: 'hidden',
            width: fullscreen ? '100vw' : '80vw'
          }
        }
      }}
    >
      <Stack sx={{ height: '100%', overflow: 'hidden' }}>
        <Stack direction="row" sx={{ alignItems: 'center', borderBottom: '1px solid rgba(255,255,255,0.08)', gap: 1.5, p: 1.5 }}>
          <Box sx={{ flex: 1, minWidth: 0 }}>
            <Typography noWrap sx={{ fontWeight: 800 }}>
              {current.title || title || '图片预览'}
            </Typography>
            {(current.subtitle || subtitle || hasMany) && (
              <Typography color="grey.300" noWrap variant="body2">
                {[current.subtitle || subtitle, hasMany ? `${currentIndex + 1} / ${slides.length}` : ''].filter(Boolean).join(' · ')}
              </Typography>
            )}
          </Box>
          <Stack direction="row" sx={{ alignItems: 'center', display: { xs: 'none', md: 'flex' }, gap: 1, minWidth: 240 }}>
            <Typography variant="caption">{Math.round(zoom * 100)}%</Typography>
            <Slider
              max={3}
              min={0.5}
              onChange={(_, value) => setZoom(Array.isArray(value) ? value[0] : value)}
              size="small"
              step={0.1}
              value={zoom}
            />
          </Stack>
          <IconButton color="inherit" disabled={isVideo} onClick={() => setZoom((value) => Math.max(0.5, Number((value - 0.1).toFixed(1))))}>
            <ZoomOut />
          </IconButton>
          <IconButton color="inherit" disabled={isVideo} onClick={() => setZoom((value) => Math.min(3, Number((value + 0.1).toFixed(1))))}>
            <ZoomIn />
          </IconButton>
          <IconButton color="inherit" disabled={isVideo} onClick={() => setRotation((value) => (value + 90) % 360)}>
            <RotateRight />
          </IconButton>
          <IconButton color="inherit" onClick={() => setFullscreen((value) => !value)}>
            {fullscreen ? <FullscreenExit /> : <Fullscreen />}
          </IconButton>
          <IconButton color="inherit" onClick={onClose}>
            <Close />
          </IconButton>
        </Stack>
        <Box
          sx={{
            alignItems: 'center',
            display: 'flex',
            flex: 1,
            justifyContent: 'center',
            overflow: zoom > 1 ? 'auto' : 'hidden',
            position: 'relative',
            p: fullscreen ? 2 : 3
          }}
        >
          {hasMany && (
            <IconButton
              color="inherit"
              onClick={() => go(currentIndex - 1)}
              sx={{ bgcolor: 'rgba(15,23,42,0.55)', left: 16, position: 'absolute', top: '50%', zIndex: 2 }}
            >
              <ChevronLeft />
            </IconButton>
          )}
          {current.src && (
            <Box sx={{ alignItems: 'center', display: 'flex', height: '100%', justifyContent: 'center', maxHeight: '100%', maxWidth: '100%', overflow: 'hidden', width: '100%' }}>
              {isVideo ? (
                <Box
                  component="video"
                  controls
                  preload="metadata"
                  src={current.src}
                  sx={{
                    bgcolor: 'common.black',
                    display: 'block',
                    maxHeight: '100%',
                    maxWidth: '100%',
                    objectFit: 'contain'
                  }}
                />
              ) : (
                <Box
                  alt={current.title || title || 'preview'}
                  component="img"
                  src={current.src}
                  sx={{
                    display: 'block',
                    height: zoom === 1 ? '100%' : 'auto',
                    maxHeight: '100%',
                    maxWidth: '100%',
                    objectFit: 'contain',
                    transform: `scale(${zoom}) rotate(${rotation}deg)`,
                    transformOrigin: 'center center',
                    transition: 'transform 120ms ease',
                    width: zoom === 1 ? '100%' : 'auto'
                  }}
                />
              )}
            </Box>
          )}
          {hasMany && (
            <IconButton
              color="inherit"
              onClick={() => go(currentIndex + 1)}
              sx={{ bgcolor: 'rgba(15,23,42,0.55)', position: 'absolute', right: 16, top: '50%', zIndex: 2 }}
            >
              <ChevronRight />
            </IconButton>
          )}
        </Box>
        {hasMany && (
          <Stack direction="row" sx={{ borderTop: '1px solid rgba(255,255,255,0.08)', gap: 1, justifyContent: 'center', p: 1.5 }}>
            <Button color="inherit" onClick={() => go(currentIndex - 1)} startIcon={<ChevronLeft />} variant="outlined">
              上一张
            </Button>
            <Button color="inherit" endIcon={<ChevronRight />} onClick={() => go(currentIndex + 1)} variant="outlined">
              下一张
            </Button>
          </Stack>
        )}
      </Stack>
    </Dialog>
  );
}
