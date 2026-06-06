<script lang="ts">
  import { onMount } from 'svelte'
  import { api } from './api'

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
      sourceMessage = 'Saved.'
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

<section class="card">
  <h2>B3 portfolio (Investidor10)</h2>
  <p class="muted">Paste your public Investidor10 wallet URL. Scraping uses a headless browser and can take up to a minute.</p>
  <input bind:value={walletUrl} placeholder="https://investidor10.com.br/carteiras/..." />
  <div class="actions">
    <button on:click={saveSource} disabled={saving}>{saving ? 'Saving…' : 'Save URL'}</button>
    <button class="ghost" on:click={loadAssets} disabled={busyAssets || !savedUrl}>{busyAssets ? 'Loading…' : 'Load assets'}</button>
    <button class="ghost" on:click={loadDividends} disabled={busyDividends || !savedUrl}>{busyDividends ? 'Loading…' : 'Dividend dates'}</button>
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
    <h3>Upcoming ex-dividend dates</h3>
    <div class="ptable">
      <div class="prow phead"><div>Asset</div><div>Date</div></div>
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
