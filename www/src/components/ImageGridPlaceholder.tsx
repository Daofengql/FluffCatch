import { Box, Paper, Stack, Typography } from '@mui/material';

const slots = Array.from({ length: 8 }, (_, index) => index + 1);

export function ImageGridPlaceholder() {
  return (
    <Box
      sx={{
        display: 'grid',
        gap: 2,
        gridTemplateColumns: {
          xs: '1fr 1fr',
          md: 'repeat(4, 1fr)'
        }
      }}
    >
      {slots.map((slot) => (
        <Paper
          key={slot}
          sx={{
            alignItems: 'center',
            aspectRatio: '1 / 1',
            background: 'rgba(255,255,255,0.72)',
            border: '1px dashed rgba(124,58,237,0.32)',
            display: 'flex',
            justifyContent: 'center'
          }}
        >
          <Stack sx={{ alignItems: 'center' }}>
            <Typography color="primary" sx={{ fontWeight: 800 }}>
              #{slot}
            </Typography>
            <Typography color="text.secondary" variant="body2">
              待接入图片
            </Typography>
          </Stack>
        </Paper>
      ))}
    </Box>
  );
}
