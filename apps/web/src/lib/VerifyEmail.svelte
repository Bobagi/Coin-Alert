<script lang="ts">
  import { onMount } from 'svelte'
  import { api } from './api'
  import { hashToken, navigate, currentUser } from './stores'
  import { t } from './i18n'
  import LanguageDropdown from './LanguageDropdown.svelte'

  let status: 'loading' | 'ok' | 'error' = 'loading'
  let message = ''

  onMount(async () => {
    const token = hashToken()
    if (!token) {
      status = 'error'
      message = $t('verify.invalid')
      return
    }
    try {
      await api.verifyEmail(token)
      status = 'ok'
      // Refresh the session user (if signed in) so the "confirm your email" banner disappears.
      try {
        currentUser.set(await api.me())
      } catch {
        /* not signed in — fine */
      }
    } catch (e) {
      status = 'error'
      message = (e as Error).message
    }
  })

  function go() {
    navigate('dashboard')
  }
</script>

<div class="wrap">
  <div class="card auth">
    <div class="top">
      <div class="brand">Coin<span>Hub</span></div>
      <LanguageDropdown compact />
    </div>

    {#if status === 'loading'}
      <h1 class="title">{$t('verify.title')}</h1>
      <p class="muted mt-3">{$t('verify.checking')}</p>
    {:else if status === 'ok'}
      <h1 class="title">{$t('verify.okTitle')}</h1>
      <p class="muted mt-2">{$t('verify.okText')}</p>
      <button class="btn-block mt-5" on:click={go}>{$t('verify.continue')}</button>
    {:else}
      <h1 class="title">{$t('verify.errorTitle')}</h1>
      <p class="error mt-2">{message}</p>
      <button class="btn-block mt-5" on:click={go}>{$t('verify.continue')}</button>
    {/if}
  </div>
</div>

<style>
  .wrap { display: grid; place-items: center; min-height: 100vh; padding: var(--space-5); }
  .auth { width: 100%; max-width: 400px; }
  .top { display: flex; justify-content: space-between; align-items: center; }
  .brand { font-size: var(--text-xl); font-weight: 800; }
  .brand span { color: var(--brand); }
  .title { font-size: var(--text-lg); margin-top: var(--space-4); }
</style>
