import { CircularProgress, Stack } from '@mui/material';
import { lazy, Suspense } from 'react';
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom';
import { ErrorBoundary } from './components/common/ErrorBoundary';
import { AdminLayout } from './components/layout/AdminLayout';
import { PublicLayout } from './components/layout/PublicLayout';

const AdminEventsPage = lazy(() => import('./pages/admin/AdminEventsPage').then((module) => ({ default: module.AdminEventsPage })));
const AdminSettingsPage = lazy(() => import('./pages/admin/AdminSettingsPage').then((module) => ({ default: module.AdminSettingsPage })));
const EventDetailPage = lazy(() => import('./pages/public/EventDetailPage').then((module) => ({ default: module.EventDetailPage })));
const HomePage = lazy(() => import('./pages/public/HomePage').then((module) => ({ default: module.HomePage })));
const LoginPage = lazy(() => import('./pages/public/LoginPage').then((module) => ({ default: module.LoginPage })));
const SubmitHubPage = lazy(() => import('./pages/public/SubmitHubPage').then((module) => ({ default: module.SubmitHubPage })));

function RouteFallback() {
  return (
    <Stack sx={{ alignItems: 'center', justifyContent: 'center', minHeight: 320 }}>
      <CircularProgress />
    </Stack>
  );
}

function App() {
  return (
    <BrowserRouter>
      <ErrorBoundary>
        <Suspense fallback={<RouteFallback />}>
          <Routes>
            <Route element={<PublicLayout />}>
              <Route index element={<HomePage />} />
              <Route path="/submit" element={<SubmitHubPage />} />
              <Route path="/events/:eventId" element={<EventDetailPage />} />
              <Route path="/login" element={<LoginPage />} />
            </Route>

            <Route path="/admin" element={<AdminLayout />}>
              <Route index element={<Navigate replace to="/admin/events" />} />
              <Route path="dashboard" element={<Navigate replace to="/admin/events" />} />
              <Route path="events" element={<AdminEventsPage />} />
              <Route path="submissions" element={<Navigate replace to="/admin/events" />} />
              <Route path="settings" element={<Navigate replace to="/admin/settings/site" />} />
              <Route path="settings/:section" element={<AdminSettingsPage />} />
            </Route>

            <Route path="*" element={<Navigate replace to="/" />} />
          </Routes>
        </Suspense>
      </ErrorBoundary>
    </BrowserRouter>
  );
}

export default App;
