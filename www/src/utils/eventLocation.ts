import type { EventCard } from '../api/client';

export function formatEventLocation(event: Pick<EventCard, 'cityName' | 'location' | 'provinceName'>) {
  const region = [event.provinceName, event.cityName].filter(Boolean).join(' ');
  const detail = event.location?.trim() || '';
  if (region && detail) return `${region} / ${detail}`;
  return region || detail || '地点待定';
}

export function formatEventRegion(event: Pick<EventCard, 'cityName' | 'provinceName'>) {
  return [event.provinceName, event.cityName].filter(Boolean).join(' ') || '行政区待定';
}
