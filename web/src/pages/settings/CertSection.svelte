<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError } from '../../lib/api';
  import { errorText } from '../../lib/errors';
  import { i18n, t } from '../../lib/i18n/index.svelte';
  import Alert from '../../lib/ui/Alert.svelte';
  import Badge from '../../lib/ui/Badge.svelte';
  import Button from '../../lib/ui/Button.svelte';
  import Card from '../../lib/ui/Card.svelte';
  import CopyField from '../../lib/ui/CopyField.svelte';
  import Input from '../../lib/ui/Input.svelte';
  import Modal from '../../lib/ui/Modal.svelte';
  import Skeleton from '../../lib/ui/Skeleton.svelte';
  import { toast } from '../../lib/ui/toasts.svelte';

  interface Cert {
    kind: 'acme' | 'self_signed';
    domain: string;
    sha256: string;
    notAfter: string;
    lastError: string;
  }

  let cert = $state<Cert | null>(null);
  let domain = $state('');
  let domainError = $state('');
  let confirm = $state<'domain' | 'ip' | null>(null);
  let confirmOpen = $state(false);
  let status = $state<'idle' | 'loading' | 'done'>('idle');
  let renewing = $state<'idle' | 'loading' | 'done'>('idle');

  onMount(() => {
    api<Cert>('certs').then((c) => (cert = c)).catch((e) => toast(errorText(e)));
  });

  const expires = $derived(cert ? new Date(cert.notAfter).toLocaleDateString(i18n.locale === 'ru' ? 'ru-RU' : 'en-US', { day: 'numeric', month: 'long', year: 'numeric' }) : '');

  function errorFor(e: unknown): string {
    if (e instanceof ApiError) {
      if (e.code === 'validation') {
        const code = (e.data.fields as Record<string, string>)?.domain ?? 'invalid';
        const d = (e.data.detail ?? {}) as Record<string, string>;
        return t(`certs.errors.${code}`, { ips: d.ips ?? '', server: d.server ?? '' });
      }
      if (e.code === 'acme_failed') return t('certs.errors.acme_failed', { detail: String(e.data.detail ?? '') });
    }
    return errorText(e);
  }

  function ask(mode: 'domain' | 'ip') {
    if (mode === 'domain' && !domain.trim()) {
      domainError = t('certs.errors.invalid');
      return;
    }
    domainError = '';
    confirm = mode;
    confirmOpen = true;
  }

  async function run() {
    const mode = confirm;
    confirmOpen = false;
    status = 'loading';
    try {
      cert = mode === 'domain'
        ? await api<Cert>('certs/acme', { method: 'POST', body: { domain: domain.trim() } })
        : await api<Cert>('certs/self-signed', { method: 'POST' });
      status = 'done';
      toast(mode === 'domain' ? t('certs.issued') : t('certs.switched_ip'));
      domain = '';
      setTimeout(() => (status = 'idle'), 1600);
    } catch (e) {
      status = 'idle';
      if (mode === 'domain') domainError = errorFor(e);
      else toast(errorFor(e));
    }
  }

  async function renew() {
    renewing = 'loading';
    try {
      cert = await api<Cert>('certs/renew', { method: 'POST' });
      renewing = 'done';
      toast(t('certs.renewed'));
      setTimeout(() => (renewing = 'idle'), 1600);
    } catch (e) {
      renewing = 'idle';
      toast(errorFor(e));
      cert = await api<Cert>('certs');
    }
  }
</script>

<Card>
  <div class="head">
    <h2 class="t-heading">{t('settings.cert')}</h2>
    {#if cert}<Badge variant={cert.kind === 'acme' ? 'recommended' : 'default'}>{t(`certs.kind.${cert.kind}`)}</Badge>{/if}
  </div>
  {#if !cert}
    <div class="stack"><Skeleton width="50%" height="18px" /><Skeleton height="40px" /></div>
  {:else}
    <div class="stack">
      {#if cert.lastError}<Alert kind="warning">{t('dash.cert_error', { error: cert.lastError })}</Alert>{/if}
      <dl class="facts">
        <dt class="t-small muted">{cert.kind === 'acme' ? t('certs.domain') : t('certs.name')}</dt>
        <dd class="t-code">{cert.domain}</dd>
        <dt class="t-small muted">{t('certs.valid_until')}</dt>
        <dd>{expires}</dd>
      </dl>
      {#if cert.kind === 'self_signed'}
        <CopyField label={t('certs.fingerprint')} value={cert.sha256} />
        <p class="t-small muted note">{t('certs.self_signed_note')}</p>
        <form class="issue" onsubmit={(e) => (e.preventDefault(), ask('domain'))} novalidate>
          <Input label={t('certs.domain_label')} hint={t('certs.domain_hint')} placeholder="vpn.example.com" bind:value={domain} mono autocomplete="off" spellcheck="false" error={domainError || undefined} />
          <div><Button type="submit" variant="secondary" status={status}>{t('certs.issue')}</Button></div>
        </form>
      {:else}
        <p class="t-small muted note">{t('certs.acme_note')}</p>
        <div class="actions">
          <Button variant="secondary" status={renewing} onclick={renew}>{t('certs.renew')}</Button>
          <Button variant="quiet" status={status} onclick={() => ask('ip')}>{t('certs.to_ip')}</Button>
        </div>
      {/if}
    </div>
  {/if}
</Card>

<Modal bind:open={confirmOpen} title={t('certs.confirm_title')}>
  <p class="muted">{confirm === 'domain' ? t('certs.confirm_domain', { domain: domain.trim() }) : t('certs.confirm_ip')}</p>
  {#snippet footer()}
    <Button variant="quiet" onclick={() => (confirmOpen = false)}>{t('common.cancel')}</Button>
    <Button variant="primary" onclick={run}>{t('certs.confirm')}</Button>
  {/snippet}
</Modal>

<style>
  .head { display: flex; align-items: center; justify-content: space-between; gap: var(--s3); margin-bottom: var(--s4); }
  .stack { display: flex; flex-direction: column; gap: var(--s4); }
  .facts { display: grid; grid-template-columns: max-content 1fr; gap: var(--s2) var(--s6); margin: 0; }
  .facts dd { margin: 0; min-width: 0; overflow-wrap: anywhere; }
  .note { max-width: 64ch; }
  .issue { display: flex; flex-direction: column; gap: var(--s3); padding-top: var(--s4); border-top: 1px solid var(--line); }
  .actions { display: flex; gap: var(--s3); flex-wrap: wrap; }
</style>
