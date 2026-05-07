import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom';
import { AdminLayout } from './components/layout/AdminLayout';
import { PublicLayout } from './components/layout/PublicLayout';
import { AdminEventsPage } from './pages/admin/AdminEventsPage';
import { AdminSettingsPage } from './pages/admin/AdminSettingsPage';
import { EventDetailPage } from './pages/public/EventDetailPage';
import { HomePage } from './pages/public/HomePage';
import { LoginPage } from './pages/public/LoginPage';
import { SubmitHubPage } from './pages/public/SubmitHubPage';

function App() {
  return (
    <BrowserRouter>
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
    </BrowserRouter>
  );
}

export default App;
