<script lang="ts">
  import { t } from '../lib/i18n/index.svelte';
  import Card from '../lib/ui/Card.svelte';
  import EmptyState from '../lib/ui/EmptyState.svelte';
  import Page from '../shell/Page.svelte';

  let { title }: { title: string } = $props();
</script>

<Page {title}>
  <Card><EmptyState text={t('page.placeholder')} /></Card>
</Page>
