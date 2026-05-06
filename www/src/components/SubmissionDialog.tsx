import { Close } from '@mui/icons-material';
import { Box, Dialog, DialogContent, DialogTitle, IconButton, Stack, Typography } from '@mui/material';
import { type EventCard } from '../api/client';
import { SubmissionForm } from './SubmissionForm';

type SubmissionDialogProps = {
  event: EventCard | null;
  open: boolean;
  onClose: () => void;
  onSubmitted?: () => void;
};

export function SubmissionDialog({ event, onClose, onSubmitted, open }: SubmissionDialogProps) {
  return (
    <Dialog
      fullWidth
      maxWidth="md"
      onClose={(_, reason) => {
        if (reason === 'backdropClick' || reason === 'escapeKeyDown') return;
        onClose();
      }}
      open={open}
      slotProps={{
        paper: { sx: { borderRadius: 3 } }
      }}
    >
      <DialogTitle sx={{ pb: 1 }}>
        <Stack direction="row" sx={{ alignItems: 'flex-start', gap: 2 }}>
          <Box sx={{ flex: 1, minWidth: 0 }}>
            <Typography sx={{ fontWeight: 900 }} variant="h5">
              上传返图
            </Typography>
            <Typography color="text.secondary" sx={{ mt: 0.5 }} variant="body2">
              {event ? `投稿到「${event.title}」；批量选择图片后会按队列逐张提交审核。` : '请选择要投稿的兽聚。'}
            </Typography>
          </Box>
          <IconButton onClick={onClose}>
            <Close />
          </IconButton>
        </Stack>
      </DialogTitle>
      <DialogContent sx={{ pt: 2 }}>
        <SubmissionForm event={event} onRequestClose={onClose} onSubmitted={onSubmitted} showCloseButton />
      </DialogContent>
    </Dialog>
  );
}
