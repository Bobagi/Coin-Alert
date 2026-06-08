<script lang="ts">
  import { locale, setLocale, availableLocales, type Locale } from './i18n'
  import Flag from './Flag.svelte'

  // compact = show the short label (PT/EN/ES) in the trigger instead of the full name.
  export let compact = false

  let open = false
  let container: HTMLDivElement

  $: current = availableLocales.find((option) => option.code === $locale) ?? availableLocales[0]

  function choose(code: Locale) {
    setLocale(code)
    open = false
  }

  function onWindowClick(event: MouseEvent) {
    if (open && container && !container.contains(event.target as Node)) open = false
  }
  function onKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape') open = false
  }
</script>

<svelte:window on:click={onWindowClick} on:keydown={onKeydown} />

<div class="lang" bind:this={container}>
  <button
    type="button"
    class="ghost trigger"
    aria-haspopup="listbox"
    aria-expanded={open}
    on:click|stopPropagation={() => (open = !open)}
  >
    <Flag code={current.code} size={20} />
    <span class="name">{compact ? current.label : current.name}</span>
    <span class="caret" class:up={open}>▾</span>
  </button>

  {#if open}
    <div class="menu" role="listbox">
      {#each availableLocales as option}
        <button
          type="button"
          class="menu-item"
          class:active={option.code === $locale}
          role="option"
          aria-selected={option.code === $locale}
          on:click={() => choose(option.code)}
        >
          <Flag code={option.code} size={20} />
          <span>{option.name}</span>
          {#if option.code === $locale}<span class="check">✓</span>{/if}
        </button>
      {/each}
    </div>
  {/if}
</div>

<style>
  .lang { position: relative; display: inline-block; }
  .trigger { gap: var(--space-2); }
  .caret { font-size: 0.7em; transition: transform 0.15s ease; }
  .caret.up { transform: rotate(180deg); }
  .menu { right: 0; top: calc(100% + 6px); }
  .menu-item .check { margin-left: auto; color: var(--brand); }
</style>
