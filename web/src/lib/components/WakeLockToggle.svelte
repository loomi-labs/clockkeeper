<script lang="ts">
  import { onMount, onDestroy } from "svelte";

  let wakeLock: WakeLockSentinel | null = $state(null);
  let supported = $state(false);
  let userEnabled = $state(false);

  onMount(() => {
    supported = "wakeLock" in navigator;
  });

  async function acquire(): Promise<boolean> {
    try {
      wakeLock = await navigator.wakeLock.request("screen");
      wakeLock.addEventListener("release", () => {
        wakeLock = null;
      });
      return true;
    } catch {
      wakeLock = null;
      return false;
    }
  }

  async function toggleWakeLock() {
    if (userEnabled) {
      userEnabled = false;
      await wakeLock?.release();
      wakeLock = null;
    } else {
      userEnabled = await acquire();
    }
  }

  function onVisibilityChange() {
    if (document.visibilityState === "visible" && userEnabled && !wakeLock) {
      acquire();
    }
  }

  onDestroy(() => {
    wakeLock?.release();
  });
</script>

<svelte:document onvisibilitychange={onVisibilityChange} />

{#if supported}
  <button
    onclick={toggleWakeLock}
    class="rounded-lg border border-border p-2 transition-colors {userEnabled
      ? 'bg-amber-100 border-amber-300 text-amber-600 dark:bg-amber-500/20 dark:border-amber-600 dark:text-amber-400'
      : 'text-secondary hover:bg-hover hover:text-medium'}"
    title={userEnabled ? "Screen will stay on (click to disable)" : "Keep screen on"}
    aria-label={userEnabled ? "Disable screen wake lock" : "Enable screen wake lock"}
    aria-pressed={userEnabled}
  >
    <svg
      class="h-4 w-4"
      fill={userEnabled ? "currentColor" : "none"}
      viewBox="0 0 24 24"
      stroke="currentColor"
      stroke-width="2"
    >
      <path
        stroke-linecap="round"
        stroke-linejoin="round"
        d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z"
      />
    </svg>
  </button>
{/if}
