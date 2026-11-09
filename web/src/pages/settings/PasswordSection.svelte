<script lang="ts">
  import { api, ApiError } from '../../lib/api';
  import { errorText } from '../../lib/errors';
  import { t } from '../../lib/i18n/index.svelte';
  import Button from '../../lib/ui/Button.svelte';
  import Card from '../../lib/ui/Card.svelte';
  import Input from '../../lib/ui/Input.svelte';
  import { toast } from '../../lib/ui/toasts.svelte';

  let current = $state('');
  let next = $state('');
  let errors = $state<{ current?: string; next?: string }>({});
  let status = $state<'idle' | 'loading' | 'done'>('idle');

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    errors = {};
    if ([...next].length < 12) {
      errors = { next: t('settings.errors.password_too_short') };
      return;
    }
    status = 'loading';
    try {
      await api('auth/password', { method: 'POST', body: { current, next } });
      current = next = '';
      status = 'done';
      toast(t('settings.password_changed'));
      setTimeout(() => (status = 'idle'), 1600);
    } catch (err) {
      status = 'idle';
      if (err instanceof ApiError && err.code === 'wrong_password') errors = { current: t('settings.errors.wrong_password') };
      else if (err instanceof ApiError && err.code === 'password_too_short') errors = { next: t('settings.errors.password_too_short') };
      else toast(errorText(err));
    }
  }
</script>

<Card>
  <h2 class="t-heading">{t('settings.password')}</h2>
  <form onsubmit={submit} novalidate>
    <input type="text" name="username" autocomplete="username" hidden />
    <div class="grid">
      <Input label={t('settings.password_current')} type="password" autocomplete="current-password" bind:value={current} error={errors.current} />
      <Input label={t('settings.password_next')} hint={t('settings.password_hint')} type="password" autocomplete="new-password" bind:value={next} error={errors.next} />
    </div>
    <div><Button type="submit" variant="secondary" {status} disabled={!current || !next}>{t('settings.password_change')}</Button></div>
  </form>
</Card>

<style>
  h2 { margin-bottom: var(--s4); }
  form { display: flex; flex-direction: column; gap: var(--s4); }
  .grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--s4); }
  @media (max-width: 599px) {
    .grid { grid-template-columns: 1fr; }
  }
</style>
