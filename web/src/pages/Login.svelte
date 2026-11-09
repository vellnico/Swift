<script lang="ts">
  import { onMount } from 'svelte';
  import { ApiError } from '../lib/api';
  import { t } from '../lib/i18n/index.svelte';
  import { dur, fadeShift, reducedMotion } from '../lib/motion';
  import { login } from '../lib/session.svelte';
  import Button from '../lib/ui/Button.svelte';
  import Input from '../lib/ui/Input.svelte';
  import Logo from '../lib/ui/Logo.svelte';

  let { onsuccess }: { onsuccess: (logoRect: DOMRect | null) => void } = $props();

  let username = $state('');
  let password = $state('');
  let error = $state('');
  let status = $state<'idle' | 'loading'>('idle');
  let shaking = $state(false);
  let logoEl = $state<HTMLElement>();
  let userInput = $state<HTMLInputElement>();

  onMount(() => userInput?.focus());

  function shake() {
    if (reducedMotion()) return;
    shaking = false;
    requestAnimationFrame(() => (shaking = true));
    setTimeout(() => (shaking = false), dur.slow);
  }

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    if (status === 'loading') return;
    error = '';
    status = 'loading';
    try {
      await login(username.trim(), password);
      onsuccess(logoEl?.getBoundingClientRect() ?? null);
    } catch (err) {
      const code = err instanceof ApiError ? err.code : 'internal';
      if (code === 'rate_limited') {
        const sec = Number((err as ApiError).data.retry_after ?? 900);
        error = t('login.error.rate_limited', { min: Math.max(1, Math.ceil(sec / 60)) });
      } else if (code === 'invalid_credentials') {
        error = t('login.error.invalid');
      } else if (code === 'network') {
        error = t('login.error.network');
      } else {
        error = t('login.error.internal');
      }
      status = 'idle';
      shake();
    }
  }
</script>

<div class="screen">
  <div class="center">
    <div class="brand" bind:this={logoEl}>
      <Logo size={40} gleam />
    </div>
    <form class="form" class:shaking onsubmit={submit} novalidate out:fadeShift={{ y: 24, duration: dur.base }}>
      <Input
        bind:input={userInput}
        label={t('login.username')}
        bind:value={username}
        autocomplete="username"
        autocapitalize="none"
        spellcheck="false"
        required
      />
      <Input
        label={t('login.password')}
        type="password"
        bind:value={password}
        autocomplete="current-password"
        error={error || undefined}
        required
      />
      <Button type="submit" variant="primary" wide status={status}>{t('login.submit')}</Button>
    </form>
  </div>
</div>

<style>
  .screen {
    position: fixed;
    inset: 0;
    z-index: 5;
    overflow-y: auto;
    display: grid;
    place-items: center;
    padding: var(--s6) var(--s4);
  }
  .center {
    width: min(360px, 100%);
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--s12);
  }
  .brand { display: inline-flex; }
  .form {
    width: 100%;
    display: flex;
    flex-direction: column;
    gap: var(--s5);
  }
  .form :global(button[type='submit']) { margin-top: var(--s2); }
  .shaking { animation: shake var(--dur-slow) var(--ease-out); }
  @keyframes shake {
    15% { transform: translateX(-8px); }
    35% { transform: translateX(7px); }
    55% { transform: translateX(-5px); }
    75% { transform: translateX(3px); }
    100% { transform: translateX(0); }
  }
</style>
