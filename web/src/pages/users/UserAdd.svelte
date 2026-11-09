<script lang="ts">
  import { errorText, fieldErrors } from '../../lib/errors';
  import { t } from '../../lib/i18n/index.svelte';
  import { createUser, type User } from '../../lib/users.svelte';
  import Button from '../../lib/ui/Button.svelte';
  import Input from '../../lib/ui/Input.svelte';
  import Modal from '../../lib/ui/Modal.svelte';

  let { open = $bindable(false), oncreated }: { open?: boolean; oncreated: (u: User) => void } = $props();
  let name = $state('');
  let error = $state('');
  let status = $state<'idle' | 'loading'>('idle');

  $effect(() => {
    if (open) {
      name = '';
      error = '';
    }
  });

  async function submit(e?: SubmitEvent) {
    e?.preventDefault();
    if (status === 'loading') return;
    error = '';
    status = 'loading';
    try {
      const u = await createUser(name);
      open = false;
      oncreated(u);
    } catch (err) {
      error = fieldErrors(err, 'users').name ?? errorText(err);
    } finally {
      status = 'idle';
    }
  }
</script>

<Modal bind:open title={t('users.add_title')}>
  <form onsubmit={submit} novalidate>
    <p class="muted hint">{t('users.add_hint')}</p>
    <Input label={t('users.name')} placeholder={t('users.name_placeholder')} bind:value={name} error={error || undefined} autocomplete="off" maxlength={64} />
    <button type="submit" class="sr-only" aria-hidden="true" tabindex="-1"></button>
  </form>
  {#snippet footer()}
    <Button variant="quiet" onclick={() => (open = false)}>{t('common.cancel')}</Button>
    <Button variant="primary" status={status} onclick={() => submit()}>{t('users.add')}</Button>
  {/snippet}
</Modal>

<style>
  .hint { margin-bottom: var(--s5); }
</style>
