<script lang="ts">
  import Plus from '@lucide/svelte/icons/plus';
  import Trash from '@lucide/svelte/icons/trash-2';
  import { t } from '../lib/i18n/index.svelte';
  import Alert from '../lib/ui/Alert.svelte';
  import Badge from '../lib/ui/Badge.svelte';
  import Button from '../lib/ui/Button.svelte';
  import Card from '../lib/ui/Card.svelte';
  import CopyField from '../lib/ui/CopyField.svelte';
  import EmptyState from '../lib/ui/EmptyState.svelte';
  import IconButton from '../lib/ui/IconButton.svelte';
  import Input from '../lib/ui/Input.svelte';
  import Logo from '../lib/ui/Logo.svelte';
  import Modal from '../lib/ui/Modal.svelte';
  import Select from '../lib/ui/Select.svelte';
  import Skeleton from '../lib/ui/Skeleton.svelte';
  import Spinner from '../lib/ui/Spinner.svelte';
  import StatusDot from '../lib/ui/StatusDot.svelte';
  import Switch from '../lib/ui/Switch.svelte';
  import Tooltip from '../lib/ui/Tooltip.svelte';
  import { toast } from '../lib/ui/toasts.svelte';
  import Page from '../shell/Page.svelte';

  let name = $state('');
  let port = $state(8443);
  let fp = $state('chrome');
  let on = $state(true);
  let off = $state(false);
  let modal = $state(false);
  let status = $state<'idle' | 'loading' | 'done'>('idle');
  let logoKey = $state(0);

  function confirm() {
    status = 'loading';
    setTimeout(() => (status = 'done'), 900);
    setTimeout(() => (status = 'idle'), 2600);
  }
</script>

<Page title={t('kit.title')}>
  {#snippet action()}
    <Button variant="primary" status={status} onclick={confirm}>{t('kit.confirm')}</Button>
  {/snippet}

  <section>
    <h2 class="t-heading">{t('kit.type')}</h2>
    <div class="grid">
      <Card>
        <p class="t-hero">24%</p>
        <p class="t-small muted">{t('kit.hero_caption')}</p>
      </Card>
      <Card>
        <p class="t-title">{t('nav.profiles')}</p>
        <p class="t-heading">{t('kit.card_title')}</p>
        <p class="t-body">{t('kit.card_body')}</p>
        <p class="t-small muted">{t('kit.input_hint')}</p>
        <p class="t-code muted">a3f1c9e2-7b4d-4e8a-9c1f-2d6b8e0a5f37</p>
      </Card>
    </div>
  </section>

  <section>
    <h2 class="t-heading">{t('kit.gleam')}</h2>
    <Card>
      <div class="row">
        {#key logoKey}<Logo size={32} gleam />{/key}
        <Button variant="quiet" onclick={() => logoKey++}>{t('kit.gleam_run')}</Button>
      </div>
    </Card>
  </section>

  <section>
    <h2 class="t-heading">{t('kit.buttons')}</h2>
    <Card>
      <div class="row">
        <Button variant="primary" status={status} onclick={confirm}>{t('kit.primary')}</Button>
        <Button variant="secondary">{t('kit.secondary')}</Button>
        <Button variant="quiet">{t('kit.quiet')}</Button>
        <Button variant="secondary" status="loading">{t('kit.loading')}</Button>
        <Button variant="secondary" disabled>{t('kit.disabled')}</Button>
        <Button variant="secondary">
          {#snippet icon()}<Plus size={18} strokeWidth={1.5} />{/snippet}
          {t('common.create')}
        </Button>
        <IconButton label={t('common.create')} variant="secondary"><Plus size={18} strokeWidth={1.5} /></IconButton>
        <IconButton label={t('common.close')}><Trash size={18} strokeWidth={1.5} /></IconButton>
      </div>
    </Card>
  </section>

  <section>
    <h2 class="t-heading">{t('kit.inputs')}</h2>
    <Card>
      <div class="grid">
        <Input label={t('kit.input_label')} placeholder={t('kit.input_placeholder')} hint={t('kit.input_hint')} bind:value={name} />
        <Input label={t('kit.input_label')} value="Мама" error={t('kit.input_error')} />
        <Input label={t('kit.port_label')} type="number" mono hint={t('kit.port_hint')} bind:value={port} recommended={443} />
        <Select
          label={t('kit.select_label')}
          hint={t('kit.select_hint')}
          bind:value={fp}
          recommended="chrome"
          options={[
            { value: 'chrome', label: 'Chrome' },
            { value: 'firefox', label: 'Firefox' },
            { value: 'safari', label: 'Safari' },
          ]}
        />
      </div>
    </Card>
  </section>

  <section>
    <h2 class="t-heading">{t('kit.switches')}</h2>
    <Card>
      <div class="row">
        <Switch bind:checked={on} label={t('kit.switch_label')} showLabel />
        <Switch bind:checked={off} label={t('kit.switch_label')} />
        <Switch checked={true} disabled label={t('kit.disabled')} />
      </div>
    </Card>
  </section>

  <section>
    <h2 class="t-heading">{t('kit.cards')}</h2>
    <div class="grid">
      {#each [0, 1, 2] as i (i)}
        <Card interactive>
          <div class="card-head">
            <p class="t-heading">{t('kit.card_title')}</p>
            <Switch checked={i !== 1} label={t('kit.switch_label')} />
          </div>
          <p class="t-small muted">{t('kit.card_body')}</p>
          <div class="row tight">
            <Badge>{t('kit.badge_tcp')}</Badge>
            <Badge><span class="t-code">443</span></Badge>
            {#if i === 0}<Badge variant="recommended">{t('common.recommended')}</Badge>{/if}
          </div>
        </Card>
      {/each}
    </div>
  </section>

  <section>
    <h2 class="t-heading">{t('kit.badges')}</h2>
    <Card>
      <div class="row">
        <Badge>{t('kit.badge_tcp')}</Badge>
        <Badge variant="recommended">{t('common.recommended')}</Badge>
        <Badge variant="solid">26.10.31</Badge>
        <Tooltip text={t('kit.tooltip_text')}>
          {#snippet children({ describedby })}
            <Button variant="quiet" aria-describedby={describedby}>{t('kit.tooltip_trigger')}</Button>
          {/snippet}
        </Tooltip>
      </div>
    </Card>
  </section>

  <section>
    <h2 class="t-heading">{t('kit.feedback')}</h2>
    <div class="grid">
      <Alert title={t('kit.error_title')}>{t('kit.error_body')}</Alert>
      <Alert kind="warning">{t('kit.warning_body')}</Alert>
    </div>
    <div class="row">
      <Button
        variant="secondary"
        onclick={() => toast(t('kit.toast_text'), { action: { label: t('common.undo'), run: () => {} } })}
      >
        {t('kit.toast_show')}
      </Button>
      <Button variant="secondary" onclick={() => (modal = true)}>{t('kit.modal_open')}</Button>
    </div>
  </section>

  <section>
    <h2 class="t-heading">{t('kit.status')}</h2>
    <Card>
      <div class="row">
        <StatusDot status="ok" label={t('xray.status.ok')} />
        <StatusDot status="busy" label={t('xray.status.busy')} />
        <StatusDot status="off" label={t('xray.status.off')} />
        <Spinner label={t('common.loading')} />
      </div>
    </Card>
  </section>

  <section>
    <h2 class="t-heading">{t('kit.copy')}</h2>
    <Card>
      <div class="grid">
        <CopyField label={t('kit.code_label')} value="Zk3iT0P0bRx6Qv2nC8yL1uWm4sJ7hAeD9fGqXo5tNcI" />
        <CopyField label="URL" value="https://203.0.113.7:24680/k3j9x2m4p8q1z7wv/" />
      </div>
    </Card>
  </section>

  <section>
    <h2 class="t-heading">{t('kit.loading_states')}</h2>
    <Card>
      <div class="stack">
        <Skeleton width="40%" height="20px" />
        <Skeleton height="14px" />
        <Skeleton width="70%" height="14px" />
      </div>
    </Card>
  </section>

  <section>
    <h2 class="t-heading">{t('kit.empty')}</h2>
    <Card>
      <EmptyState text={t('kit.empty_text')}>
        {#snippet action()}
          <Button variant="secondary">{t('kit.empty_action')}</Button>
        {/snippet}
      </EmptyState>
    </Card>
  </section>
</Page>

<Modal bind:open={modal} title={t('kit.modal_title')}>
  <p class="muted modal-text">{t('kit.modal_body')}</p>
  <Input label={t('kit.input_label')} placeholder={t('kit.input_placeholder')} />
  {#snippet footer()}
    <Button variant="quiet" onclick={() => (modal = false)}>{t('common.cancel')}</Button>
    <Button variant="primary" onclick={() => (modal = false)}>{t('common.create')}</Button>
  {/snippet}
</Modal>

<style>
  section { display: flex; flex-direction: column; gap: var(--s4); }
  .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: var(--s4); }
  .row { display: flex; flex-wrap: wrap; align-items: center; gap: var(--s3); }
  .row.tight { gap: var(--s2); margin-top: var(--s4); }
  .stack { display: flex; flex-direction: column; gap: var(--s3); }
  .card-head { display: flex; align-items: center; justify-content: space-between; gap: var(--s3); margin-bottom: var(--s2); }
  .modal-text { margin-bottom: var(--s4); }
  @media (max-width: 400px) {
    .grid { grid-template-columns: 1fr; }
  }
</style>
