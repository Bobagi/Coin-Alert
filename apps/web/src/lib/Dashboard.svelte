<script lang="ts">
  import { onMount } from 'svelte'
  import { api, type TradingSettings, type CredentialStatus, type Operation } from './api'
  import { currentUser } from './stores'
  import { t } from './i18n'
  import LanguageSwitcher from './LanguageSwitcher.svelte'
  import AllocationChart from './AllocationChart.svelte'
  import PortfolioPanel from './PortfolioPanel.svelte'

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
      credMsg = $t('binance.validatedSaved')
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
      settingsMsg = $t('settings.saved')
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
      tradeMsg = $t('buy.bought', {
        qty: fmt(operation.quantity),
        symbol: operation.symbol,
        price: fmt(operation.purchase_price_per_unit)
      })
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
  <LanguageSwitcher />
  {#if credentials}
    <span class="pill">{$t('header.binance')} {credentials.has_active_credential ? credentials.active_environment : $t('header.notConnected')}</span>
  {/if}
  <span class="muted">{$currentUser?.email}</span>
  <button class="ghost" on:click={logout}>{$t('header.signOut')}</button>
</header>

<main class="container">
  {#if loadingError}<div class="card error">{loadingError}</div>{/if}

  <details class="card start" open>
    <summary><strong>{$t('start.title')}</strong></summary>
    <p class="muted" style="margin-top:10px;">{$t('start.intro')}</p>
    <ol>
      <li>{$t('start.s1')}</li>
      <li>{$t('start.s2')}</li>
      <li>{$t('start.s3')}</li>
      <li>{$t('start.s4')}</li>
    </ol>
  </details>

  <div class="grid">
    <section class="card">
      <h2>{$t('binance.title')}</h2>
      <p class="muted">{$t('binance.subtitle')}</p>
      <details class="help"><summary>{$t('help.summary')}</summary><p>{$t('binance.help')}</p></details>
      {#if credentials?.has_active_credential}
        <div class="pill" style="margin-top:12px;">{$t('binance.activePrefix')}: {credentials.active_environment} • {credentials.masked_api_key}</div>
      {:else}
        <p class="muted" style="margin-top:12px;">{$t('binance.connectHint')}</p>
      {/if}
      <label>{$t('binance.environment')}</label>
      <select bind:value={credEnv}>
        <option value="TESTNET">{$t('binance.testnet')}</option>
        <option value="PRODUCTION">{$t('binance.production')}</option>
      </select>
      <label>{$t('binance.apiKey')}</label>
      <input bind:value={credKey} placeholder={$t('binance.apiKey')} />
      <label>{$t('binance.apiSecret')}</label>
      <input type="password" bind:value={credSecret} placeholder={$t('binance.apiSecret')} />
      <button style="width:100%; margin-top:14px;" disabled={credBusy} on:click={saveCredentials}>
        {credBusy ? $t('binance.saving') : $t('binance.save')}
      </button>
      {#if credMsg}<p class="success">{credMsg}</p>{/if}
      {#if credErr}<p class="error">{credErr}</p>{/if}
    </section>

    <section class="card">
      <h2>{$t('buy.title')}</h2>
      <p class="muted">{$t('buy.subtitle')}</p>
      <details class="help"><summary>{$t('help.summary')}</summary><p>{$t('buy.help')}</p></details>
      <label>{$t('buy.pair')}</label>
      <input bind:value={tradeSymbol} on:blur={checkPrice} placeholder="BTCUSDT" />
      {#if tradePrice !== null}<div class="muted">{$t('buy.currentPrice', { price: fmt(tradePrice) })}</div>{/if}
      <label>{$t('buy.amount')}</label>
      <input type="number" bind:value={tradeAmount} min="0" step="0.01" />
      <label>{$t('buy.target')}</label>
      <input type="number" bind:value={tradeTarget} min="0" step="0.01" />
      <button style="width:100%; margin-top:14px;" disabled={tradeBusy} on:click={buy}>
        {tradeBusy ? $t('buy.placing') : $t('buy.button')}
      </button>
      {#if tradeMsg}<p class="success">{tradeMsg}</p>{/if}
      {#if tradeErr}<p class="error">{tradeErr}</p>{/if}
    </section>

    {#if settings}
      <section class="card">
        <h2>{$t('settings.title')}</h2>
        <p class="muted">{$t('settings.subtitle')}</p>
        <details class="help"><summary>{$t('help.summary')}</summary><p>{$t('settings.help')}</p></details>
        <label>{$t('settings.defaultPair')}</label>
        <input bind:value={settings.trading_pair_symbol} />
        <div class="two">
          <div>
            <label>{$t('settings.capital')}</label>
            <input type="number" bind:value={settings.capital_threshold} min="0" step="0.01" />
          </div>
          <div>
            <label>{$t('settings.target')}</label>
            <input type="number" bind:value={settings.target_profit_percent} min="0" step="0.01" />
          </div>
        </div>
        <div class="two">
          <div>
            <label>{$t('settings.stopLoss')}</label>
            <input type="number" bind:value={settings.stop_loss_percent} min="0" step="0.01" placeholder={$t('settings.stopLossNone')} />
          </div>
          <div>
            <label>{$t('settings.dailyHour')}</label>
            <input type="number" bind:value={settings.daily_purchase_hour_utc} min="0" max="23" />
          </div>
        </div>
        <label class="row">
          <input type="checkbox" bind:checked={settings.live_trading_enabled} style="width:auto" />
          {$t('settings.enableLive')}
        </label>
        <button style="width:100%; margin-top:14px;" disabled={settingsBusy} on:click={saveSettings}>
          {settingsBusy ? $t('settings.saving') : $t('settings.save')}
        </button>
        {#if settingsMsg}<p class="muted">{settingsMsg}</p>{/if}
      </section>
    {/if}

    <section class="card">
      <h2>{$t('alloc.title')}</h2>
      <details class="help"><summary>{$t('help.summary')}</summary><p>{$t('alloc.help')}</p></details>
      {#if operations.some((operation) => operation.status === 'OPEN')}
        <AllocationChart {operations} />
      {:else}
        <p class="muted" style="margin-top:12px;">{$t('alloc.none')}</p>
      {/if}
    </section>
  </div>

  <section class="card" style="margin-top:18px;">
    <h2>{$t('ops.title')}</h2>
    {#if operations.length === 0}
      <p class="muted">{$t('ops.none')}</p>
    {:else}
      <div class="table">
        <div class="trow thead">
          <div>{$t('ops.pair')}</div>
          <div>{$t('ops.status')}</div>
          <div>{$t('ops.qty')}</div>
          <div>{$t('ops.buyPrice')}</div>
          <div>{$t('ops.target')}</div>
          <div>{$t('ops.purchased')}</div>
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

  <PortfolioPanel />
</main>

<style>
  .topbar { display: flex; align-items: center; gap: 14px; padding: 16px 24px; border-bottom: 1px solid var(--border); flex-wrap: wrap; }
  .brand { font-weight: 800; font-size: 1.3em; }
  .brand span { color: var(--brand); }
  .spacer { flex: 1; }
  .container { max-width: 1200px; margin: 24px auto; padding: 0 18px; }
  .start { margin-bottom: 18px; }
  .start summary { cursor: pointer; font-size: 1.05em; }
  .start ol { margin: 8px 0 0; padding-left: 20px; line-height: 1.7; color: var(--text); }
  .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 16px; align-items: start; }
  .two { display: flex; gap: 12px; }
  .two > div { flex: 1; }
  .row { display: flex; align-items: center; gap: 8px; margin-top: 14px; font-weight: 600; }
  .table { display: flex; flex-direction: column; }
  .trow { display: grid; grid-template-columns: 1fr 1fr 1fr 1.2fr 1.2fr 1.6fr; gap: 8px; padding: 10px 6px; border-bottom: 1px solid var(--border); align-items: center; }
  .thead { color: var(--muted); font-weight: 700; font-size: 0.85em; }
</style>
