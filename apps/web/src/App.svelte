<script lang="ts">
  import { onMount } from 'svelte'
  import { currentUser, route } from './lib/stores'
  import { api } from './lib/api'
  import { t } from './lib/i18n'
  import Login from './lib/Login.svelte'
  import Dashboard from './lib/Dashboard.svelte'
  import AccountSettings from './lib/AccountSettings.svelte'
  import TopNav from './lib/TopNav.svelte'
  import ResetPassword from './lib/ResetPassword.svelte'
  import VerifyEmail from './lib/VerifyEmail.svelte'
  import VerifyBanner from './lib/VerifyBanner.svelte'

  let loading = true
  let emailEnabled = false

  onMount(async () => {
    try {
      emailEnabled = (await api.getAuthProviders()).email
    } catch {
      emailEnabled = false
    }
    try {
      currentUser.set(await api.me())
    } catch {
      currentUser.set(null)
    } finally {
      loading = false
    }
  })
</script>

{#if $route === 'reset'}
  <ResetPassword />
{:else if $route === 'verify'}
  <VerifyEmail />
{:else if loading}
  <div class="center muted">{$t('app.loading')}</div>
{:else if $currentUser}
  <TopNav />
  {#if emailEnabled && !$currentUser.email_verified}
    <VerifyBanner />
  {/if}
  {#if $route === 'account'}
    <AccountSettings />
  {:else}
    <Dashboard />
  {/if}
{:else}
  <Login />
{/if}

<style>
  .center { display: grid; place-items: center; min-height: 100vh; }
</style>
