import { useEffect, useState } from 'react';

export function usePersistentState<T extends string>(key: string, fallback: T, allowedValues: readonly T[]) {
  const [value, setValue] = useState<T>(() => {
    try {
      const stored = window.localStorage.getItem(key) as T | null;
      return stored && allowedValues.includes(stored) ? stored : fallback;
    } catch {
      return fallback;
    }
  });

  useEffect(() => {
    try {
      window.localStorage.setItem(key, value);
    } catch {
      // localStorage may be unavailable in private or restricted browser contexts.
    }
  }, [key, value]);

  return [value, setValue] as const;
}
