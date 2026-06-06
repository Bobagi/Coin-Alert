<script lang="ts">
  import { onMount } from 'svelte'
  import { api, type TradingSettings, type CredentialStatus, type Operation } from './api'
  import { currentUser } from './stores'
  import AllocationChart from './AllocationChart.svelte'

  let settings: TradingSettings | null = null
  let credentials: CredentialStatus | null = null
  let operations: Operation[] = []
  let loadingError = ''

  let credEnv = 'TESTNET'
  let credKey = ''
  let credSecret = ''
  let credMsg = ''
  let credErr = ''
  let credBusy = false

  let settingsMsg = ''
  let settingsBusy = false

  let tradeSymbol = 'BTCUSDT'
  let tradeAmount = 15
  let tradeTarget = 1.5
  let tradePrice: number | null = null
  let tradeMsg = ''
  let tradeErr = ''
  let tradeBusy = false

  const fmt = (value: number | null) =>
    value === null || value === undefined ? '—' : value.toLocaleString(undefined, { maximumFractionDigits: 8 })

  async function loadAll() {
    try {
      const [loadedSettings, loadedCredentials, loadedOperations] = await Promise.all([
        api.getSettings(),
        api.getCredentials(),
        api.getOperations()
      ])
      settings = loadedSettings
      credentials = loadedCredentials
      operations = loadedOperations
      tradeSymbol = loadedSettings.trading_pair_symbol || 'BTCUSDT'
      tradeTarget = loadedSettings.target_profit_percent || 1.5
      if (loadedSettings.capital_threshold > 0) tradeAmount = loadedSettings.capital_threshold
    } catch (e) {
      loadingError = (e as Error).message
    }
  }

  async function saveCredentials() {
    credBusy = true
    credMsg = ''
    credErr = ''
    try {
      await api.saveCredentials(credEnv, credKey, credSecret)
      credKey = ''
      credSecret = ''
      credMsg = 'Validated and saved.'
      credentials = await api.getCredentials()
    } catch (e) {
      credErr = (e as Error).message
    } finally {
      credBusy = false
    }
  }

  async function saveSettings() {
    if (!settings) return
    settingsBusy = true
    settingsMsg = ''
    try {
      settings = await api.saveSettings(settings)
      settingsMsg = 'Settings saved.'
    } catch (e) {
      settingsMsg = (e as Error).message
    } finally {
      settingsBusy = false
    }
  }

  async function checkPrice() {
    tradeErr = ''
    try {
      tradePrice = (await api.getPrice(tradeSymbol)).price
    } catch (e) {
      tradeErr = (e as Error).message
      tradePrice = null
    }
  }

  async function buy() {
    tradeBusy = true
    tradeMsg = ''
    tradeErr = ''
    try {
      const operation = await api.buy(tradeSymbol, tradeAmount, tradeTarget)
      tradeMsg = `Bought ${fmt(operation.quantity)} ${operation.symbol} @ ${fmt(operation.purchase_price_per_unit)}.`
      operations = await api.getOperations()
    } catch (e) {
      tradeErr = (e as Error).message
    } finally {
      tradeBusy = false
    }
  }

  async function logout() {
    try {
      await api.logout()
    } catch {
      /* ignore */
    }
    currentUser.set(null)
  }

  onMount(loadAll)
</script>

<header class="topbar">
  <div class="brand">Coin<span>Hub</span></div>
  <div class="spacer"></div>
  {#if credentials}
    <span class="pill">Binance: {credentials.has_active_credential ? credentials.active_environment : 'not connected'}</span>
  {/if}
  <span class="muted">{$currentUser?.email}</span>
  <button class="ghost" on:click={logout}>Sign out</button>
</header>

<main class="container">
  {#if loadingError}<div class="card error">{loadingError}</div>{/if}

  <div class="grid">
    <section class="card">
      <h2>Binance connection</h2>
      {#if credentials?.has_active_credential}
        <div class="pill">Active: {credentials.active_environment} • key {credentials.masked_api_key}</div>
      {:else}
        <p class="muted">Connect with trade-only keys (withdrawals disabled). New accounts start on Testnet.</p>
      {/if}
      <label>Environment</label>
      <select bind:value={credEnv}>
        <option value="TESTNET">Testnet</option>
        <option value="PRODUCTION">Production (real money)</option>
      </select>
      <label>API key</label>
      <input bind:value={credKey} placeholder="API key" />
      <label>API secret</label>
      <input type="password" bind:value={credSecret} placeholder="API secret" />
      <button style="width:100%; margin-top:14px;" disabled={credBusy} on:click={saveCredentials}>
        {credBusy ? 'Validating…' : 'Validate & save'}
      </button>
      {#if credMsg}<p class="success">{credMsg}</p>{/if}
      {#if credErr}<p class="error">{credErr}</p>{/if}
    </section>

    <section class="card">
      <h2>Buy</h2>
      <p class="muted">Market buy plus a take-profit limit sell at your target.</p>
      <label>Pair</label>
      <input bind:value={tradeSymbol} on:blur={checkPrice} placeholder="BTCUSDT" />
      {#if tradePrice !== null}<div class="muted">Current price: {fmt(tradePrice)}</div>{/if}
      <label>Amount (quote currency)</label>
      <input type="number" bind:value={tradeAmount} min="0" step="0.01" />
      <label>Target profit %</label>
      <input type="number" bind:value={tradeTarget} min="0" step="0.01" />
      <button style="width:100%; margin-top:14px;" disabled={tradeBusy} on:click={buy}>
        {tradeBusy ? 'Placing…' : 'Buy + set take-profit'}
      </button>
      {#if tradeMsg}<p class="success">{tradeMsg}</p>{/if}
      {#if tradeErr}<p class="error">{tradeErr}</p>{/if}
    </section>

    {#if settings}
      <section class="card">
        <h2>Bot settings</h2>
        <label>Default pair</label>
        <input bind:value={settings.trading_pair_symbol} />
        <div class="two">
          <div>
            <label>Capital per buy</label>
            <input type="number" bind:value={settings.capital_threshold} min="0" step="0.01" />
          </div>
          <div>
            <label>Target profit %</label>
            <input type="number" bind:value={settings.target_profit_percent} min="0" step="0.01" />
          </div>
        </div>
        <div class="two">
          <div>
            <label>Stop-loss %</label>
            <input type="number" bind:value={settings.stop_loss_percent} min="0" step="0.01" placeholder="none" />
          </div>
          <div>
            <label>Daily buy hour (UTC)</label>
            <input type="number" bind:value={settings.daily_purchase_hour_utc} min="0" max="23" />
          </div>
        </div>
        <label class="row">
          <input type="checkbox" bind:checked={settings.live_trading_enabled} style="width:auto" />
          Enable live (real-money) trading
        </label>
        <button style="width:100%; margin-top:14px;" disabled={settingsBusy} on:click={saveSettings}>
          {settingsBusy ? 'Saving…' : 'Save settings'}
        </button>
        {#if settingsMsg}<p class="muted">{settingsMsg}</p>{/if}
      </section>
    {/if}

    <section class="card">
      <h2>Open allocation</h2>
      {#if operations.some((operation) => operation.status === 'OPEN')}
        <AllocationChart {operations} />
      {:else}
        <p class="muted">No open positions yet.</p>
      {/if}
    </section>
  </div>

  <section class="card" style="margin-top:18px;">
    <h2>Operations</h2>
    {#if operations.length === 0}
      <p class="muted">No operations yet. Connect Binance and place your first buy.</p>
    {:else}
      <div class="table">
        <div class="trow thead">
          <div>Pair</div>
          <div>Status</div>
          <div>Qty</div>
          <div>Buy price</div>
          <div>Target</div>
          <div>Purchased</div>
        </div>
        {#each operations as operation (operation.id)}
          <div class="trow">
            <div>{operation.symbol}</div>
            <div><span class="badge {operation.status === 'SOLD' ? 'green' : 'amber'}">{operation.status}</span></div>
            <div>{fmt(operation.quantity)}</div>
            <div>{fmt(operation.purchase_price_per_unit)}</div>
            <div>{fmt(operation.sell_target_price_per_unit)}</div>
            <div class="muted">{new Date(operation.purchased_at).toLocaleString()}</div>
          </div>
        {/each}
      </div>
    {/if}
  </section>
</main>

<style>
  .topbar { display: flex; align-items: center; gap: 14px; padding: 16px 24px; border-bottom: 1px solid var(--border); }
  .brand { font-weight: 800; font-size: 1.3em; }
  .brand span { color: var(--brand); }
  .spacer { flex: 1; }
  .container { max-width: 1200px; margin: 24px auto; padding: 0 18px; }
  .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 16px; align-items: start; }
  .two { display: flex; gap: 12px; }
  .two > div { flex: 1; }
  .row { display: flex; align-items: center; gap: 8px; margin-top: 14px; font-weight: 600; }
  .table { display: flex; flex-direction: column; }
  .trow { display: grid; grid-template-columns: 1fr 1fr 1fr 1.2fr 1.2fr 1.6fr; gap: 8px; padding: 10px 6px; border-bottom: 1px solid var(--border); align-items: center; }
  .thead { color: var(--muted); font-weight: 700; font-size: 0.85em; }
</style>
