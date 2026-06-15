<script lang="ts">
  import { onDestroy } from 'svelte'
  import { Chart, LineController, LineElement, PointElement, LinearScale, CategoryScale, Filler, Tooltip } from 'chart.js'
  import { api, type Operation } from './api'
  import { t, intlLocale } from './i18n'

  export let operations: Operation[] = []

  Chart.register(LineController, LineElement, PointElement, LinearScale, CategoryScale, Filler, Tooltip)

  const periods: Array<'24h' | '7d' | '1M' | '3M'> = ['24h', '7d', '1M', '3M']
  const quoteAssets = ['USDT', 'FDUSD', 'BUSD', 'USDC', 'TUSD', 'BRL', 'EUR', 'GBP', 'TRY', 'USD', 'BTC', 'ETH', 'BNB']

  type Position = {
    symbol: string; base: string; quote: string; quantity: number
    avgCost: number; price: number; cost: number; value: number; pnl: number; pnlPct: number
  }

  let positions: Position[] = []
  let selectedSymbol = ''
  let selectedPeriod: '24h' | '7d' | '1M' | '3M' = '24h'
  let seriesPoints: { t: number; close: number }[] = []
  let seriesLoading = false
  let lineCanvas: HTMLCanvasElement
  let lineChart: Chart | null = null
  let lastPositionsKey = ''
  let lastSeriesKey = ''

  $: hasPositions = positions.length > 0
  $: selected = positions.find((position) => position.symbol === selectedSymbol) || null
  $: totalCost = positions.reduce((sum, position) => sum + position.cost, 0)
  $: totalValue = positions.reduce((sum, position) => sum + position.value, 0)
  $: totalPnl = totalValue - totalCost
  $: totalPnlPct = totalCost > 0 ? (totalPnl / totalCost) * 100 : 0
  $: mainQuote = positions.length ? positions[0].quote : ''

  function splitSymbol(symbol: string) {
    for (const quote of quoteAssets) {
      if (symbol.endsWith(quote) && symbol.length > quote.length) return { base: symbol.slice(0, -quote.length), quote }
    }
    return { base: symbol, quote: '' }
  }

  function formatMoney(value: number, quote: string) {
    const fiat: Record<string, string> = { BRL: 'BRL', EUR: 'EUR', GBP: 'GBP', TRY: 'TRY', USDT: 'USD', USDC: 'USD', BUSD: 'USD', FDUSD: 'USD', TUSD: 'USD', USD: 'USD' }
    if (fiat[quote]) return new Intl.NumberFormat($intlLocale, { style: 'currency', currency: fiat[quote] }).format(value)
    return value.toLocaleString($intlLocale, { maximumFractionDigits: 8 }) + (quote ? ' ' + quote : '')
  }

  function formatTimeLabel(timestamp: number) {
    const date = new Date(timestamp)
    if (selectedPeriod === '24h') return date.toLocaleTimeString($intlLocale, { hour: '2-digit', minute: '2-digit' })
    return date.toLocaleDateString($intlLocale, { day: '2-digit', month: '2-digit' })
  }

  async function loadPositions() {
    const aggregate = new Map<string, { quantity: number; cost: number }>()
    for (const operation of operations) {
      if (operation.status !== 'OPEN') continue
      const entry = aggregate.get(operation.symbol) || { quantity: 0, cost: 0 }
      entry.quantity += operation.quantity
      entry.cost += operation.quantity * operation.purchase_price_per_unit
      aggregate.set(operation.symbol, entry)
    }
    const symbols = [...aggregate.keys()]
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

    positions = symbols
      .map((symbol) => {
        const { base, quote } = splitSymbol(symbol)
        const { quantity, cost } = aggregate.get(symbol) as { quantity: number; cost: number }
        const price = priceBySymbol.get(symbol) || 0
        const avgCost = quantity > 0 ? cost / quantity : 0
        const value = quantity * price
        const pnl = value - cost
        return { symbol, base, quote, quantity, avgCost, price, cost, value, pnl, pnlPct: cost > 0 ? (pnl / cost) * 100 : 0 }
      })
      .sort((first, second) => second.value - first.value)

    if (!selectedSymbol || !positions.find((position) => position.symbol === selectedSymbol)) {
      selectedSymbol = positions.length ? positions[0].symbol : ''
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

  function renderLine() {
    if (!lineCanvas || !selected || seriesPoints.length < 2) return
    if (lineChart) lineChart.destroy()
    const priceValues = seriesPoints.map((point) => point.close)
    const averageCost = selected.avgCost
    const inProfit = selected.price >= averageCost
    const priceColor = inProfit ? '#2bd66a' : '#ff5a5f'
    const fillColor = inProfit ? 'rgba(43, 214, 106, 0.10)' : 'rgba(255, 90, 95, 0.10)'

    lineChart = new Chart(lineCanvas, {
      type: 'line',
      data: {
        labels: seriesPoints.map((point) => formatTimeLabel(point.t)),
        datasets: [
          {
            label: 'price',
            data: priceValues,
            borderColor: priceColor,
            backgroundColor: fillColor,
            borderWidth: 2,
            fill: true,
            tension: 0.25,
            pointRadius: priceValues.map((_, index) => (index === priceValues.length - 1 ? 3.5 : 0)),
            pointHoverRadius: 4.5,
            pointHitRadius: 24,
            pointBackgroundColor: priceColor
          },
          {
            label: 'avgCost',
            data: priceValues.map(() => averageCost),
            borderColor: '#ffd43b',
            borderDash: [6, 4],
            borderWidth: 1.5,
            fill: false,
            pointRadius: 0,
            pointHitRadius: 0
          }
        ]
      },
      options: {
        maintainAspectRatio: false,
        interaction: { mode: 'index', intersect: false },
        plugins: {
          tooltip: {
            mode: 'index',
            intersect: false,
            callbacks: {
              label: (context) => {
                const prefix = context.dataset.label === 'avgCost' ? $t('prof.avgCost') + ': ' : $t('prof.current') + ': '
                return ' ' + prefix + formatMoney(Number(context.parsed.y), selected ? selected.quote : '')
              }
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
              callback: (value) => Number(value).toLocaleString($intlLocale, { notation: 'compact', maximumFractionDigits: 1 })
            }
          }
        }
      }
    })
  }

  $: {
    const positionsKey = operations
      .filter((operation) => operation.status === 'OPEN')
      .map((operation) => operation.symbol + ':' + operation.quantity + ':' + operation.purchase_price_per_unit)
      .sort()
      .join('|')
    if (positionsKey !== lastPositionsKey) {
      lastPositionsKey = positionsKey
      loadPositions()
    }
  }

  $: {
    const seriesKey = selectedSymbol + '|' + selectedPeriod
    if (selectedSymbol && seriesKey !== lastSeriesKey) {
      lastSeriesKey = seriesKey
      loadSeries()
    }
  }

  $: if (lineCanvas && seriesPoints.length >= 2 && selected) renderLine()

  onDestroy(() => lineChart?.destroy())
</script>

{#if !hasPositions}
  <p class="muted mt-3">{$t('prof.none')}</p>
{:else}
  <div class="prof-summary {totalPnl >= 0 ? 'pos' : 'neg'}">
    <span class="ps-label">{$t('prof.ifSellNow')}</span>
    <span class="ps-value">{totalPnl >= 0 ? '+' : ''}{formatMoney(totalPnl, mainQuote)} · {totalPnl >= 0 ? '+' : ''}{totalPnlPct.toFixed(1)}%</span>
  </div>

  <div class="coinpills mt-3">
    {#each positions as position (position.symbol)}
      <button type="button" class="coinpill" class:active={position.symbol === selectedSymbol} on:click={() => (selectedSymbol = position.symbol)}>
        {position.base} <span class={position.pnl >= 0 ? 'up' : 'down'}>{position.pnl >= 0 ? '+' : ''}{position.pnlPct.toFixed(1)}%</span>
      </button>
    {/each}
  </div>

  {#if selected}
    <div class="detail-head mt-4">
      <span class="sel-name">{selected.base}<span class="muted"> · {selectedSymbol}</span></span>
      <div class="periods">
        {#each periods as period}
          <button type="button" class="period-btn" class:active={selectedPeriod === period} on:click={() => (selectedPeriod = period)}>{period}</button>
        {/each}
      </div>
    </div>
    <div class="prof-meta">
      <span>{$t('prof.avgCost')}: <strong class="cost-val">{formatMoney(selected.avgCost, selected.quote)}</strong></span>
      <span>{$t('prof.current')}: <strong class={selected.pnl >= 0 ? 'up' : 'down'}>{formatMoney(selected.price, selected.quote)}</strong></span>
    </div>
    <div class="line-wrap">
      {#if seriesLoading}
        <div class="line-state muted">{$t('app.loading')}</div>
      {:else if seriesPoints.length < 2}
        <div class="line-state muted">{$t('alloc.noHistory')}</div>
      {:else}
        <canvas bind:this={lineCanvas}></canvas>
      {/if}
    </div>
    <p class="muted prof-chart-hint">{$t('prof.chartHint')}</p>
  {/if}
{/if}

<style>
  .prof-summary { display: flex; flex-direction: column; gap: 2px; border: 1px solid var(--border); border-left: 3px solid var(--green); border-radius: var(--radius-md); background: var(--surface-2); padding: var(--space-3) var(--space-4); }
  .prof-summary.neg { border-left-color: var(--red); }
  .ps-label { color: var(--muted); font-size: var(--text-xs); font-weight: 700; text-transform: uppercase; letter-spacing: 0.02em; }
  .prof-summary.pos .ps-value { color: var(--green); }
  .prof-summary.neg .ps-value { color: var(--red); }
  .ps-value { font-size: var(--text-xl); font-weight: 800; }

  .coinpills { display: flex; gap: var(--space-2); flex-wrap: wrap; }
  .coinpill { display: inline-flex; align-items: center; gap: var(--space-2); height: auto; padding: var(--space-2) var(--space-3); background: var(--surface-2); border: 1px solid var(--border-strong); color: var(--text); border-radius: var(--radius-pill); font-size: var(--text-sm); font-weight: 600; }
  .coinpill.active { background: var(--brand); border-color: var(--brand); color: var(--on-brand); }
  .up { color: var(--green); font-weight: 700; }
  .down { color: var(--red); font-weight: 700; }
  .coinpill.active .up, .coinpill.active .down { color: var(--on-brand); }

  .detail-head { display: flex; align-items: center; justify-content: space-between; gap: var(--space-3); flex-wrap: wrap; }
  .sel-name { font-size: var(--text-lg); font-weight: 800; }
  .periods { display: flex; gap: var(--space-1); }
  .period-btn { height: 2rem; padding: 0 var(--space-3); background: var(--surface-2); border: 1px solid var(--border-strong); color: var(--muted); border-radius: var(--radius-sm); font-size: var(--text-xs); font-weight: 700; }
  .period-btn.active { background: var(--brand); border-color: var(--brand); color: var(--on-brand); }

  .prof-meta { display: flex; gap: var(--space-4); flex-wrap: wrap; margin-top: var(--space-2); font-size: var(--text-sm); color: var(--muted); }
  .cost-val { color: var(--brand); }
  .line-wrap { position: relative; height: 220px; margin-top: var(--space-3); }
  .line-state { position: absolute; inset: 0; display: grid; place-items: center; }
  .prof-chart-hint { margin-top: var(--space-2); font-size: var(--text-xs); }
</style>
