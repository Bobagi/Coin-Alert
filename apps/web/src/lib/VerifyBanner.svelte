<script lang="ts">
  import { api } from './api'
  import { t } from './i18n'

  let busy = false
  let message = ''

  async function resend() {
    busy = true
    message = ''
    try {
      message = (await api.resendVerification()).message
    } catch (e) {
      message = (e as Error).message
    } finally {
      busy = false
    }
  }
</script>

<div class="verify-banner-outer">
  <div class="verify-banner">
    <span class="vb-text">⚠ {$t('verify.bannerText')}</span>
    <button class="btn-sm vb-action" disabled={busy} on:click={resend}>{busy ? $t('common.saving') : $t('verify.resend')}</button>
    {#if message}<span class="muted vb-msg">{message}</span>{/if}
  </div>
</div>

<style>
  .verify-banner-outer { max-width: var(--page-max); margin: 0 auto; padding: var(--space-4) var(--space-5) 0; }
  .verify-banner {
    display: flex; align-items: center; gap: var(--space-3); flex-wrap: wrap;
    background: rgba(250, 176, 5, 0.12); border: 1px solid rgba(250, 176, 5, 0.35);
    color: var(--amber); border-radius: var(--radius-md); padding: var(--space-3) var(--space-4);
  }
  .vb-text { font-weight: 600; }
  .vb-action { margin-left: auto; }
  .vb-msg { flex-basis: 100%; }
</style>
