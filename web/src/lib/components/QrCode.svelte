<script lang="ts">
  // A QR code as inline SVG — no canvas, no network, no runtime dependency on
  // anything the player's phone has to load.
  //
  // Deliberately dumb: it draws one code at one size. Enlarging it, opening it in
  // a modal or picking what to encode is the consumer's business.
  import { renderSVG } from "uqr";

  let {
    value,
    label,
    size = 200,
  }: {
    value: string;
    label: string;
    /** Rendered edge length in CSS pixels, including the white quiet zone. */
    size?: number;
  } = $props();

  // Medium error correction survives a phone camera at an angle; border 2 is the
  // quiet zone scanners need to find the code at all.
  const svg = $derived(renderSVG(value, { ecc: "M", border: 2 }));
</script>

<div class="flex flex-col items-center gap-2">
  <!--
    The background stays white in every theme: a dark-on-dark QR code is not
    scannable, and the modules are drawn in black by uqr.
  -->
  <div
    role="img"
    aria-label={label}
    class="rounded-xl bg-white p-2 shadow-sm [&>svg]:block [&>svg]:h-full [&>svg]:w-full"
    style="width: {size}px; height: {size}px"
  >
    {@html svg}
  </div>
  <span class="text-xs font-medium text-secondary">{label}</span>
</div>
