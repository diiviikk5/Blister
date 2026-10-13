<script lang="ts">
  let { size = 26 }: { size?: number } = $props();
</script>

<svg width={size} height={size} viewBox="0 0 64 64" aria-hidden="true">
  <rect x="4" y="4" width="56" height="56" rx="16" fill="color-mix(in srgb, var(--ib-accent) 12%, var(--ib-card))" stroke="var(--ib-line)" stroke-width="2" />
  <path d="M32 15v21m0 0-8-8m8 8 8-8M19 46h26" fill="none" stroke="var(--ib-accent)" stroke-width="4" stroke-linecap="round" stroke-linejoin="round" />
</svg>
