<script lang="ts">
  import '../src/lib/styles/fonts.css';
  import '../src/lib/styles/tokens.css';
  import '../src/lib/styles/base.css';
  import { buzz, copyText } from '../src/lib/clipboard';
  import { unblur, fadeShift, stagger } from '../src/lib/motion';
  import Logo from '../src/lib/ui/Logo.svelte';
  import { appsFor, platformOf, tr, type PageData } from './data';

  let { data }: { data: PageData } = $props();
  const s = (key: string, params: Record<string, string> = {}) => tr(data.strings, key, params);

  const platform = platformOf(navigator.userAgent);
  const apps = appsFor(platform);
  let copied = $state(false);
  let showLinks = $state(false);
  let copiedLink = $state(-1);

  async function copy(text: string, index = -1) {
    if (!(await copyText(text))) return;
    buzz();
    if (index < 0) {
      copied = true;
      setTimeout(() => (copied = false), 1600);
    } else {
      copiedLink = index;
      setTimeout(() => (copiedLink = -1), 1600);
    }
  }
</script>

<main class="page">
  <header class="head" in:fadeShift>
    <Logo size={24} gleam />
    <h1 class="t-title">{data.name ? s('sub.for', { name: data.name }) : s('sub.title')}</h1>
  </header>

  {#if !data.enabled}
    <section class="card" in:fadeShift={{ delay: 60 }}>
      <p class="muted">{s('sub.disabled')}</p>
    </section>
  {:else}
    <section class="card qr-card" in:fadeShift={{ delay: 60 }}>
      <div class="qr" in:unblur>
        <img src="?format=qr" alt="QR" width="240" height="240" />
      </div>
      <p class="t-small muted center">{s('sub.scan')}</p>
      <button type="button" class="btn primary" class:done={copied} onclick={() => copy(data.url)}>
        {copied ? s('sub.copied') : s('sub.copy')}
      </button>
    </section>

    <section class="card" in:fadeShift={{ delay: 120 }}>
      <h2 class="t-heading">{s('sub.apps')}</h2>
      <ul class="apps">
        {#each apps as app, i (app.id)}
          <li in:fadeShift={{ delay: 160 + stagger(i) }}>
            <a class="btn" class:primary-outline={i === 0} href={app.link(data.url, 'Swift')}>{s('sub.add', { app: app.name })}</a>
            <a class="get" href={app.download} target="_blank" rel="noopener noreferrer">{s('sub.get', { app: app.name })}</a>
          </li>
        {/each}
      </ul>
    </section>

    {#if data.links.length}
      <section class="card">
        <button type="button" class="disclosure" aria-expanded={showLinks} onclick={() => (showLinks = !showLinks)}>
          <span class="t-heading">{s('sub.links')}</span>
          <span class="chev" class:open={showLinks} aria-hidden="true">
            <svg width="18" height="18" viewBox="0 0 24 24"><path d="M6 9l6 6 6-6" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" /></svg>
          </span>
        </button>
        {#if showLinks}
          <p class="t-small muted hint">{s('sub.links_hint')}</p>
          <ul class="links">
            {#each data.links as l, i (l.url)}
              <li>
                <span class="link-name">{l.name}</span>
                <button type="button" class="code" class:flash={copiedLink === i} onclick={() => copy(l.url, i)}>{l.url}</button>
              </li>
            {/each}
          </ul>
        {/if}
      </section>
    {/if}

    <section class="card">
      <h2 class="t-heading">{s('sub.v2raytun.title')}</h2>
      <ol class="steps t-small muted">
        <li>{s('sub.v2raytun.step1')}</li>
        <li>{s('sub.v2raytun.step2')}</li>
        <li>{s('sub.v2raytun.step3')}</li>
        <li>{s('sub.v2raytun.step4')}</li>
      </ol>
    </section>
  {/if}
</main>

<style>
  .page {
    width: min(480px, 100%);
    margin: 0 auto;
    padding: var(--s10) var(--s4) var(--s16);
    display: flex;
    flex-direction: column;
    gap: var(--s4);
  }
  .head { display: flex; flex-direction: column; gap: var(--s6); margin-bottom: var(--s4); }
  .card {
    background: var(--surface-1);
    box-shadow: var(--edge);
    border-radius: var(--radius-surface);
    padding: var(--s5);
    display: flex;
    flex-direction: column;
    gap: var(--s4);
  }
  .qr-card { align-items: center; }
  .qr {
    padding: var(--s4);
    border-radius: var(--radius-surface);
    background: var(--qr-light);
  }
  .qr img { width: min(240px, 60vw); height: auto; display: block; image-rendering: pixelated; }
  .center { text-align: center; max-width: 36ch; }
  .btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 100%;
    height: var(--control-h);
    padding: 0 var(--s4);
    border: 0;
    border-radius: var(--radius-control);
    background: transparent;
    box-shadow: inset 0 0 0 1px var(--line-strong);
    color: var(--text-1);
    font: var(--t-body);
    font-weight: 600;
    text-decoration: none;
    transition: transform var(--dur-fast) var(--ease-out);
  }
  .btn:active { transform: scale(0.97); }
  .btn.primary { background: var(--accent); color: var(--accent-fg); box-shadow: none; }
  .btn.primary-outline { box-shadow: var(--edge-lit); }
  .btn:focus-visible { box-shadow: var(--focus-ring); }
  .apps { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: var(--s3); }
  .apps li { display: flex; flex-direction: column; gap: var(--s2); align-items: center; }
  .get { font: var(--t-small); color: var(--text-2); }
  .get:hover { color: var(--text-1); }
  .disclosure {
    display: flex;
    justify-content: space-between;
    align-items: center;
    width: 100%;
    padding: 0;
    border: 0;
    background: none;
    color: var(--text-1);
    text-align: left;
  }
  .chev { color: var(--text-2); display: grid; transition: transform var(--dur-base) var(--ease-out); }
  .chev.open { transform: rotate(180deg); }
  .hint { margin-top: calc(var(--s2) * -1); }
  .links { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: var(--s3); }
  .links li { display: flex; flex-direction: column; gap: var(--s1); }
  .link-name { font: var(--t-small); font-weight: 500; }
  .code {
    width: 100%;
    padding: var(--s2) var(--s3);
    border: 0;
    border-radius: var(--radius-control);
    background: var(--surface-2);
    box-shadow: inset 0 0 0 1px var(--line);
    color: var(--text-2);
    font: var(--t-code);
    text-align: left;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .code.flash { box-shadow: var(--edge-lit); color: var(--text-1); }
  .steps { margin: 0; padding-left: var(--s5); display: flex; flex-direction: column; gap: var(--s2); }
</style>
