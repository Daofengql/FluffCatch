import { Box, Typography } from '@mui/material';
import type { ReactNode } from 'react';

type PageHeaderProps = {
  title: string;
  subtitle?: string;
  actions?: ReactNode;
};

export function PageHeader({ title, subtitle, actions }: PageHeaderProps) {
  return (
    <Box sx={{ alignItems: 'center', display: 'flex', justifyContent: 'space-between', mb: 3 }}>
      <Box>
        <Typography component="h1" sx={{ fontWeight: 700 }} variant="h5">
          {title}
        </Typography>
        {subtitle && (
          <Typography color="text.secondary" variant="body2">
            {subtitle}
          </Typography>
        )}
      </Box>
      {actions && <Box sx={{ alignItems: 'center', display: 'flex', gap: 1 }}>{actions}</Box>}
    </Box>
  );
}
