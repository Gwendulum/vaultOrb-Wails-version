<script lang="ts">
  import { onMount } from "svelte";
  import { Events, WML } from "@wailsio/runtime";
  import { VaultService } from "../bindings/vault-wails";
  import type { DbRow } from "../bindings/vault-wails/models";

  let time: string = $state("Listening for Time event...");

  let domain = $state("");
  let user = $state("");
  let password = $state("");
  let updatedPassword = $state("");
  let revealedPasswords = $state(new Map<number, string>());

  let resultPassword = $state("");
  let errorMessage = $state("");
  type View = "LOGIN" | "VAULT" | "SETTINGS";
  let currentView = $state<View>("LOGIN");

  let entries = $state<DbRow[]>([]);

  function handleKeyDown(event: KeyboardEvent) {
    const isModifier = event.ctrlKey || event.metaKey;

    if (!isModifier) return;

    if (event.key.toLowerCase() === "z" && !event.shiftKey) {
      event.preventDefault();
      handleUndo();
    }

    if (
      event.key.toLowerCase() === "y" ||
      (event.key.toLowerCase() === "z" && event.shiftKey)
    ) {
      event.preventDefault();
      handleRedo();
    }

    if (event.key.toLowerCase() === "r") {
      event.preventDefault();
      handleRedo();
    }
  }

  async function handleUndo() {
    try {
      errorMessage = "";
      await VaultService.Undo();
      await handleList();
    } catch (err) {
      errorMessage = String(err);
    }
  }

  async function handleRedo() {
    try {
      errorMessage = "";
      await VaultService.Redo();
      await handleList();
    } catch (err) {
      errorMessage = String(err);
    }
  }

  onMount(() => {
    Events.On("time", (v: any) => {
      // On a narrow screen the full RFC1123 stamp is too wide for the footer, so
      // show just the clock time there (matching the CSS breakpoint).
      const full = v.data;
      const compact = (full.match(/\d{1,2}:\d{2}:\d{2}/) || [full])[0];
      time = window.matchMedia("(max-width: 640px)").matches ? compact : full;
    });
    // Wire up data-wml-openURL links (logos + footer "Docs" link).
    WML.Reload();
  });

  async function handleLogin() {
    try {
      errorMessage = "";
      await VaultService.Login(password);
      currentView = "VAULT";
      await handleList();
      password = "";
    } catch (err) {
      errorMessage = String(err);
    }
  }

  async function handleList() {
    try {
      errorMessage = "";
      entries = await VaultService.List();
    } catch (err) {
      errorMessage = String(err);
    }
  }

  async function handleRegister() {
    try {
      console.log("new entry registered");
      errorMessage = "";
      await VaultService.Register(domain, user, password);
      domain = "";
      user = "";
      password = "";
      await handleList();
    } catch (err) {
      errorMessage = String(err);
    }
  }

  async function showPassword(
    id: number,
    entryDomain: string,
    entryUser: string,
  ) {
    try {
      errorMessage = "";
      const pass = await VaultService.Get(entryDomain, entryUser);
      revealedPasswords.set(id, pass);
      await handleList();
    } catch (err) {
      errorMessage = String(err);
    }
  }

  async function handleDelete(entryDomain: string, entryUser: string) {
    try {
      errorMessage = "";
      await VaultService.Delete(entryDomain, entryUser);
      await handleList();
    } catch (err) {
      errorMessage = String(err);
    }
  }

  async function handleUpdate(entryDomain: string, entryUser: string) {
    if (!updatedPassword) return;
    try {
      errorMessage = "";
      await VaultService.Update(entryDomain, entryUser, updatedPassword);
      await handleList();
      editingId = null;
      updatedPassword = "";
    } catch (err) {
      errorMessage = String(err);
    }
  }

  let editingId = $state<number | null>(null);

  function startEditing(id: number) {
    editingId = id;
    updatedPassword = "";
  }

  function cancelEditing() {
    editingId = null;
    updatedPassword = "";
  }
</script>

<svelte:window onkeydown={handleKeyDown} />

<main class="container">
  {#if currentView == "LOGIN"}
    <!-- LOGIN VIEW -->
    <div class="card">
      <h1>Vault Setup</h1>
      <input
        type="password"
        placeholder="Master Password"
        bind:value={password}
      />
      <button onclick={handleLogin}>Unlock</button>

      {#if errorMessage}
        <p class="error">{errorMessage}</p>
      {/if}
    </div>
  {:else if currentView == "VAULT"}
    <!-- VAULT VIEW -->
    <div class="card register-card">
      <h1>Register new entry</h1>
      <div class="input-row">
        <input type="text" placeholder="domain" bind:value={domain} />
        <input type="text" placeholder="user" bind:value={user} />
        <input type="password" placeholder="password" bind:value={password} />
        <button class="btn-primary" onclick={handleRegister}>Register</button>
      </div>
    </div>
    <div class="card table-card">
      <table class="vault-table">
        <thead>
          <tr>
            <th>ID</th>
            <th>Domain</th>
            <th>Username</th>
            <th>Password</th>
            <th>Created</th>
            <th>Updated</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {#each entries as entry}
            <tr>
              <td class="col-id">{entry.id}</td>
              <td class="col-domain">{entry.domain}</td>
              <td class="col-user">{entry.username}</td>
              <td class="col-pass">
                {#if revealedPasswords.has(entry.id)}
                  <span class="password-text"
                    >{revealedPasswords.get(entry.id)}</span
                  >
                {:else}
                  <button
                    class="btn-secondary"
                    onclick={() =>
                      showPassword(entry.id, entry.domain, entry.username)}
                  >
                    Show
                  </button>
                {/if}
              </td>
              <td class="col-date">{entry.created_at}</td>
              <td class="col-date">{entry.updated_at}</td>
              <td class="col-actions">
                {#if editingId === entry.id}
                  <input
                    type="password"
                    placeholder="New password"
                    bind:value={updatedPassword}
                  />
                  <button
                    class="btn-primary"
                    onclick={() => handleUpdate(entry.domain, entry.username)}
                  >
                    Save
                  </button>
                  <button class="btn-secondary" onclick={cancelEditing}
                    >Cancel</button
                  >
                {:else}
                  <button
                    class="btn-primary"
                    onclick={() => startEditing(entry.id)}>⬆</button
                  >
                  <button
                    class="btn-danger"
                    onclick={() => handleDelete(entry.domain, entry.username)}
                    >❌</button
                  >
                {/if}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</main>

<footer class="footer">
  <span>{time}</span>
</footer>

<style>
  /* Page layout container */
  .container {
    display: flex;
    flex-direction: column; /* Stack register card and vault table top-to-bottom */
    justify-content: flex-start; /* Align contents to the top of the viewport */
    align-items: center; /* Horizontally center cards */
    min-height: calc(100vh - 60px); /* Span full window minus footer height */
    padding: 1.5rem;
    box-sizing: border-box;
    gap: 0.5rem; /* 24px space between cards */
  }

  /* Shared structure for all card blocks */
  .card {
    background-color: #181825;
    border: 1px solid #313244;
    border-radius: 10px;
    padding: 1.5rem 2rem;
    width: 100%;
    max-width: 1100px;
    box-sizing: border-box;
  }

  /* Horizontal layout row for registration inputs */
  .input-row {
    display: flex;
    gap: 1rem;
  }

  /* Specific card adjustments for the entries table */
  .table-card {
    flex: 1;
    max-height: calc(100vh - 280px); /* Keeps table within window bounds */
    overflow-y: auto; /* Internal scrollbar if list grows long */
  }

  /* Table structure styling */
  .vault-table {
    width: 100%;
    border-collapse: collapse;
  }

  .vault-table th,
  .vault-table td {
    padding: 0.8rem;
    text-align: left;
    border-bottom: 1px solid #313244;
    vertical-align: middle;
  }

  .vault-table th {
    color: #a6adc8;
    font-size: 0.85rem;
    text-transform: uppercase;
  }

  .col-id {
    width: 50px;
  }
  .col-date {
    font-size: 0.85rem;
    color: #a6adc8;
  }
  .col-actions {
    text-align: right;
  }
  .col-actions button {
    margin-left: 0.4rem;
  }

  .password-text {
    font-family: monospace;
    color: #a6e3a1;
  }

  /* Footer bar layout */
  .footer {
    height: 40px;
    position: absolute;
    bottom: 0;
    width: 100%;
    display: flex;
    justify-content: center;
    align-items: center;
    border-top: 1px solid #313244;
    color: #a6adc8;
    font-size: 0.85rem;
  }
</style>
