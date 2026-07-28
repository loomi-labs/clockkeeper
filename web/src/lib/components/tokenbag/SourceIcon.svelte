<script lang="ts">
  // Where a registrant joined from: their own phone, or the shared tablet that
  // was passed around the table. Shown on the waiting chips and on the seat rows,
  // which is why it lives in its own component rather than in either of them.
  let { viaShared }: { viaShared: boolean } = $props();
</script>

{#if viaShared}
  <svg
    class="h-3 w-3 shrink-0 text-muted"
    fill="none"
    viewBox="0 0 24 24"
    stroke="currentColor"
    stroke-width="2"
    role="img"
    aria-label="Added on the shared device"
  >
    <path
      stroke-linecap="round"
      stroke-linejoin="round"
      d="M9.75 17h4.5m-6.75 4h9a1.5 1.5 0 001.5-1.5v-15A1.5 1.5 0 0016.5 3h-9A1.5 1.5 0 006 4.5v15A1.5 1.5 0 007.5 21z"
    />
  </svg>
{:else}
  <svg
    class="h-3 w-3 shrink-0 text-muted"
    fill="none"
    viewBox="0 0 24 24"
    stroke="currentColor"
    stroke-width="2"
    role="img"
    aria-label="Joined from their own phone"
  >
    <path
      stroke-linecap="round"
      stroke-linejoin="round"
      d="M10.5 1.5h-2.25A2.25 2.25 0 006 3.75v16.5a2.25 2.25 0 002.25 2.25h7.5a2.25 2.25 0 002.25-2.25V3.75a2.25 2.25 0 00-2.25-2.25H13.5m-3 0V3h3V1.5m-3 0h3m-3 18.75h3"
    />
  </svg>
{/if}
