import { ApiError } from './api';
import { t } from './i18n/index.svelte';

/** A user-facing sentence for any failed request. */
export function errorText(err: unknown): string {
  if (!(err instanceof ApiError)) return t('errors.internal');
  switch (err.code) {
    case 'network':
      return t('errors.network');
    case 'xray_rejected':
      return t('errors.xray_rejected', { detail: shorten(String(err.data.detail ?? '')) });
    case 'not_found':
      return t('errors.not_found');
    default:
      return t('errors.internal');
  }
}

/** Field errors of a 422 validation response, already translated. */
export function fieldErrors(err: unknown, prefix: string): Record<string, string> {
  if (!(err instanceof ApiError) || err.code !== 'validation') return {};
  const fields = (err.data.fields ?? {}) as Record<string, string>;
  const detail = (err.data.detail ?? {}) as Record<string, string>;
  const by = detail.by ? t(`${prefix}.by.${detail.by}`) : '';
  const out: Record<string, string> = {};
  for (const [field, code] of Object.entries(fields)) {
    const byText = by.startsWith(`${prefix}.by.`) ? `«${detail.by}»` : by;
    out[field] = t(`${prefix}.errors.${code}`, { by: byText, network: detail.network ?? '' });
  }
  return out;
}

function shorten(s: string): string {
  const line = s.split('\n').filter(Boolean).pop() ?? s;
  const tail = line.split(' > ').pop() ?? line;
  return tail.length > 160 ? tail.slice(0, 157) + '…' : tail;
}
