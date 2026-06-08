<script lang="ts">
  import type { Locale } from './i18n'

  // Inline SVG flags instead of emoji: emoji flags do NOT render on Windows (they fall back to the
  // two-letter code like "BR"), so we draw them ourselves to look identical on every platform.
  export let code: Locale
  export let size = 20

  $: height = Math.round((size * 2) / 3)
</script>

<span class="flag" style="width:{size}px; height:{height}px" aria-hidden="true">
  {#if code === 'pt'}
    <svg viewBox="0 0 30 20" xmlns="http://www.w3.org/2000/svg">
      <rect width="30" height="20" fill="#009b3a" />
      <polygon points="15,2.4 27.6,10 15,17.6 2.4,10" fill="#ffdf00" />
      <circle cx="15" cy="10" r="4.4" fill="#002776" />
    </svg>
  {:else if code === 'en'}
    <svg viewBox="0 0 30 20" xmlns="http://www.w3.org/2000/svg">
      <rect width="30" height="20" fill="#b22234" />
      <g fill="#fff">
        <rect y="1.54" width="30" height="1.54" />
        <rect y="4.62" width="30" height="1.54" />
        <rect y="7.69" width="30" height="1.54" />
        <rect y="10.77" width="30" height="1.54" />
        <rect y="13.85" width="30" height="1.54" />
        <rect y="16.92" width="30" height="1.54" />
      </g>
      <rect width="12" height="10.77" fill="#3c3b6e" />
      <g fill="#fff">
        <circle cx="2.4" cy="2.2" r="0.6" /><circle cx="6" cy="2.2" r="0.6" /><circle cx="9.6" cy="2.2" r="0.6" />
        <circle cx="4.2" cy="4.5" r="0.6" /><circle cx="7.8" cy="4.5" r="0.6" />
        <circle cx="2.4" cy="6.8" r="0.6" /><circle cx="6" cy="6.8" r="0.6" /><circle cx="9.6" cy="6.8" r="0.6" />
        <circle cx="4.2" cy="9" r="0.6" /><circle cx="7.8" cy="9" r="0.6" />
      </g>
    </svg>
  {:else}
    <svg viewBox="0 0 30 20" xmlns="http://www.w3.org/2000/svg">
      <rect width="30" height="20" fill="#aa151b" />
      <rect y="5" width="30" height="10" fill="#f1bf00" />
    </svg>
  {/if}
</span>

<style>
  .flag {
    display: inline-block;
    border-radius: 3px;
    overflow: hidden;
    line-height: 0;
    flex: 0 0 auto;
    box-shadow: 0 0 0 1px rgba(0, 0, 0, 0.3) inset;
  }
  .flag svg { display: block; width: 100%; height: 100%; }
</style>
