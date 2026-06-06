<script lang="ts">
  import { onDestroy } from 'svelte'
  import { Chart, DoughnutController, ArcElement, Tooltip, Legend } from 'chart.js'
  import type { Operation } from './api'

  export let operations: Operation[] = []

  Chart.register(DoughnutController, ArcElement, Tooltip, Legend)

  const palette = ['#22d3ee', '#6366f1', '#22c55e', '#fbbf24', '#ef4444', '#a855f7', '#14b8a6']

  let canvas: HTMLCanvasElement
  let chart: Chart | null = null

  function buildData(ops: Operation[]) {
    const bySymbol = new Map<string, number>()
    for (const op of ops) {
      if (op.status !== 'OPEN') continue
      bySymbol.set(op.symbol, (bySymbol.get(op.symbol) || 0) + op.quantity * op.purchase_price_per_unit)
    }
    return { labels: [...bySymbol.keys()], values: [...bySymbol.values()] }
  }

  function render() {
    if (!canvas) return
    const { labels, values } = buildData(operations)
    if (chart) chart.destroy()
    chart = new Chart(canvas, {
      type: 'doughnut',
      data: { labels, datasets: [{ data: values, backgroundColor: palette, borderColor: '#0b1120', borderWidth: 2 }] },
      options: { cutout: '62%', plugins: { legend: { position: 'bottom', labels: { color: '#94a3b8' } } } }
    })
  }

  $: if (canvas && operations) render()

  onDestroy(() => chart?.destroy())
</script>

<canvas bind:this={canvas} height="220"></canvas>
