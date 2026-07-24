<script lang="ts">
  import { client } from "~/lib/api";
  import { getErrorMessage } from "~/lib/errors";
  import type {
    Character,
    Game,
    InfoCard,
  } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
  import {
    ACCENT_STYLES,
    customCardToDisplay,
    generateStandardCards,
    type DisplayCard,
    type DisplayCharacter,
  } from "~/lib/info-cards";
  import { iconSuffix } from "~/lib/team-styles";
  import ConfirmDialog from "./ConfirmDialog.svelte";
  import CharacterPickerModal from "./CharacterPickerModal.svelte";

  const MAX_CARD_CHARACTERS = 6;
  const TITLE_MAX = 100;
  const BODY_MAX = 2000;

  let {
    game,
    onshow,
    onclose,
  }: {
    game: Game;
    onshow: (card: DisplayCard, character?: DisplayCharacter | null) => void;
    onclose: () => void;
  } = $props();

  const standardCards = $derived(generateStandardCards(game));
  const inPlayCharacters = $derived<Character[]>([
    ...game.selectedCharacters,
    ...game.selectedTravellerCharacters,
  ]);

  // "My cards" — lazily loaded on open.
  let customCards = $state<InfoCard[]>([]);
  let loadingCards = $state(true);
  let cardsError = $state("");

  // View state: the modal shows one of these at a time.
  let pickForCard = $state<DisplayCard | null>(null);
  let formOpen = $state(false);

  // Inline edit/create form.
  let formId = $state<bigint | null>(null);
  let formTitle = $state("");
  let formBody = $state("");
  let formCharacters = $state<Character[]>([]);
  let formError = $state("");
  let formSaving = $state(false);

  // Character selection (for the form) — the character list is cached.
  let charPickerOpen = $state(false);
  let allCharacters = $state<Character[]>([]);
  let loadingChars = $state(false);

  // Deletion.
  let deleteTarget = $state<InfoCard | null>(null);
  let deleting = $state(false);

  async function loadCards() {
    loadingCards = true;
    cardsError = "";
    try {
      const resp = await client.listInfoCards({});
      customCards = resp.cards;
    } catch (err) {
      cardsError = getErrorMessage(err, "Failed to load your cards");
    } finally {
      loadingCards = false;
    }
  }

  async function ensureCharacters() {
    if (allCharacters.length > 0) return;
    loadingChars = true;
    try {
      const resp = await client.listCharacters({});
      allCharacters = resp.characters;
    } catch (err) {
      formError = getErrorMessage(err, "Failed to load characters");
    } finally {
      loadingChars = false;
    }
  }

  function tapStandard(card: DisplayCard) {
    if (card.needsCharacterPick) {
      pickForCard = card;
    } else {
      onshow(card);
    }
  }

  function pickCharacter(c: Character) {
    const card = pickForCard;
    if (!card) return;
    pickForCard = null;
    onshow(card, {
      id: c.id,
      name: c.name,
      edition: c.edition,
      iconSuffix: iconSuffix(c.team),
    });
  }

  function tapCustom(card: InfoCard) {
    onshow(customCardToDisplay(card));
  }

  function startNew() {
    formId = null;
    formTitle = "";
    formBody = "";
    formCharacters = [];
    formError = "";
    formOpen = true;
  }

  function startEdit(card: InfoCard) {
    formId = card.id;
    formTitle = card.title;
    formBody = card.body;
    formCharacters = [...card.characters];
    formError = "";
    formOpen = true;
  }

  function removeFormCharacter(id: string) {
    formCharacters = formCharacters.filter((c) => c.id !== id);
  }

  function addFormCharacter(char: Character) {
    if (formCharacters.length >= MAX_CARD_CHARACTERS) return;
    if (formCharacters.some((c) => c.id === char.id)) return;
    formCharacters = [...formCharacters, char];
  }

  async function openCharPicker() {
    await ensureCharacters();
    charPickerOpen = true;
  }

  async function saveForm() {
    const title = formTitle.trim();
    if (!title) {
      formError = "Title is required";
      return;
    }
    formSaving = true;
    formError = "";
    const characterIds = formCharacters.map((c) => c.id);
    try {
      if (formId === null) {
        await client.createInfoCard({ title, body: formBody, characterIds });
      } else {
        await client.updateInfoCard({
          id: formId,
          title,
          body: formBody,
          characterIds,
        });
      }
      formOpen = false;
      await loadCards();
    } catch (err) {
      formError = getErrorMessage(err, "Failed to save card");
    } finally {
      formSaving = false;
    }
  }

  async function doDelete() {
    const card = deleteTarget;
    if (!card) return;
    deleting = true;
    try {
      await client.deleteInfoCard({ id: card.id });
      deleteTarget = null;
      await loadCards();
    } catch (err) {
      cardsError = getErrorMessage(err, "Failed to delete card");
      deleteTarget = null;
    } finally {
      deleting = false;
    }
  }

  function hideBrokenIcon(e: Event) {
    (e.target as HTMLImageElement).style.display = "none";
  }

  loadCards();
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
  onclick={onclose}
  onkeydown={(e) => e.key === "Escape" && onclose()}
>
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="mx-4 flex max-h-[85vh] w-full max-w-lg flex-col rounded-xl border border-border bg-surface shadow-2xl"
    onclick={(e) => e.stopPropagation()}
  >
    <!-- Header -->
    <div
      class="flex items-center justify-between border-b border-border px-5 py-3"
    >
      <h2 class="text-lg font-semibold text-primary">
        {#if pickForCard}
          Choose a character
        {:else if formOpen}
          {formId === null ? "New card" : "Edit card"}
        {:else}
          Info Cards
        {/if}
      </h2>
      <button
        onclick={onclose}
        class="rounded-lg p-1 text-muted transition-colors hover:bg-hover hover:text-primary"
        aria-label="Close"
      >
        <svg
          class="h-5 w-5"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          stroke-width="2"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            d="M6 18L18 6M6 6l12 12"
          />
        </svg>
      </button>
    </div>

    <div class="flex-1 overflow-y-auto px-5 py-4">
      {#if pickForCard}
        <!-- Character-select step for parameterized standard cards -->
        <button
          onclick={() => (pickForCard = null)}
          class="mb-3 text-sm text-secondary transition-colors hover:text-primary"
        >
          ← Back
        </button>
        <p class="mb-3 text-sm text-secondary">
          Pick the character for "<span class="font-medium text-primary"
            >{pickForCard.title}</span
          >".
        </p>
        {#if inPlayCharacters.length === 0}
          <p class="py-6 text-center text-sm text-muted">
            No characters are in play yet.
          </p>
        {:else}
          <div class="grid grid-cols-2 gap-2 sm:grid-cols-3">
            {#each inPlayCharacters as char (char.id)}
              <button
                onclick={() => pickCharacter(char)}
                class="flex flex-col items-center gap-1.5 rounded-lg border border-border bg-hover p-2.5 text-center transition-colors hover:brightness-110"
              >
                <img
                  src="/characters/{char.edition}/{char.id}{iconSuffix(
                    char.team,
                  )}.webp"
                  alt=""
                  class="h-12 w-12 rounded-full"
                  onerror={hideBrokenIcon}
                />
                <span class="text-xs font-medium text-primary">{char.name}</span
                >
              </button>
            {/each}
          </div>
        {/if}
      {:else if formOpen}
        <!-- Inline create / edit form -->
        {#if formError}
          <div
            class="mb-3 rounded-lg border border-error-border bg-error-bg px-3 py-2 text-sm text-error-text"
          >
            {formError}
          </div>
        {/if}
        <label
          class="mb-1 block text-xs font-medium text-secondary"
          for="ic-title"
        >
          Title
        </label>
        <input
          id="ic-title"
          type="text"
          bind:value={formTitle}
          maxlength={TITLE_MAX}
          placeholder="Card title..."
          class="w-full rounded-lg border border-border bg-transparent px-3 py-2 text-sm text-primary outline-none focus:border-indigo-500"
        />

        <label
          class="mt-3 mb-1 block text-xs font-medium text-secondary"
          for="ic-body"
        >
          Body
        </label>
        <textarea
          id="ic-body"
          bind:value={formBody}
          maxlength={BODY_MAX}
          rows="4"
          placeholder="Optional text shown on the card..."
          class="w-full resize-y rounded-lg border border-border bg-transparent px-3 py-2 text-sm text-primary outline-none focus:border-indigo-500"
        ></textarea>

        <div class="mt-3">
          <span class="mb-1 block text-xs font-medium text-secondary">
            Characters ({formCharacters.length}/{MAX_CARD_CHARACTERS})
          </span>
          <div class="flex flex-wrap items-center gap-2">
            {#each formCharacters as char (char.id)}
              <span
                class="flex items-center gap-1.5 rounded-full border border-border bg-hover py-1 pr-1 pl-2 text-xs text-primary"
              >
                <img
                  src="/characters/{char.edition}/{char.id}{iconSuffix(
                    char.team,
                  )}.webp"
                  alt=""
                  class="h-5 w-5 rounded-full"
                  onerror={hideBrokenIcon}
                />
                {char.name}
                <button
                  onclick={() => removeFormCharacter(char.id)}
                  class="rounded-full p-0.5 text-muted hover:text-red-500"
                  aria-label="Remove {char.name}"
                >
                  <svg
                    class="h-3 w-3"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    stroke-width="2.5"
                    ><path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      d="M6 18L18 6M6 6l12 12"
                    /></svg
                  >
                </button>
              </span>
            {/each}
            {#if formCharacters.length < MAX_CARD_CHARACTERS}
              <button
                onclick={openCharPicker}
                disabled={loadingChars}
                class="rounded-full border border-dashed border-border px-3 py-1 text-xs text-secondary transition-colors hover:border-indigo-500 hover:text-primary disabled:opacity-50"
              >
                {loadingChars ? "Loading…" : "+ Add character"}
              </button>
            {/if}
          </div>
        </div>

        <div class="mt-5 flex justify-end gap-2">
          <button
            onclick={() => (formOpen = false)}
            class="rounded-lg border border-border px-4 py-2 text-sm font-medium text-secondary transition-colors hover:bg-hover hover:text-primary"
          >
            Cancel
          </button>
          <button
            onclick={saveForm}
            disabled={formSaving || !formTitle.trim()}
            class="rounded-lg bg-indigo-500 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-indigo-400 disabled:opacity-50"
          >
            {formSaving ? "Saving…" : "Save"}
          </button>
        </div>
      {:else}
        <!-- Standard cards -->
        <h3
          class="mb-2 text-xs font-semibold tracking-wide text-muted uppercase"
        >
          Standard
        </h3>
        <div class="grid grid-cols-2 gap-2 sm:grid-cols-3">
          {#each standardCards as card (card.id)}
            <button
              onclick={() => tapStandard(card)}
              class="relative flex aspect-[3/4] flex-col items-center justify-center gap-1.5 overflow-hidden rounded-lg p-2 text-center transition-transform hover:scale-[1.03]"
              style="background: {ACCENT_STYLES[card.accent]
                .background}; color: {ACCENT_STYLES[card.accent].text};"
            >
              <span
                class="font-serif text-[0.65rem] leading-tight font-bold tracking-wide uppercase"
                style="text-shadow: 0 1px 3px rgba(0,0,0,0.5);"
              >
                {card.title}
              </span>
              {#if card.characters.length > 0}
                <div class="flex -space-x-1.5">
                  {#each card.characters as ch (ch.id)}
                    <img
                      src="/characters/{ch.edition}/{ch.id}{ch.iconSuffix}.webp"
                      alt=""
                      class="h-6 w-6 rounded-full ring-1 ring-black/20"
                      onerror={hideBrokenIcon}
                    />
                  {/each}
                </div>
              {/if}
              {#if card.needsCharacterPick}
                <span class="text-[0.55rem] tracking-wide uppercase opacity-75">
                  pick character
                </span>
              {/if}
            </button>
          {/each}
        </div>

        <!-- My cards -->
        <div class="mt-5 flex items-center justify-between">
          <h3 class="text-xs font-semibold tracking-wide text-muted uppercase">
            My cards
          </h3>
          <button
            onclick={startNew}
            class="rounded-lg border border-indigo-500 px-3 py-1 text-xs font-medium text-indigo-500 transition-colors hover:bg-indigo-500 hover:text-white"
          >
            + New card
          </button>
        </div>

        {#if cardsError}
          <div
            class="mt-2 rounded-lg border border-error-border bg-error-bg px-3 py-2 text-sm text-error-text"
          >
            {cardsError}
          </div>
        {/if}

        {#if loadingCards}
          <p class="mt-3 text-sm text-secondary">Loading your cards…</p>
        {:else if customCards.length === 0}
          <p class="mt-3 py-4 text-center text-sm text-muted">
            No custom cards yet.
          </p>
        {:else}
          <div class="mt-2 space-y-1">
            {#each customCards as card (card.id)}
              <div
                class="group flex items-center gap-2 rounded-lg px-2 py-1.5 hover:bg-hover"
              >
                <button
                  onclick={() => tapCustom(card)}
                  class="flex min-w-0 flex-1 items-center gap-2 text-left"
                >
                  {#if card.characters.length > 0}
                    <div class="flex shrink-0 -space-x-1.5">
                      {#each card.characters.slice(0, 3) as ch (ch.id)}
                        <img
                          src="/characters/{ch.edition}/{ch.id}{iconSuffix(
                            ch.team,
                          )}.webp"
                          alt=""
                          class="h-6 w-6 rounded-full ring-1 ring-border"
                          onerror={hideBrokenIcon}
                        />
                      {/each}
                    </div>
                  {/if}
                  <span class="truncate text-sm text-primary">{card.title}</span
                  >
                </button>
                <button
                  onclick={() => startEdit(card)}
                  class="rounded p-1 text-muted opacity-100 transition-opacity hover:text-primary sm:opacity-0 sm:group-hover:opacity-100"
                  aria-label="Edit {card.title}"
                >
                  <svg
                    class="h-4 w-4"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    stroke-width="2"
                    ><path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"
                    /></svg
                  >
                </button>
                <button
                  onclick={() => (deleteTarget = card)}
                  class="rounded p-1 text-muted opacity-100 transition-opacity hover:text-red-500 sm:opacity-0 sm:group-hover:opacity-100"
                  aria-label="Delete {card.title}"
                >
                  <svg
                    class="h-4 w-4"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    stroke-width="2"
                    ><path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                    /></svg
                  >
                </button>
              </div>
            {/each}
          </div>
        {/if}
      {/if}
    </div>
  </div>
</div>

{#if charPickerOpen}
  <CharacterPickerModal
    title="Add characters"
    characters={allCharacters}
    selectedIds={new Set(formCharacters.map((c) => c.id))}
    onselect={addFormCharacter}
    ondeselect={removeFormCharacter}
    onclose={() => (charPickerOpen = false)}
  />
{/if}

{#if deleteTarget}
  <ConfirmDialog
    title="Delete card"
    message={`Delete "${deleteTarget.title}"? This cannot be undone.`}
    confirmLabel={deleting ? "Deleting…" : "Delete"}
    onconfirm={doDelete}
    oncancel={() => (deleteTarget = null)}
  />
{/if}
