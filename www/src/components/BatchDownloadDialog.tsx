import { Cancel, CheckCircle, CloudDownload, Download, RadioButtonUnchecked } from '@mui/icons-material';
import { Box, Button, Dialog, DialogActions, DialogContent, DialogTitle, LinearProgress, List, ListItem, ListItemIcon, ListItemText, Typography } from '@mui/material';
import { useRef, useState } from 'react';
import { downloadPhotosAsZip } from '../utils/download';

type DownloadItem = { url: string; filename: string };

type BatchDownloadDialogProps = {
  open: boolean;
  items: DownloadItem[];
  zipName: string;
  onClose: () => void;
};

type ItemStatus = 'pending' | 'downloading' | 'done';

export function BatchDownloadDialog({ open, items, zipName, onClose }: BatchDownloadDialogProps) {
  const [started, setStarted] = useState(false);
  const [currentIndex, setCurrentIndex] = useState(0);
  const [currentName, setCurrentName] = useState('');
  const [error, setError] = useState('');
  const abortRef = useRef<AbortController | null>(null);

  const total = items.length;
  const done = currentIndex;

  function handleClose() {
    abortRef.current?.abort();
    setStarted(false);
    setCurrentIndex(0);
    setCurrentName('');
    setError('');
    onClose();
  }

  async function handleStart() {
    setStarted(true);
    setError('');
    const controller = new AbortController();
    abortRef.current = controller;

    try {
      await downloadPhotosAsZip(
        items,
        zipName,
        (current, _total, name) => {
          setCurrentIndex(current);
          setCurrentName(name);
        },
        controller.signal,
      );
      handleClose();
    } catch (err) {
      if (err instanceof DOMException && err.name === 'AbortError') {
        return;
      }
      setError(err instanceof Error ? err.message : '下载失败');
    }
  }

  const statusIcon = (index: number, _name: string): React.ReactNode => {
    if (index < done) return <CheckCircle color="success" fontSize="small" />;
    if (index === done && started && !error) return <CloudDownload color="primary" fontSize="small" />;
    return <RadioButtonUnchecked sx={{ color: 'text.disabled' }} fontSize="small" />;
  };

  return (
    <Dialog fullWidth maxWidth="sm" onClose={handleClose} open={open}>
      <DialogTitle>批量下载</DialogTitle>
      <DialogContent dividers>
        {started && !error && (
          <Box sx={{ mb: 2 }}>
            <Typography color="text.secondary" sx={{ mb: 1 }} variant="body2">
              正在下载: {done} / {total}
            </Typography>
            <LinearProgress sx={{ height: 8, borderRadius: 4 }} value={(done / total) * 100} variant="determinate" />
          </Box>
        )}
        {error && (
          <Typography color="error" sx={{ mb: 2 }} variant="body2">{error}</Typography>
        )}
        <List dense disablePadding>
          {items.map((item, index) => (
            <ListItem key={item.filename} disableGutters>
              <ListItemIcon sx={{ minWidth: 36 }}>
                {statusIcon(index, item.filename)}
              </ListItemIcon>
              <ListItemText
                primary={item.filename}
                slotProps={{
                  primary: {
                    color: index < done ? 'text.secondary' : 'text.primary',
                    noWrap: true,
                    sx: { textDecoration: index < done ? 'line-through' : undefined },
                    variant: 'body2',
                  },
                }}
              />
            </ListItem>
          ))}
        </List>
      </DialogContent>
      <DialogActions>
        <Button onClick={handleClose} startIcon={<Cancel />}>
          {started ? '取消' : '关闭'}
        </Button>
        {!started && (
          <Button onClick={handleStart} startIcon={<Download />} variant="contained">
            开始下载 ({total} 张)
          </Button>
        )}
      </DialogActions>
    </Dialog>
  );
}
