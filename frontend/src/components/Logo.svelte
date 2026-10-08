<script lang="ts">
  let { size = 26 }: { size?: number } = $props();
</script>

<svg width={size} height={size} viewBox="0 0 64 64" aria-hidden="true">
  <rect x="7" y="7" width="54" height="54" rx="12" fill="var(--ib-ex)" />
  <rect x="3" y="3" width="54" height="54" rx="12" fill="var(--ib-accent)" stroke="var(--ib-line)" stroke-width="4" />
  <path d="M30 13v19m0 0-8-8m8 8 8-8" fill="none" stroke="#140d1f" stroke-width="6" stroke-linecap="round" stroke-linejoin="round" />
  <path d="M13 43h5M27 43h5M41 43h5" stroke="#140d1f" stroke-width="6" stroke-linecap="round" />
</svg>
