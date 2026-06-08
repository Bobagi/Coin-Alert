<script lang="ts">
  import { onDestroy } from 'svelte'
  import { Chart, DoughnutController, ArcElement, Tooltip } from 'chart.js'
  import { t } from './i18n'
  import type { Operation } from './api'

  export let operations: Operation[] = []

  Chart.register(DoughnutController, ArcElement, Tooltip)

  const palette = ['#ffd43b', '#fab005', '#ff922b', '#2bd66a', '#ff5a5f', '#9775fa', '#4dabf7']

  let canvas: HTMLCanvasElement
  let chart: Chart | null = null
  let legend: { label: string; value: number; percent: number; color: string }[] = []
  let total = 0

  const fmt = (value: number) => value.toLocaleString(undefined, { maximumFractionDigits: 2 })

  function render() {
    if (!canvas) return
    const bySymbol = new Map<string, number>()
    for (const operation of operations) {
      if (operation.status !== 'OPEN') continue
      bySymbol.set(operation.symbol, (bySymbol.get(operation.symbol) || 0) + operation.quantity * operation.purchase_price_per_unit)
    }
    const labels = [...bySymbol.keys()]
    const values = [...bySymbol.values()]
    total = values.reduce((sum, value) => sum + value, 0)
    legend = labels.map((label, index) => ({
      label,
      value: values[index],
      percent: total > 0 ? (values[index] / total) * 100 : 0,
      color: palette[index % palette.length]
    }))

    if (chart) chart.destroy()
    chart = new Chart(canvas, {
      type: 'doughnut',
      data: { labels, datasets: [{ data: values, backgroundColor: palette, borderColor: '#15130d', borderWidth: 2 }] },
      options: {
        cutout: '62%',
        maintainAspectRatio: false,
        plugins: {
          legend: { display: false },
          tooltip: {
            callbacks: {
              label: (context) => {
                const value = Number(context.parsed)
                const percent = total > 0 ? (value / total) * 100 : 0
                return ` ${context.label}: ${fmt(value)} (${percent.toFixed(1)}%)`
              }
            }
          }
        }
      }
    })
  }

  $: if (canvas && operations) render()

  onDestroy(() => chart?.destroy())
</script>

<div class="alloc">
  <div class="chart-wrap"><canvas bind:this={canvas}></canvas></div>
  <div class="legend">
    {#each legend as item}
      <div class="legend-row">
        <span class="dot" style="background:{item.color}"></span>
        <span class="sym">{item.label}</span>
        <span class="val muted">{fmt(item.value)}</span>
        <span class="pct">{item.percent.toFixed(1)}%</span>
      </div>
    {/each}
    <div class="legend-row total">
      <span class="dot hidden"></span>
      <span class="sym">{$t('alloc.total')}</span>
      <span class="val muted">{fmt(total)}</span>
      <span class="pct">100%</span>
    </div>
  </div>
</div>

<style>
  .alloc { display: flex; flex-direction: column; gap: var(--space-4); }
  .chart-wrap { position: relative; height: 200px; width: 100%; max-width: 240px; margin: 0 auto; }
  .legend { display: flex; flex-direction: column; gap: var(--space-2); }
  .legend-row { display: grid; grid-template-columns: auto 1fr auto auto; align-items: center; gap: var(--space-3); font-size: var(--text-sm); }
  .legend-row.total { border-top: 1px solid var(--border); padding-top: var(--space-2); font-weight: 700; }
  .dot { width: 10px; height: 10px; border-radius: 3px; }
  .dot.hidden { visibility: hidden; }
  .sym { font-weight: 600; }
  .val { text-align: right; }
  .pct { font-weight: 700; color: var(--brand); min-width: 52px; text-align: right; }
</style>
