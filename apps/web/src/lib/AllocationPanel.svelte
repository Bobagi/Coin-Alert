<script lang="ts">
  import { onDestroy } from 'svelte'
  import {
    Chart,
    DoughnutController,
    ArcElement,
    LineController,
    LineElement,
    PointElement,
    LinearScale,
    CategoryScale,
    Filler,
    Tooltip
  } from 'chart.js'
  import { api, type Operation } from './api'
  import { t, locale } from './i18n'

  export let operations: Operation[] = []

  Chart.register(DoughnutController, ArcElement, LineController, LineElement, PointElement, LinearScale, CategoryScale, Filler, Tooltip)

  const palette = ['#ffd43b', '#adb5bd', '#9775fa', '#2bd66a', '#ff922b', '#4dabf7', '#ff5a5f', '#f783ac']
  const periods: Array<'24h' | '7d' | '1M' | '3M'> = ['24h', '7d', '1M', '3M']
  // Quote assets, longest first so e.g. "USDT" matches before "USD".
  const quoteAssets = ['USDT', 'FDUSD', 'BUSD', 'USDC', 'TUSD', 'BRL', 'EUR', 'GBP', 'TRY', 'USD', 'BTC', 'ETH', 'BNB']

  type Holding = { symbol: string; base: string; quote: string; quantity: number; price: number; value: number; color: string }

  let holdings: Holding[] = []
  let total = 0
  let selectedSymbol = ''
  let selectedPeriod: '24h' | '7d' | '1M' | '3M' = '24h'
  let seriesPoints: { t: number; close: number }[] = []
  let seriesLoading = false

  let donutCanvas: HTMLCanvasElement
  let lineCanvas: HTMLCanvasElement
  let donutChart: Chart | null = null
  let lineChart: Chart | null = null

  let lastHoldingsKey = ''
  let lastSeriesKey = ''

  $: hasHoldings = holdings.length > 0
  $: selectedHolding = holdings.find((holding) => holding.symbol === selectedSymbol) || null
  $: mainQuote = holdings.length ? holdings[0].quote : ''
  $: changePercent =
    seriesPoints.length >= 2 ? ((seriesPoints[seriesPoints.length - 1].close - seriesPoints[0].close) / seriesPoints[0].close) * 100 : 0

  function splitSymbol(symbol: string) {
    for (const quote of quoteAssets) {
      if (symbol.endsWith(quote) && symbol.length > quote.length) return { base: symbol.slice(0, -quote.length), quote }
    }
    return { base: symbol, quote: '' }
  }

  function formatMoney(value: number, quote: string) {
    const fiat: Record<string, string> = { BRL: 'BRL', EUR: 'EUR', GBP: 'GBP', TRY: 'TRY', USDT: 'USD', USDC: 'USD', BUSD: 'USD', FDUSD: 'USD', TUSD: 'USD', USD: 'USD' }
    if (fiat[quote]) {
      return new Intl.NumberFormat($locale, { style: 'currency', currency: fiat[quote] }).format(value)
    }
    return value.toLocaleString($locale, { maximumFractionDigits: 8 }) + (quote ? ' ' + quote : '')
  }

  function selectSymbol(symbol: string) {
    selectedSymbol = symbol
  }

  async function loadHoldings() {
    const quantityBySymbol = new Map<string, number>()
    for (const operation of operations) {
      if (operation.status !== 'OPEN') continue
      quantityBySymbol.set(operation.symbol, (quantityBySymbol.get(operation.symbol) || 0) + operation.quantity)
    }
    const symbols = [...quantityBySymbol.keys()]
    const priceResults = await Promise.all(
      symbols.map(async (symbol) => {
        try {
          return { symbol, price: (await api.getPrice(symbol)).price }
        } catch {
          return { symbol, price: 0 }
        }
      })
    )
    const priceBySymbol = new Map(priceResults.map((result) => [result.symbol, result.price]))

    holdings = symbols
      .map((symbol) => {
        const { base, quote } = splitSymbol(symbol)
        const quantity = quantityBySymbol.get(symbol) || 0
        const price = priceBySymbol.get(symbol) || 0
        return { symbol, base, quote, quantity, price, value: quantity * price, color: '' }
      })
      .sort((first, second) => second.value - first.value)
      .map((holding, index) => ({ ...holding, color: palette[index % palette.length] }))

    total = holdings.reduce((sum, holding) => sum + holding.value, 0)
    if (!selectedSymbol || !holdings.find((holding) => holding.symbol === selectedSymbol)) {
      selectedSymbol = holdings.length ? holdings[0].symbol : ''
    }
  }

  async function loadSeries() {
    if (!selectedSymbol) {
      seriesPoints = []
      return
    }
    seriesLoading = true
    try {
      seriesPoints = (await api.getKlines(selectedSymbol, selectedPeriod)).points || []
    } catch {
      seriesPoints = []
    } finally {
      seriesLoading = false
    }
  }

  function renderDonut() {
    if (!donutCanvas) return
    if (donutChart) donutChart.destroy()
    donutChart = new Chart(donutCanvas, {
      type: 'doughnut',
      data: {
        labels: holdings.map((holding) => holding.base),
        datasets: [{ data: holdings.map((holding) => holding.value), backgroundColor: holdings.map((holding) => holding.color), borderColor: '#15130d', borderWidth: 2 }]
      },
      options: {
        cutout: '64%',
        maintainAspectRatio: false,
        plugins: {
          tooltip: {
            callbacks: {
              label: (context) => {
                const percent = total > 0 ? (Number(context.parsed) / total) * 100 : 0
                return ` ${context.label}: ${formatMoney(Number(context.parsed), mainQuote)} (${percent.toFixed(1)}%)`
              }
            }
          }
        }
      }
    })
  }

  function formatTimeLabel(timestamp: number) {
    const date = new Date(timestamp)
    if (selectedPeriod === '24h') return date.toLocaleTimeString($locale, { hour: '2-digit', minute: '2-digit' })
    return date.toLocaleDateString($locale, { day: '2-digit', month: '2-digit' })
  }

  function renderLine() {
    if (!lineCanvas || !selectedHolding || seriesPoints.length < 2) return
    if (lineChart) lineChart.destroy()
    // The chart shows the COIN'S PRICE over the period (not quantity × price); the header and pills
    // are what reflect how much of it you hold.
    const values = seriesPoints.map((point) => point.close)
    const lastIndex = values.length - 1
    lineChart = new Chart(lineCanvas, {
      type: 'line',
      data: {
        labels: seriesPoints.map((point) => formatTimeLabel(point.t)),
        datasets: [
          {
            data: values,
            borderColor: '#ffd43b',
            backgroundColor: 'rgba(255, 212, 59, 0.10)',
            borderWidth: 2,
            fill: true,
            tension: 0.25,
            pointRadius: values.map((_, index) => (index === lastIndex ? 3.5 : 0)),
            pointBackgroundColor: '#ffd43b'
          }
        ]
      },
      options: {
        maintainAspectRatio: false,
        plugins: {
          tooltip: {
            callbacks: {
              label: (context) => ' ' + formatMoney(Number(context.parsed.y), selectedHolding ? selectedHolding.quote : '')
            }
          }
        },
        scales: {
          x: { grid: { display: false }, ticks: { color: '#b8ad8a', maxTicksLimit: 6, font: { size: 10 } } },
          y: {
            grid: { color: 'rgba(255,255,255,0.05)' },
            ticks: {
              color: '#b8ad8a',
              maxTicksLimit: 4,
              font: { size: 10 },
              callback: (value) => Number(value).toLocaleString($locale, { notation: 'compact', maximumFractionDigits: 1 })
            }
          }
        }
      }
    })
  }

  // Reload holdings only when the open positions actually change.
  $: {
    const holdingsKey = operations
      .filter((operation) => operation.status === 'OPEN')
      .map((operation) => operation.symbol + ':' + operation.quantity)
      .sort()
      .join('|')
    if (holdingsKey !== lastHoldingsKey) {
      lastHoldingsKey = holdingsKey
      loadHoldings()
    }
  }

  // Reload the price series when the selected coin or period changes.
  $: {
    const seriesKey = selectedSymbol + '|' + selectedPeriod
    if (selectedSymbol && seriesKey !== lastSeriesKey) {
      lastSeriesKey = seriesKey
      loadSeries()
    }
  }

  $: if (donutCanvas && holdings.length) renderDonut()
  $: if (lineCanvas && seriesPoints.length >= 2 && selectedHolding) renderLine()

  onDestroy(() => {
    donutChart?.destroy()
    lineChart?.destroy()
  })
</script>

{#if !hasHoldings}
  <p class="muted mt-3">{$t('alloc.none')}</p>
{:else}
  <div class="alloc-cols mt-3">
    <div class="donut-col">
      <div class="donut-wrap"><canvas bind:this={donutCanvas}></canvas></div>
      <div class="wallet-total">
        <div class="muted">{$t('alloc.walletTotal')}</div>
        <div class="total-value">{formatMoney(total, mainQuote)}</div>
      </div>
      <div class="legend">
        {#each holdings as holding}
          <button type="button" class="legend-row" class:active={holding.symbol === selectedSymbol} on:click={() => selectSymbol(holding.symbol)}>
            <span class="dot" style="background:{holding.color}"></span>
            <span class="lname">{holding.base}</span>
            <span class="lpct">{total > 0 ? ((holding.value / total) * 100).toFixed(1) : '0.0'}%</span>
          </button>
        {/each}
      </div>
    </div>

    <div class="detail-col">
      <div class="detail-head">
        <div class="sel-info">
          <span class="sel-name">{selectedHolding?.base}<span class="muted"> · {selectedSymbol}</span></span>
          {#if seriesPoints.length >= 2}
            <span class="chg {changePercent >= 0 ? 'up' : 'down'}">{changePercent >= 0 ? '▲' : '▼'} {Math.abs(changePercent).toFixed(1)}%</span>
          {/if}
        </div>
        <div class="periods">
          {#each periods as period}
            <button type="button" class="period-btn" class:active={selectedPeriod === period} on:click={() => (selectedPeriod = period)}>{period}</button>
          {/each}
        </div>
      </div>

      <div class="sel-value">{selectedHolding ? formatMoney(selectedHolding.value, selectedHolding.quote) : '—'}</div>

      <div class="line-wrap">
        {#if seriesLoading}
          <div class="line-state muted">{$t('app.loading')}</div>
        {:else if seriesPoints.length < 2}
          <div class="line-state muted">{$t('alloc.noHistory')}</div>
        {:else}
          <canvas bind:this={lineCanvas}></canvas>
        {/if}
      </div>

      <div class="coinpills">
        {#each holdings as holding}
          <button type="button" class="coinpill" class:active={holding.symbol === selectedSymbol} on:click={() => selectSymbol(holding.symbol)}>
            <span class="dot" style="background:{holding.color}"></span>
            {holding.base} · {formatMoney(holding.value, holding.quote)}
          </button>
        {/each}
      </div>
    </div>
  </div>
{/if}

<style>
  .alloc-cols { display: grid; grid-template-columns: 300px 1fr; gap: var(--space-5); align-items: start; }
  @media (max-width: 760px) { .alloc-cols { grid-template-columns: 1fr; } }

  .donut-col { display: flex; flex-direction: column; align-items: center; gap: var(--space-3); }
  .donut-wrap { position: relative; width: 100%; max-width: 220px; height: 200px; }
  .wallet-total { text-align: center; }
  .wallet-total .total-value { font-weight: 800; font-size: var(--text-lg); color: var(--brand); }

  .legend { width: 100%; display: flex; flex-direction: column; }
  .legend-row {
    display: grid; grid-template-columns: auto 1fr auto; align-items: center; gap: var(--space-3);
    width: 100%; height: auto; padding: var(--space-2) var(--space-2);
    background: transparent; border: none; border-top: 1px solid var(--border);
    color: var(--text); font-size: var(--text-sm); font-weight: 600; text-align: left; cursor: pointer;
  }
  .legend-row:hover { filter: none; background: var(--surface-2); }
  .legend-row.active { color: var(--brand); }
  .legend-row .lpct { color: var(--brand); font-weight: 700; }
  .dot { width: 11px; height: 11px; border-radius: 50%; flex: 0 0 auto; }

  .detail-head { display: flex; align-items: center; justify-content: space-between; gap: var(--space-3); flex-wrap: wrap; }
  .sel-info { display: flex; align-items: center; gap: var(--space-3); flex-wrap: wrap; }
  .sel-name { font-size: var(--text-lg); font-weight: 800; }
  .chg { font-size: var(--text-sm); font-weight: 700; padding: 2px var(--space-2); border-radius: var(--radius-sm); border: 1px solid; }
  .chg.up { color: var(--green); border-color: rgba(43, 214, 106, 0.5); background: rgba(43, 214, 106, 0.1); }
  .chg.down { color: var(--red); border-color: rgba(255, 90, 95, 0.5); background: rgba(255, 90, 95, 0.1); }
  .sel-value { font-size: var(--text-2xl); font-weight: 800; margin-top: var(--space-1); }

  .periods { display: flex; gap: var(--space-1); }
  .period-btn { height: 2rem; padding: 0 var(--space-3); background: var(--surface-2); border: 1px solid var(--border-strong); color: var(--muted); border-radius: var(--radius-sm); font-size: var(--text-xs); font-weight: 700; }
  .period-btn.active { background: var(--brand); border-color: var(--brand); color: var(--on-brand); }

  .line-wrap { position: relative; height: 220px; margin-top: var(--space-4); }
  .line-state { position: absolute; inset: 0; display: grid; place-items: center; }

  .coinpills { display: flex; gap: var(--space-2); flex-wrap: wrap; margin-top: var(--space-4); }
  .coinpill { display: inline-flex; align-items: center; gap: var(--space-2); height: auto; padding: var(--space-2) var(--space-3); background: var(--surface-2); border: 1px solid var(--border-strong); color: var(--text); border-radius: var(--radius-pill); font-size: var(--text-sm); font-weight: 600; }
  .coinpill.active { background: var(--brand); border-color: var(--brand); color: var(--on-brand); }
</style>
