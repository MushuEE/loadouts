/**
 * Built-in loadout covers, served from public/covers.
 *
 * Stored on the loadout as a root-relative cover_image_url, exactly like a custom URL, so
 * the backend needs no notion of "preset" and a future upload lands in the same field.
 * They ship with the app rather than being hotlinked, so a cover never breaks or leaks
 * who is looking at it.
 */
export interface CoverPreset {
  url: string;
  label: string;
}

export const COVER_PRESETS: CoverPreset[] = [
  { url: '/covers/alpine.svg', label: 'Alpine' },
  { url: '/covers/forest.svg', label: 'Forest' },
  { url: '/covers/desert.svg', label: 'Desert' },
  { url: '/covers/coast.svg', label: 'Coast' },
  { url: '/covers/snow.svg', label: 'Snow' },
  { url: '/covers/night-camp.svg', label: 'Night camp' },
  { url: '/covers/trail-run.svg', label: 'Trail run' },
  { url: '/covers/cycling.svg', label: 'Cycling' },
];

/** Mirrors backend core.ValidateImageURL, so a bad custom URL fails before the round trip. */
export function isAcceptableImageURL(raw: string): boolean {
  if (raw === '') return true;
  if (raw.startsWith('/') && !raw.startsWith('//')) return !/[\\\r\n]/.test(raw);
  try {
    const u = new URL(raw);
    return (u.protocol === 'http:' || u.protocol === 'https:') && u.host !== '';
  } catch {
    return false;
  }
}
