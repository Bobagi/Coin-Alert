<script lang="ts">
  import { locale, setLocale, availableLocales } from './i18n'
</script>

<div class="langswitch" aria-label="Language">
  {#each availableLocales as option}
    <button class:active={$locale === option.code} on:click={() => setLocale(option.code)}>{option.label}</button>
  {/each}
</div>
