<script lang="ts">
  import { onMount } from 'svelte'
  import { currentUser, route } from './lib/stores'
  import { api } from './lib/api'
  import { t } from './lib/i18n'
  import Login from './lib/Login.svelte'
  import Dashboard from './lib/Dashboard.svelte'
  import AccountSettings from './lib/AccountSettings.svelte'
  import TopNav from './lib/TopNav.svelte'

  let loading = true

  onMount(async () => {
    try {
      currentUser.set(await api.me())
    } catch {
      currentUser.set(null)
    } finally {
      loading = false
    }
  })
</script>

{#if loading}
  <div class="center muted">{$t('app.loading')}</div>
{:else if $currentUser}
  <TopNav />
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
