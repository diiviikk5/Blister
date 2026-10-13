<script lang="ts">
  import { coverageCells } from "../lib/coverage";
  import { store } from "../lib/store.svelte";
  let { map = "", progress = 0, status = "downloading", heads = [], compact = false, torrent = false }:
    { map?: string; progress?: number; status?: string; heads?: number[]; compact?: boolean; torrent?: boolean } = $props();
  const cells = $derived(coverageCells(map, progress, status === "completed", compact ? 80 : 160));
  const active = $derived(status === "downloading" || status === "starting");
  const writers = $derived(new Set(heads.filter((h) => Number.isFinite(h) && h >= 0 && h <= 1).map((h) => Math.min(cells.length - 1, Math.floor(h * cells.length)))));
  const animated = $derived(active && store.rice.bar.animation !== "off" && store.rice.fx.motion !== 0);
</script>

<figure class="transfer" class:compact class:active class:complete={status === "completed"} class:animated aria-label={torrent ? "Torrent piece coverage" : "Downloaded byte coverage"}>
  <div class="matrix" aria-hidden="true">
    {#each cells as fill, i (i)}
      {#key fill}
        <i class:received={fill > 0} class:writing={writers.has(i)} style:--coverage="{fill / 9}"></i>
      {/key}
    {/each}
  </div>
  {#if !compact}
    <figcaption><span class="legend"><i></i> {torrent ? "Pieces received" : "Bytes received"}</span><span>{status === "completed" ? "Complete" : active ? "Receiving" : status === "paused" ? "Paused" : status === "error" ? "Interrupted" : "Waiting"}<b class="signal" class:working={animated} aria-hidden="true"></b></span></figcaption>
  {/if}
</figure>

<style>
  .transfer { margin: 0; padding: 15px; border: 1px solid var(--ib-line); border-radius: var(--r); background: var(--ib-card); }
  .matrix { display: grid; grid-template-columns: repeat(20, minmax(0, 1fr)); gap: 4px; }
  .matrix i { display: block; aspect-ratio: 1; border-radius: 3px; background: color-mix(in srgb, var(--ib-faint) 24%, var(--ib-paper)); transition: background-color 200ms ease-out; }
  .matrix .received { background: color-mix(in srgb, var(--ib-accent) calc(var(--coverage) * 100%), var(--ib-paper)); }
  .matrix .writing { background: var(--ib-hot); }
  .complete .matrix i { background: var(--ok); }
  .animated .received { animation: receive 400ms cubic-bezier(0, 0, 0.2, 1) both; }
  .animated .writing { animation: write 1.4s ease-in-out infinite; }
  .compact { padding: 0; border: 0; background: none; }
  .compact .matrix { gap: 3px; }
  .compact .matrix i { aspect-ratio: auto; height: 7px; border-radius: 2px; }
  figcaption { display: flex; justify-content: space-between; align-items: center; gap: 8px; margin-top: 12px; color: var(--ib-muted); font: 10px var(--ib-mono); }
  figcaption > span, .legend { display: inline-flex; align-items: center; gap: 6px; }
  .legend i { width: 6px; height: 6px; border-radius: 2px; background: var(--ib-accent); }
  .signal { width: 5px; height: 5px; border-radius: 50%; background: var(--ib-faint); }
  .working { background: var(--ok); animation: write 1.4s ease-in-out infinite; }
  @keyframes receive { from { opacity: 0.35; transform: scale(0.7); } to { opacity: 1; transform: scale(1); } }
  @keyframes write { 50% { opacity: 0.4; } }
  @media (prefers-reduced-motion: reduce) { .animated .received, .animated .writing, .working { animation: none; } .matrix i { transition: none; } }
</style>
