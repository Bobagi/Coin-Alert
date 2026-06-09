<script lang="ts">
  import { onMount } from 'svelte'
  import { api, type TradingSettings, type CredentialStatus, type Operation, type Execution } from './api'
  import { binanceStatus } from './stores'
  import { t, locale } from './i18n'
  import AllocationPanel from './AllocationPanel.svelte'
  import PortfolioPanel from './PortfolioPanel.svelte'
  import LegalFooter from './LegalFooter.svelte'
  import SymbolAutocomplete from './SymbolAutocomplete.svelte'

  let activeTab: 'connection' | 'trade' | 'b3' = 'connection'
  let opsView: 'positions' | 'history' = 'positions'
  const environments = ['TESTNET', 'PRODUCTION']

  let settings: TradingSettings | null = null
  let credentials: CredentialStatus | null = null
  let operations: Operation[] = []
  let executions: Execution[] = []
  let symbols: string[] = []
  let loadingError = ''

  // credEnv is the environment currently selected in the connection tab (which the key form targets).
  let credEnv = 'TESTNET'
  let credKey = ''
  let credSecret = ''
  let credMsg = ''
  let credErr = ''
  let credBusy = false

  let envBusy = ''
  let envMsg = ''
  let envErr = ''

  let settingsMsg = ''
  let settingsErr = ''
  let settingsBusy = false
  let botToggleBusy = false

  let dailyHourLocal = 4
  const hours = Array.from({ length: 24 }, (_, index) => index)
  const localTimeZone = typeof Intl !== 'undefined' ? Intl.DateTimeFormat().resolvedOptions().timeZone : 'UTC'
  const tzOffset = timezoneOffsetLabel()

  let tradeSymbol = 'BTCUSDT'
  let tradeAmount = 15
  let tradeTarget = 1.5
  let tradePrice: number | null = null
  let tradeFilters: { min_notional: number; tick_size: number; step_size: number } | null = null
  let tradeMsg = ''
  let tradeErr = ''
  let tradeBusy = false

  let sellBusyId: number | null = null
  let placeSellBusyId: number | null = null
  let opsMsg = ''
  let opsErr = ''

  const fmt = (value: number | null) =>
    value === null || value === undefined ? '—' : value.toLocaleString(undefined, { maximumFractionDigits: 8 })

  function formatHour(hour: number) {
    return String(hour).padStart(2, '0') + ':00'
  }
  function utcHourToLocal(utcHour: number) {
    const date = new Date()
    date.setUTCHours(utcHour, 0, 0, 0)
    return date.getHours()
  }
  function localHourToUtc(localHour: number) {
    const date = new Date()
    date.setHours(localHour, 0, 0, 0)
    return date.getUTCHours()
  }
  function timezoneOffsetLabel() {
    const totalMinutes = -new Date().getTimezoneOffset()
    const sign = totalMinutes >= 0 ? '+' : '-'
    const absMinutes = Math.abs(totalMinutes)
    const wholeHours = Math.floor(absMinutes / 60)
    const minutes = absMinutes % 60
    return `GMT${sign}${wholeHours}${minutes ? ':' + String(minutes).padStart(2, '0') : ''}`
  }
  function nextDailyRun(utcHour: number) {
    const now = new Date()
    const next = new Date()
    next.setUTCHours(utcHour, 0, 0, 0)
    if (next <= now) next.setUTCDate(next.getUTCDate() + 1)
    return next
  }

  $: botEnabled = !!settings && settings.daily_purchase_enabled
  $: botActive = botEnabled && !!settings && settings.capital_threshold > 0
  $: connected = !!credentials?.has_active_credential
  $: needsLiveWarning = botActive && credentials?.active_environment === 'PRODUCTION' && !!settings && !settings.live_trading_enabled
  $: nextRun = nextDailyRun(localHourToUtc(dailyHourLocal))
  $: nextRunLabel = nextRun.toLocaleString($locale, { weekday: 'short', hour: '2-digit', minute: '2-digit' })
  $: hoursUntilNext = Math.max(1, Math.round((nextRun.getTime() - Date.now()) / 3600000))

  const isConfigured = (environment: string) => !!credentials?.configured_environments?.includes(environment)
  const isActive = (environment: string) => !!credentials?.has_active_credential && credentials?.active_environment === environment

  function publishBinanceStatus(status: CredentialStatus) {
    binanceStatus.set({ has_active_credential: status.has_active_credential, active_environment: status.active_environment })
  }

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
      publishBinanceStatus(loadedCredentials)
      if (loadedCredentials.has_active_credential) credEnv = loadedCredentials.active_environment
      dailyHourLocal = utcHourToLocal(loadedSettings.daily_purchase_hour_utc)
      tradeSymbol = loadedSettings.trading_pair_symbol || 'BTCUSDT'
      tradeTarget = loadedSettings.target_profit_percent || 1.5
      if (loadedSettings.capital_threshold > 0) tradeAmount = loadedSettings.capital_threshold
    } catch (e) {
      loadingError = (e as Error).message
    }
  }

  async function loadExecutions() {
    try {
      executions = await api.getExecutions()
    } catch {
      executions = []
    }
  }

  async function loadSymbols() {
    try {
      symbols = (await api.getSymbols()).symbols || []
    } catch {
      symbols = []
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
      // Saving keys activates that environment, so reload everything for the now-active environment.
      await loadAll()
      await loadExecutions()
      loadSymbols()
    } catch (e) {
      credErr = (e as Error).message
    } finally {
      credBusy = false
    }
  }

  // Selecting an environment targets it for the key form and, if it already has keys, activates it.
  async function selectEnvironment(environment: string) {
    credEnv = environment
    envMsg = ''
    envErr = ''
    if (!isConfigured(environment) || isActive(environment)) return
    envBusy = environment
    try {
      await api.activateEnvironment(environment)
      await loadAll()
      await loadExecutions()
      loadSymbols()
      envMsg = $t('binance.activated')
    } catch (e) {
      envErr = (e as Error).message
    } finally {
      envBusy = ''
    }
  }

  async function saveSettings() {
    if (!settings) return
    settingsBusy = true
    settingsMsg = ''
    settingsErr = ''
    try {
      settings.daily_purchase_hour_utc = localHourToUtc(dailyHourLocal)
      settings = await api.saveSettings(settings)
      settingsMsg = $t('settings.saved')
    } catch (e) {
      settingsErr = (e as Error).message || $t('settings.savedError')
    } finally {
      settingsBusy = false
    }
  }

  async function toggleBot() {
    if (!settings) return
    botToggleBusy = true
    settingsErr = ''
    settingsMsg = ''
    const previousValue = settings.daily_purchase_enabled
    try {
      settings.daily_purchase_enabled = !previousValue
      settings.daily_purchase_hour_utc = localHourToUtc(dailyHourLocal)
      settings = await api.saveSettings(settings)
    } catch (e) {
      if (settings) settings.daily_purchase_enabled = previousValue
      settingsErr = (e as Error).message
    } finally {
      botToggleBusy = false
    }
  }

  async function checkPrice() {
    if (!tradeSymbol) return
    tradeErr = ''
    try {
      tradePrice = (await api.getPrice(tradeSymbol)).price
    } catch (e) {
      tradeErr = (e as Error).message
      tradePrice = null
    }
    try {
      tradeFilters = await api.getSymbolFilters(tradeSymbol)
    } catch {
      tradeFilters = null
    }
  }

  $: belowMinimum = !!tradeFilters && tradeFilters.min_notional > 0 && tradeAmount < tradeFilters.min_notional

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
      loadExecutions()
    } catch (e) {
      tradeErr = (e as Error).message
    } finally {
      tradeBusy = false
    }
  }

  async function sellNow(operationId: number) {
    if (!confirm($t('ops.sellConfirm'))) return
    sellBusyId = operationId
    opsMsg = ''
    opsErr = ''
    try {
      await api.sellOperation(operationId)
      opsMsg = $t('ops.sold')
      operations = await api.getOperations()
      loadExecutions()
    } catch (e) {
      opsErr = (e as Error).message
    } finally {
      sellBusyId = null
    }
  }

  // Retry placing the take-profit sell order for a position whose original sell failed.
  async function placeSell(operationId: number) {
    placeSellBusyId = operationId
    opsMsg = ''
    opsErr = ''
    try {
      await api.placeSellOrder(operationId)
      opsMsg = $t('ops.sellPlaced')
      operations = await api.getOperations()
      loadExecutions()
    } catch (e) {
      opsErr = (e as Error).message
    } finally {
      placeSellBusyId = null
    }
  }

  onMount(async () => {
    await loadAll()
    checkPrice()
    loadSymbols()
    loadExecutions()
  })
</script>

<main class="page stack-lg">
  {#if loadingError}<div class="card error">{loadingError}</div>{/if}

  <details class="card start" open>
    <summary>
      <span class="start-caret">▸</span>
      <span class="start-title">{$t('start.title')}</span>
    </summary>
    <p class="muted mt-3">{$t('start.intro')}</p>
    <ol>
      <li>{$t('start.s1')}</li>
      <li>{$t('start.s2')}</li>
      <li>{$t('start.s3')}</li>
      <li>{$t('start.s4')}</li>
    </ol>
  </details>

  <div class="tabs" role="tablist">
    <button class="tab" role="tab" aria-selected={activeTab === 'connection'} class:active={activeTab === 'connection'} on:click={() => (activeTab = 'connection')}>{$t('tab.connection')}</button>
    <button class="tab" role="tab" aria-selected={activeTab === 'trade'} class:active={activeTab === 'trade'} on:click={() => (activeTab = 'trade')}>{$t('tab.trade')}</button>
    <button class="tab" role="tab" aria-selected={activeTab === 'b3'} class:active={activeTab === 'b3'} on:click={() => (activeTab = 'b3')}>{$t('tab.b3')}</button>
  </div>

  {#if activeTab === 'connection'}
    <section class="card conn">
      <div class="card-header">
        <span class="card-title">{$t('binance.title')}</span>
        <span class="card-subtitle">{$t('binance.subtitle')}</span>
      </div>
      <details class="help"><summary>{$t('help.summary')}</summary><p>{$t('binance.help')}</p></details>

      <div class="field">
        <span class="field-label">{$t('binance.activeEnv')}</span>
        <div class="env-switch">
          {#each environments as environment}
            <button
              type="button"
              class="env-btn"
              class:active={credEnv === environment}
              disabled={envBusy === environment}
              on:click={() => selectEnvironment(environment)}
            >
              <span>{environment === 'TESTNET' ? $t('binance.testnet') : $t('binance.production')}</span>
              {#if isActive(environment)}
                <span class="tag on">✓ {$t('binance.active')}</span>
              {:else if !isConfigured(environment)}
                <span class="tag">· {$t('binance.notConfigured')}</span>
              {/if}
            </button>
          {/each}
        </div>
        <span class="muted mt-2">{$t('binance.envHint')}</span>
      </div>
      {#if envMsg}<p class="success mt-2">{envMsg}</p>{/if}
      {#if envErr}<p class="error mt-2">{envErr}</p>{/if}

      {#if isActive(credEnv) && credentials}
        <div class="pill mt-4">{$t('binance.activePrefix')}: {credEnv} • {credentials.masked_api_key}</div>
      {:else}
        <p class="muted mt-4">{$t('binance.connectHint')}</p>
      {/if}

      <div class="field">
        <label for="cred-key">{$t('binance.apiKey')} — {credEnv === 'TESTNET' ? $t('binance.testnet') : $t('binance.production')}</label>
        <input id="cred-key" bind:value={credKey} placeholder={$t('binance.apiKey')} />
      </div>
      <div class="field">
        <label for="cred-secret">{$t('binance.apiSecret')}</label>
        <input id="cred-secret" type="password" bind:value={credSecret} placeholder={$t('binance.apiSecret')} />
      </div>
      <button class="btn-block mt-5" disabled={credBusy} on:click={saveCredentials}>
        {credBusy ? $t('binance.saving') : $t('binance.save')}
      </button>
      {#if credMsg}<p class="success mt-3">{credMsg}</p>{/if}
      {#if credErr}<p class="error mt-3">{credErr}</p>{/if}
    </section>
  {:else if activeTab === 'trade'}
    <div class="grid">
      <section class="card">
        <div class="card-header">
          <span class="card-title">{$t('buy.title')}</span>
          <span class="card-subtitle">{$t('buy.subtitle')}</span>
        </div>
        <details class="help"><summary>{$t('help.summary')}</summary><p>{$t('buy.help')}</p></details>
        <div class="field">
          <label for="trade-symbol">{$t('buy.pair')}</label>
          <SymbolAutocomplete id="trade-symbol" bind:value={tradeSymbol} options={symbols} placeholder="BTCUSDT" on:select={checkPrice} on:commit={checkPrice} />
        </div>
        {#if tradePrice !== null}<div class="muted mt-2">{$t('buy.currentPrice', { price: fmt(tradePrice) })}</div>{/if}
        <div class="field">
          <label for="trade-amount">{$t('buy.amount')}</label>
          <input id="trade-amount" type="number" bind:value={tradeAmount} min="0" step="0.01" />
          {#if tradeFilters && tradeFilters.min_notional > 0}
            <span class="muted">{$t('buy.minOrder', { min: fmt(tradeFilters.min_notional) })}</span>
          {/if}
          {#if belowMinimum && tradeFilters}<span class="error">{$t('buy.belowMin', { min: fmt(tradeFilters.min_notional) })}</span>{/if}
        </div>
        <div class="field">
          <label for="trade-target">{$t('buy.target')}</label>
          <input id="trade-target" type="number" bind:value={tradeTarget} min="0" step="0.01" />
        </div>
        <button class="btn-block mt-5" disabled={tradeBusy || belowMinimum} on:click={buy}>
          {tradeBusy ? $t('buy.placing') : $t('buy.button')}
        </button>
        {#if tradeMsg}<p class="success mt-3">{tradeMsg}</p>{/if}
        {#if tradeErr}<p class="error mt-3">{tradeErr}</p>{/if}
      </section>

      {#if settings}
        <section class="card">
          <div class="card-header">
            <span class="card-title">{$t('settings.title')}</span>
            <span class="card-subtitle">{$t('settings.subtitle')}</span>
          </div>

          <div class="bot-status" class:on={botActive}>
            <div class="bot-head">
              <span class="badge {botActive ? 'green' : 'amber'}">{botActive ? $t('bot.active') : $t('bot.inactive')}</span>
              <strong>{$t('bot.title')}</strong>
              <span class="spacer"></span>
              <button type="button" class="btn-sm bot-toggle {botEnabled ? 'ghost' : ''}" disabled={botToggleBusy} on:click={toggleBot}>
                {botToggleBusy ? $t('common.saving') : botEnabled ? $t('bot.pause') : $t('bot.resume')}
              </button>
            </div>
            {#if !botEnabled}
              <p class="muted">{$t('bot.paused')}</p>
            {:else if !botActive}
              <p class="muted">{$t('bot.off')}</p>
            {:else}
              <p class="muted">{$t('bot.summary', { time: formatHour(dailyHourLocal), capital: fmt(settings.capital_threshold), symbol: settings.trading_pair_symbol, target: settings.target_profit_percent })}</p>
              <p class="next">{$t('bot.next', { when: nextRunLabel, hours: hoursUntilNext })}</p>
              {#if !connected}<p class="warn">{$t('bot.needsConnection')}</p>{/if}
              {#if needsLiveWarning}<p class="warn">{$t('bot.needsLive')}</p>{/if}
            {/if}
          </div>

          <details class="help"><summary>{$t('help.summary')}</summary><p>{$t('settings.help')}</p></details>
          <div class="field">
            <label for="default-pair">{$t('settings.defaultPair')}</label>
            <SymbolAutocomplete id="default-pair" bind:value={settings.trading_pair_symbol} options={symbols} placeholder="BTCUSDT" />
          </div>
          <div class="grid-2 mt-4">
            <div class="field" style="margin-top:0">
              <label for="capital">{$t('settings.capital')}</label>
              <input id="capital" type="number" bind:value={settings.capital_threshold} min="0" step="0.01" />
            </div>
            <div class="field" style="margin-top:0">
              <label for="target">{$t('settings.target')}</label>
              <input id="target" type="number" bind:value={settings.target_profit_percent} min="0" step="0.01" />
            </div>
          </div>
          <div class="grid-2 mt-4">
            <div class="field" style="margin-top:0">
              <label for="stop-loss">{$t('settings.stopLoss')}</label>
              <input id="stop-loss" type="number" bind:value={settings.stop_loss_percent} min="0" step="0.01" placeholder={$t('settings.stopLossNone')} />
            </div>
            <div class="field" style="margin-top:0">
              <label for="daily-time">{$t('settings.dailyTime')}</label>
              <select id="daily-time" bind:value={dailyHourLocal}>
                {#each hours as hour}<option value={hour}>{formatHour(hour)}</option>{/each}
              </select>
            </div>
          </div>
          <p class="muted tz-note">{$t('settings.timezoneNote', { tz: localTimeZone, offset: tzOffset })}</p>
          <label class="checkbox-row">
            <input type="checkbox" bind:checked={settings.live_trading_enabled} />
            {$t('settings.enableLive')}
          </label>
          <button class="btn-block mt-5" disabled={settingsBusy} on:click={saveSettings}>
            {settingsBusy ? $t('settings.saving') : $t('settings.save')}
          </button>
          {#if settingsMsg}<p class="success mt-3">✓ {settingsMsg}</p>{/if}
          {#if settingsErr}<p class="error mt-3">{settingsErr}</p>{/if}
        </section>
      {/if}
    </div>

    <section class="card">
      <div class="card-header">
        <span class="card-title">{$t('alloc.title')}</span>
      </div>
      <details class="help"><summary>{$t('help.summary')}</summary><p>{$t('alloc.help')}</p></details>
      <AllocationPanel {operations} />
    </section>

    <section class="card">
      <div class="card-header ops-header">
        <span class="card-title">{$t('ops.title')}</span>
        <div class="subtabs">
          <button class="subtab" class:active={opsView === 'positions'} on:click={() => (opsView = 'positions')}>{$t('ops.tabPositions')}</button>
          <button class="subtab" class:active={opsView === 'history'} on:click={() => (opsView = 'history')}>{$t('ops.tabHistory')}</button>
        </div>
      </div>

      {#if opsView === 'positions'}
        <details class="help">
          <summary>{$t('ops.statusHelp')}</summary>
          <p>{$t('ops.openMeaning')}</p>
          <p>{$t('ops.soldMeaning')}</p>
          <p>{$t('ops.sellOrderMeaning')}</p>
        </details>
        {#if opsMsg}<p class="success mt-3">{opsMsg}</p>{/if}
        {#if opsErr}<p class="error mt-3">{opsErr}</p>{/if}
        {#if operations.length === 0}
          <p class="muted mt-3">{$t('ops.none')}</p>
        {:else}
          <div class="table mt-3">
            <div class="trow thead">
              <div>{$t('ops.pair')}</div>
              <div>{$t('ops.status')}</div>
              <div>{$t('ops.qty')}</div>
              <div>{$t('ops.buyPrice')}</div>
              <div>{$t('ops.target')}</div>
              <div>{$t('ops.sellOrder')}</div>
              <div>{$t('ops.purchased')}</div>
              <div class="col-actions">{$t('ops.actions')}</div>
            </div>
            {#each operations as operation (operation.id)}
              <div class="trow">
                <div>{operation.symbol}</div>
                <div><span class="badge {operation.status === 'SOLD' ? 'green' : 'amber'}">{operation.status}</span></div>
                <div>{fmt(operation.quantity)}</div>
                <div>{fmt(operation.purchase_price_per_unit)}</div>
                <div>{fmt(operation.sell_target_price_per_unit)}</div>
                <div class="sell-cell">
                  {#if operation.sell_order_id}
                    <span class="badge green" title={$t('ops.gtcHelp')}>✓</span>
                    <span class="muted gtc" title={$t('ops.gtcHelp')}>{$t('ops.gtc')}</span>
                  {:else if operation.status === 'OPEN'}
                    <span class="badge red" title={$t('ops.noSellOrder')}>⚠</span>
                  {:else}
                    <span class="muted">—</span>
                  {/if}
                </div>
                <div class="muted">{new Date(operation.purchased_at).toLocaleString()}</div>
                <div class="col-actions ops-actions">
                  {#if operation.status === 'OPEN'}
                    {#if !operation.sell_order_id}
                      <button class="btn-sm" disabled={placeSellBusyId === operation.id} on:click={() => placeSell(operation.id)}>
                        {placeSellBusyId === operation.id ? $t('ops.retrying') : $t('ops.retrySell')}
                      </button>
                    {/if}
                    <button class="danger btn-sm" disabled={sellBusyId === operation.id} on:click={() => sellNow(operation.id)}>
                      {sellBusyId === operation.id ? $t('ops.selling') : $t('ops.sellNow')}
                    </button>
                  {:else}
                    <span class="muted">—</span>
                  {/if}
                </div>
              </div>
            {/each}
          </div>
        {/if}
      {:else}
        {#if executions.length === 0}
          <p class="muted mt-3">{$t('hist.none')}</p>
        {:else}
          <div class="table htable mt-3">
            <div class="hrow thead">
              <div>{$t('hist.when')}</div>
              <div>{$t('hist.action')}</div>
              <div>{$t('hist.by')}</div>
              <div>{$t('ops.pair')}</div>
              <div>{$t('hist.price')}</div>
              <div>{$t('hist.qty')}</div>
              <div>{$t('hist.total')}</div>
              <div class="col-actions">{$t('hist.result')}</div>
            </div>
            {#each executions as execution (execution.id)}
              <div class="hrow">
                <div class="muted">{new Date(execution.executed_at).toLocaleString()}</div>
                <div><span class="badge {execution.operation_type === 'SELL' ? 'green' : 'amber'}">{execution.operation_type}</span></div>
                <div><span class="by-badge {execution.initiated_by === 'BOT' ? 'bot' : 'user'}">{execution.initiated_by === 'BOT' ? $t('hist.bot') : $t('hist.you')}</span></div>
                <div>{execution.symbol}</div>
                <div>{fmt(execution.unit_price)}</div>
                <div>{fmt(execution.quantity)}</div>
                <div>{fmt(execution.total_value)}</div>
                <div class="col-actions">
                  {#if execution.success}
                    <span class="badge green">✓</span>
                  {:else}
                    <span class="badge red" title={execution.error_message || ''}>✗</span>
                  {/if}
                </div>
              </div>
            {/each}
          </div>
        {/if}
      {/if}
    </section>
  {:else}
    <PortfolioPanel />
  {/if}

  <LegalFooter />
</main>

<style>
  .start summary { cursor: pointer; list-style: none; display: flex; align-items: center; gap: var(--space-2); }
  .start summary::-webkit-details-marker { display: none; }
  .start-caret { color: var(--brand); display: inline-block; transition: transform 0.15s ease; }
  .start[open] .start-caret { transform: rotate(90deg); }
  .start-title { font-size: var(--text-md); font-weight: 800; }
  .start ol { margin: var(--space-3) 0 0; padding-left: var(--space-5); line-height: 1.8; color: var(--text); }

  .tabs { display: flex; gap: var(--space-2); border-bottom: 1px solid var(--border); flex-wrap: wrap; }
  .tab { background: transparent; border: none; border-bottom: 2px solid transparent; border-radius: 0; color: var(--muted); font-weight: 700; height: auto; padding: var(--space-3) var(--space-4); }
  .tab:hover:not(:disabled) { filter: none; color: var(--text); }
  .tab.active { color: var(--brand); border-bottom-color: var(--brand); }

  .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: var(--space-4); align-items: start; }
  .conn { max-width: 560px; }
  .checkbox-row { display: flex; align-items: center; gap: var(--space-2); margin: var(--space-4) 0 0; font-weight: 600; }
  .tz-note { margin-top: var(--space-2); }

  .env-switch { display: flex; gap: var(--space-2); flex-wrap: wrap; }
  .env-btn { background: var(--surface-2); border: 1px solid var(--border-strong); color: var(--text); font-weight: 700; }
  .env-btn.active { background: var(--brand); border-color: var(--brand); color: var(--on-brand); }
  .env-btn .tag { font-weight: 600; opacity: 0.75; }
  .env-btn .tag.on { opacity: 1; }

  .bot-status { border: 1px solid var(--border); border-left: 3px solid var(--amber); border-radius: var(--radius-md); background: var(--surface-2); padding: var(--space-3) var(--space-4); margin-bottom: var(--space-4); }
  .bot-status.on { border-left-color: var(--green); }
  .bot-head { display: flex; align-items: center; gap: var(--space-2); margin-bottom: var(--space-2); }
  .bot-status p { margin-top: var(--space-1); line-height: 1.5; }
  .bot-status .next { color: var(--brand-soft); font-weight: 600; font-size: var(--text-sm); }
  .bot-status .warn { color: var(--amber); font-size: var(--text-sm); }

  .ops-header { flex-direction: row; align-items: center; justify-content: space-between; gap: var(--space-3); flex-wrap: wrap; }
  .subtabs { display: flex; gap: var(--space-1); }
  .subtab { background: var(--surface-2); border: 1px solid var(--border); color: var(--muted); height: 2rem; padding: 0 var(--space-3); font-size: var(--text-xs); font-weight: 700; border-radius: var(--radius-sm); }
  .subtab.active { background: var(--brand); color: var(--on-brand); border-color: var(--brand); }

  .table { display: flex; flex-direction: column; overflow-x: auto; }
  .trow { display: grid; grid-template-columns: 1fr 1fr 1fr 1.2fr 1.2fr 1.3fr 1.4fr 140px; gap: var(--space-2); padding: var(--space-3) var(--space-1); border-bottom: 1px solid var(--border); align-items: center; font-size: var(--text-sm); min-width: 900px; }
  .hrow { display: grid; grid-template-columns: 1.6fr 0.9fr 0.8fr 0.9fr 1fr 0.9fr 1fr 70px; gap: var(--space-2); padding: var(--space-3) var(--space-1); border-bottom: 1px solid var(--border); align-items: center; font-size: var(--text-sm); min-width: 820px; }
  .thead { color: var(--muted); font-weight: 700; font-size: var(--text-xs); }
  .col-actions { text-align: right; }
  .ops-actions { display: flex; flex-direction: column; gap: var(--space-1); align-items: flex-end; }
  .sell-cell { display: flex; align-items: center; gap: var(--space-2); }
  .sell-cell .gtc { font-size: var(--text-xs); }
  .by-badge { padding: 2px var(--space-2); border-radius: var(--radius-pill); font-weight: 700; font-size: var(--text-xs); white-space: nowrap; }
  .by-badge.bot { background: rgba(151, 117, 250, 0.18); color: #b197fc; }
  .by-badge.user { background: rgba(77, 171, 247, 0.18); color: #74c0fc; }
</style>
