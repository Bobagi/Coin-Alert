<script lang="ts">
  import { onMount } from 'svelte'
  import { api } from './api'
  import { t } from './i18n'

  type AssetTable = { table_name: string; header: string[]; rows: string[][]; error?: string }

  let walletUrl = ''
  let savedUrl = ''
  let saving = false
  let sourceMessage = ''
  let assets: AssetTable[] = []
  let dividends: { asset: string; date_com: string }[] = []
  let busyAssets = false
  let busyDividends = false
  let assetsError = ''
  let dividendsError = ''

  onMount(async () => {
    try {
      savedUrl = (await api.getPortfolioSource()).wallet_url
      walletUrl = savedUrl
    } catch {
      /* not set yet */
    }
  })

  async function saveSource() {
    saving = true
    sourceMessage = ''
    try {
      await api.savePortfolioSource(walletUrl)
      savedUrl = walletUrl.trim()
      sourceMessage = $t('portfolio.saved')
    } catch (e) {
      sourceMessage = (e as Error).message
    } finally {
      saving = false
    }
  }

  async function loadAssets() {
    busyAssets = true
    assetsError = ''
    try {
      assets = (await api.getPortfolioAssets()).tables || []
    } catch (e) {
      assetsError = (e as Error).message
    } finally {
      busyAssets = false
    }
  }

  async function loadDividends() {
    busyDividends = true
    dividendsError = ''
    try {
      dividends = (await api.getPortfolioDividends()).results || []
    } catch (e) {
      dividendsError = (e as Error).message
    } finally {
      busyDividends = false
    }
  }
</script>

<section class="card" style="margin-top:18px;">
  <h2>{$t('portfolio.title')}</h2>
  <p class="muted">{$t('portfolio.subtitle')}</p>
  <details class="help"><summary>{$t('help.summary')}</summary><p>{$t('portfolio.help')}</p></details>

  <input bind:value={walletUrl} placeholder={$t('portfolio.placeholder')} style="margin-top:12px;" />
  <div class="actions">
    <button on:click={saveSource} disabled={saving}>{saving ? $t('common.saving') : $t('portfolio.saveUrl')}</button>
    <button class="ghost" on:click={loadAssets} disabled={busyAssets || !savedUrl}>{busyAssets ? $t('portfolio.loading') : $t('portfolio.loadAssets')}</button>
    <button class="ghost" on:click={loadDividends} disabled={busyDividends || !savedUrl}>{busyDividends ? $t('portfolio.loading') : $t('portfolio.dividends')}</button>
  </div>
  {#if sourceMessage}<p class="muted">{sourceMessage}</p>{/if}
  {#if assetsError}<p class="error">{assetsError}</p>{/if}
  {#if dividendsError}<p class="error">{dividendsError}</p>{/if}

  {#each assets as table}
    <h3>{table.table_name}</h3>
    {#if table.error}
      <p class="error">{table.error}</p>
    {:else}
      <div class="ptable">
        {#if table.header && table.header.length}
          <div class="prow phead">{#each table.header as cell}<div>{cell}</div>{/each}</div>
        {/if}
        {#each table.rows as row}
          <div class="prow">{#each row as cell}<div>{cell}</div>{/each}</div>
        {/each}
      </div>
    {/if}
  {/each}

  {#if dividends.length}
    <h3>{$t('portfolio.upcoming')}</h3>
    <div class="ptable">
      <div class="prow phead"><div>{$t('portfolio.asset')}</div><div>{$t('portfolio.date')}</div></div>
      {#each dividends as dividend}
        <div class="prow"><div>{dividend.asset}</div><div>{dividend.date_com}</div></div>
      {/each}
    </div>
  {/if}
</section>

<style>
  .actions { display: flex; gap: 8px; margin-top: 10px; flex-wrap: wrap; }
  h3 { margin: 16px 0 6px; }
  .ptable { display: flex; flex-direction: column; overflow-x: auto; }
  .prow { display: flex; gap: 10px; padding: 6px 4px; border-bottom: 1px solid var(--border); }
  .prow > div { flex: 1; min-width: 90px; font-size: 0.85em; white-space: nowrap; }
  .phead { color: var(--muted); font-weight: 700; }
</style>
