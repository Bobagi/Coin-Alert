<script lang="ts">
  // A padlock overlay that covers a tab's content: the content stays visible (dimmed) but no control
  // works; clicking anywhere shows the alert message explaining why it is locked.
  import { showModalMessage } from './stores'

  export let message: string
  export let ctaLabel = ''
  export let onCta: (() => void) | null = null

  function notify() {
    showModalMessage(message)
  }
</script>

<div
  class="lock-overlay"
  role="button"
  tabindex="0"
  on:click={notify}
  on:keydown={(event) => (event.key === 'Enter' || event.key === ' ') && notify()}
>
  <div class="lock-card" on:click|stopPropagation on:keydown|stopPropagation role="presentation">
    <span class="lock-icon" aria-hidden="true">🔒</span>
    <p>{message}</p>
    {#if ctaLabel && onCta}
      <button class="btn-sm" on:click={onCta}>{ctaLabel}</button>
    {/if}
  </div>
</div>

<style>
  .lock-overlay {
    position: absolute;
    inset: 0;
    z-index: 5;
    display: flex;
    align-items: flex-start;
    justify-content: center;
    padding: 3rem var(--space-4) 0;
    cursor: not-allowed;
  }
  .lock-card {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-3);
    text-align: center;
    max-width: 30rem;
    background: var(--surface);
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-md);
    padding: var(--space-5);
    box-shadow: var(--shadow-lg, 0 10px 30px rgba(0, 0, 0, 0.35));
    cursor: default;
  }
  .lock-icon { font-size: 2rem; }
  .lock-card p { color: var(--text); line-height: 1.5; margin: 0; }
</style>
