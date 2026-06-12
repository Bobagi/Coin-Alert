<script lang="ts">
  import { api } from './api'
  import { currentUser } from './stores'
  import { t } from './i18n'
  import LanguageDropdown from './LanguageDropdown.svelte'

  let busy = false
  let refreshing = false
  let message = ''

  $: email = $currentUser?.email ?? ''

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

  async function refresh() {
    refreshing = true
    try {
      currentUser.set(await api.me())
    } catch {
      /* ignore */
    } finally {
      refreshing = false
    }
  }

  async function signOut() {
    try {
      await api.logout()
    } catch {
      /* ignore */
    }
    currentUser.set(null)
  }
</script>

<div class="wrap">
  <div class="card auth">
    <div class="top">
      <div class="brand">Coin<span>Hub</span></div>
      <LanguageDropdown compact />
    </div>
    <div class="icon" aria-hidden="true">✉️</div>
    <h1 class="title">{$t('wall.title')}</h1>
    <p class="muted mt-2">{$t('wall.text', { email })}</p>
    {#if message}<p class="success mt-3">{message}</p>{/if}
    <button class="btn-block mt-4" disabled={refreshing} on:click={refresh}>
      {refreshing ? $t('login.wait') : $t('wall.refresh')}
    </button>
    <button class="ghost btn-block mt-3" disabled={busy} on:click={resend}>
      {busy ? $t('common.saving') : $t('verify.resend')}
    </button>
    <button type="button" class="link-btn mt-4" on:click={signOut}>{$t('header.signOut')}</button>
  </div>
</div>

<style>
  .wrap { display: grid; place-items: center; min-height: 100vh; padding: var(--space-5); }
  .auth { width: 100%; max-width: 420px; text-align: center; }
  .top { display: flex; justify-content: space-between; align-items: center; }
  .brand { font-size: var(--text-xl); font-weight: 800; }
  .brand span { color: var(--brand); }
  .icon { font-size: 2.5rem; margin-top: var(--space-4); }
  .title { font-size: var(--text-lg); margin-top: var(--space-2); }
  .link-btn { background: transparent; border: none; color: var(--muted); font-weight: 700; padding: 0; height: auto; cursor: pointer; font-size: var(--text-sm); }
  .link-btn:hover:not(:disabled) { filter: none; text-decoration: underline; }
</style>
