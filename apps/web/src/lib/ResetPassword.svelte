<script lang="ts">
  import { onMount } from 'svelte'
  import { api } from './api'
  import { hashToken, navigate, currentUser } from './stores'
  import { t } from './i18n'
  import LanguageDropdown from './LanguageDropdown.svelte'

  let token = ''
  let newPassword = ''
  let confirmPassword = ''
  let busy = false
  let error = ''
  let done = false

  onMount(() => {
    token = hashToken()
  })

  async function submit() {
    error = ''
    if (newPassword !== confirmPassword) {
      error = $t('account.password.mismatch')
      return
    }
    busy = true
    try {
      await api.resetPassword(token, newPassword)
      done = true
      currentUser.set(null) // the reset revoked all sessions server-side
    } catch (e) {
      error = (e as Error).message
    } finally {
      busy = false
    }
  }

  function goToSignIn() {
    navigate('dashboard')
  }
</script>

<div class="wrap">
  <div class="card auth">
    <div class="top">
      <div class="brand">Coin<span>Hub</span></div>
      <LanguageDropdown compact />
    </div>

    {#if !token}
      <h1 class="title">{$t('reset.title')}</h1>
      <p class="error mt-3">{$t('reset.invalid')}</p>
      <button class="btn-block mt-4" on:click={goToSignIn}>{$t('reset.toSignIn')}</button>
    {:else if done}
      <h1 class="title">{$t('reset.doneTitle')}</h1>
      <p class="muted mt-2">{$t('reset.doneText')}</p>
      <button class="btn-block mt-5" on:click={goToSignIn}>{$t('reset.toSignIn')}</button>
    {:else}
      <h1 class="title">{$t('reset.title')}</h1>
      <form on:submit|preventDefault={submit}>
        <div class="field">
          <label for="reset-new">{$t('account.password.new')}</label>
          <input id="reset-new" type="password" bind:value={newPassword} required placeholder={$t('login.passwordPlaceholder')} autocomplete="new-password" />
        </div>
        <div class="field">
          <label for="reset-confirm">{$t('account.password.confirm')}</label>
          <input id="reset-confirm" type="password" bind:value={confirmPassword} required autocomplete="new-password" />
        </div>
        {#if error}<p class="error mt-3">{error}</p>{/if}
        <button type="submit" class="btn-block mt-5" disabled={busy || !newPassword}>
          {busy ? $t('login.wait') : $t('reset.submit')}
        </button>
      </form>
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
