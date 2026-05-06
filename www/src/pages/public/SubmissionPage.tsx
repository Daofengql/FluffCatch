import { Alert, Box, Button, Paper, Stack, TextField, Typography } from '@mui/material';
import { type FormEvent, useState } from 'react';
import { useParams } from 'react-router-dom';
import { submitPhotos } from '../../api/client';

export function SubmissionPage() {
  const eventId = Number(useParams().eventId);
  const [message, setMessage] = useState('');
  const [error, setError] = useState('');

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError('');
    setMessage('');
    const formData = new FormData(event.currentTarget);
    try {
      const result = await submitPhotos(eventId, formData);
      setMessage(`已提交 ${result.submissions.length} 张图片，等待管理员审核。`);
      event.currentTarget.reset();
    } catch (err) {
      setError(err instanceof Error ? err.message : '上传失败');
    }
  }

  return (
    <Box sx={{ alignItems: 'center', display: 'flex', justifyContent: 'center', width: '100%' }}>
      <Paper component="form" elevation={3} onSubmit={handleSubmit} sx={{ maxWidth: 680, p: { xs: 3, sm: 4 }, width: '100%' }}>
        <Stack sx={{ gap: 2 }}>
          <Box>
            <Typography sx={{ fontWeight: 800 }} variant="h4">
              上传返图
            </Typography>
            <Typography color="text.secondary" sx={{ mt: 1 }}>
              输入投稿口令后选择图片，管理员审核通过后会进入正式画廊。
            </Typography>
          </Box>
          {message && <Alert severity="success">{message}</Alert>}
          {error && <Alert severity="error">{error}</Alert>}
          <TextField label="投稿口令" name="submissionPassword" type="password" />
          <TextField label="摄影师署名（可选）" name="photographerName" />
          <TextField label="标签（空格或逗号分隔）" name="tags" placeholder="#合照 #舞台" />
          <Button component="label" variant="outlined">
            选择图片
            <input hidden multiple name="files" type="file" />
          </Button>
          <Button type="submit" variant="contained">
            提交审核
          </Button>
        </Stack>
      </Paper>
    </Box>
  );
}
