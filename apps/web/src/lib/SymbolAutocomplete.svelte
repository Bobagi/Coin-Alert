<script lang="ts">
  import { createEventDispatcher } from 'svelte'

  export let value = ''
  export let options: string[] = []
  export let id = ''
  export let placeholder = ''

  const dispatch = createEventDispatcher()
  let open = false
  let highlightIndex = -1
  let container: HTMLDivElement

  $: filtered = filterSymbols(value, options)

  function filterSymbols(query: string, allSymbols: string[]) {
    if (!allSymbols || allSymbols.length === 0) return []
    const normalized = (query || '').toUpperCase().trim()
    const matches = normalized ? allSymbols.filter((symbol) => symbol.includes(normalized)) : allSymbols
    return matches.slice(0, 30)
  }

  function choose(symbol: string) {
    value = symbol
    open = false
    highlightIndex = -1
    dispatch('select', symbol)
  }

  function onKeydown(event: KeyboardEvent) {
    if (event.key === 'ArrowDown') {
      open = true
      highlightIndex = Math.min(highlightIndex + 1, filtered.length - 1)
      event.preventDefault()
    } else if (event.key === 'ArrowUp') {
      highlightIndex = Math.max(highlightIndex - 1, 0)
      event.preventDefault()
    } else if (event.key === 'Enter' && open && highlightIndex >= 0 && filtered[highlightIndex]) {
      choose(filtered[highlightIndex])
      event.preventDefault()
    } else if (event.key === 'Escape') {
      open = false
    }
  }

  function onBlur() {
    // Delay so a click on a suggestion registers before the list closes.
    setTimeout(() => {
      open = false
      value = (value || '').toUpperCase()
      dispatch('commit', value)
    }, 120)
  }

  function onWindowClick(event: MouseEvent) {
    if (open && container && !container.contains(event.target as Node)) open = false
  }
</script>

<svelte:window on:click={onWindowClick} />

<div class="ac" bind:this={container}>
  <input
    {id}
    {placeholder}
    bind:value
    autocomplete="off"
    spellcheck="false"
    on:input={() => (open = true)}
    on:focus={() => (open = true)}
    on:keydown={onKeydown}
    on:blur={onBlur}
  />
  {#if open && filtered.length}
    <div class="menu ac-menu" role="listbox">
      {#each filtered as symbol, index}
        <button
          type="button"
          class="menu-item"
          class:active={index === highlightIndex}
          role="option"
          aria-selected={index === highlightIndex}
          on:click={() => choose(symbol)}
        >
          {symbol}
        </button>
      {/each}
    </div>
  {/if}
</div>

<style>
  .ac { position: relative; }
  .ac-menu { left: 0; right: 0; top: calc(100% + 4px); max-height: 240px; overflow-y: auto; }
</style>
