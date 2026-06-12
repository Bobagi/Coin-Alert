<script lang="ts">
  // A single, styled modal mounted once at the app root. Driven by the `appModal` store, it replaces
  // window.alert: 'verify' shows the confirm-your-email dialog with a resend button; 'message' shows
  // an arbitrary notice (e.g. a locked-screen message).
  import { appModal, closeModal } from './stores'
  import { api } from './api'
  import { t } from './i18n'

  let busy = false
  let resendMessage = ''

  async function resend() {
    busy = true
    resendMessage = ''
    try {
      resendMessage = (await api.resendVerification()).message
    } catch (e) {
      resendMessage = (e as Error).message
    } finally {
      busy = false
    }
  }

  function close() {
    resendMessage = ''
    closeModal()
  }
</script>

{#if $appModal}
  <div class="backdrop" role="presentation" on:click={close}>
    <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
    <div class="modal-card" role="dialog" aria-modal="true" on:click|stopPropagation on:keydown|stopPropagation>
      {#if $appModal.type === 'verify'}
        <span class="micon" aria-hidden="true">✉️</span>
        <h2 class="mtitle">{$t('wall.title')}</h2>
        <p class="mtext">{$t('verify.bannerText')}</p>
        {#if resendMessage}<p class="success">{resendMessage}</p>{/if}
        <div class="mactions">
          <button class="ghost btn-sm" disabled={busy} on:click={resend}>{busy ? $t('common.saving') : $t('verify.resend')}</button>
          <button class="btn-sm" on:click={close}>{$t('modal.ok')}</button>
        </div>
      {:else}
        <span class="micon" aria-hidden="true">🔒</span>
        <p class="mtext">{$appModal.text}</p>
        <div class="mactions"><button class="btn-sm" on:click={close}>{$t('modal.ok')}</button></div>
      {/if}
    </div>
  </div>
{/if}

<style>
  .backdrop { position: fixed; inset: 0; z-index: 100; background: rgba(0, 0, 0, 0.55); display: grid; place-items: center; padding: var(--space-5); }
  .modal-card {
    background: var(--surface); border: 1px solid var(--border-strong); border-radius: var(--radius-md);
    padding: var(--space-5); max-width: 30rem; width: 100%; text-align: center;
    box-shadow: 0 16px 48px rgba(0, 0, 0, 0.5); display: flex; flex-direction: column; gap: var(--space-3); align-items: center;
  }
  .micon { font-size: 2rem; }
  .mtitle { font-size: var(--text-lg); margin: 0; }
  .mtext { color: var(--text); line-height: 1.5; margin: 0; }
  .mactions { display: flex; gap: var(--space-2); justify-content: center; flex-wrap: wrap; margin-top: var(--space-2); }
</style>
