<script lang="ts">
  import { t } from '../lib/i18n/index.svelte';
  import Page from '../shell/Page.svelte';
  import CertSection from './settings/CertSection.svelte';
</script>

<Page title={t('nav.settings')}>
  <div class="sections">
    <CertSection />
  </div>
</Page>

<style>
  .sections { display: flex; flex-direction: column; gap: var(--s4); max-width: 720px; }
</style>
