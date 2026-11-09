<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '../lib/api';
  import { errorText } from '../lib/errors';
  import { t } from '../lib/i18n/index.svelte';
  import { fadeShift, stagger } from '../lib/motion';
  import Card from '../lib/ui/Card.svelte';
  import { toast } from '../lib/ui/toasts.svelte';
  import LangSwitch from '../shell/LangSwitch.svelte';
  import Page from '../shell/Page.svelte';
  import CertSection from './settings/CertSection.svelte';
  import PanelSection from './settings/PanelSection.svelte';
  import PasswordSection from './settings/PasswordSection.svelte';
  import ServerSection from './settings/ServerSection.svelte';
  import SwiftSection from './settings/SwiftSection.svelte';
  import XraySection from './settings/XraySection.svelte';
  import type { PanelInfo } from './settings/panel';

  let panel = $state<PanelInfo | null>(null);

  onMount(() => {
    api<PanelInfo>('settings/panel').then((p) => (panel = p)).catch((e) => toast(errorText(e)));
  });
</script>

<Page title={t('nav.settings')}>
  <div class="sections">
    <div in:fadeShift={{ delay: stagger(0) }}><ServerSection bind:info={panel} /></div>
    <div in:fadeShift={{ delay: stagger(1) }}><CertSection /></div>
    <div in:fadeShift={{ delay: stagger(2) }}><PanelSection bind:info={panel} /></div>
    <div in:fadeShift={{ delay: stagger(3) }}>
      <Card>
        <div class="row">
          <h2 class="t-heading">{t('settings.lang')}</h2>
          <LangSwitch />
        </div>
      </Card>
    </div>
    <div in:fadeShift={{ delay: stagger(4) }}><PasswordSection /></div>
    <div id="swift" in:fadeShift={{ delay: stagger(5) }}><SwiftSection /></div>
    <div id="xray" in:fadeShift={{ delay: stagger(6) }}><XraySection /></div>
  </div>
</Page>

<style>
  .sections { display: flex; flex-direction: column; gap: var(--s4); max-width: 720px; }
  .row { display: flex; align-items: center; justify-content: space-between; gap: var(--s4); }
</style>
