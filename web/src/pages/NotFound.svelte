<script lang="ts">
  import { t } from '../lib/i18n/index.svelte';
  import { navigate } from '../lib/router.svelte';
  import Button from '../lib/ui/Button.svelte';
  import Card from '../lib/ui/Card.svelte';
  import EmptyState from '../lib/ui/EmptyState.svelte';
  import Page from '../shell/Page.svelte';
</script>

<Page title={t('notfound.title')}>
  <Card>
    <EmptyState text={t('notfound.title')}>
      {#snippet action()}
        <Button variant="secondary" onclick={() => navigate('/')}>{t('notfound.action')}</Button>
      {/snippet}
    </EmptyState>
  </Card>
</Page>
