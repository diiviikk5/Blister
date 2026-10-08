<script lang="ts">
  // The file as 16 isometric blocks, each filling with the bytes that have
  // landed in its slice. A magenta cap marks slices a connection is writing.
  // Click to pull the lanes apart.
  let {
    map = "",
    heads = [],
    progress = 0,
    status = "downloading",
  }: { map?: string; heads?: number[]; progress?: number; status?: string } = $props();

  const LANES = 16;
  let open = $state(true);

  const lanes = $derived.by(() => {
    const out: { fill: number; head: boolean }[] = [];
    const hs = heads.map((h) => Math.min(LANES - 1, Math.floor(h * LANES)));
    for (let i = 0; i < LANES; i++) {
      let fill: number;
      if (status === "completed") fill = 1;
      else if (map) {
        const a = Math.floor((i / LANES) * map.length);
        const b = Math.max(a + 1, Math.floor(((i + 1) / LANES) * map.length));
        let s = 0;
        for (let k = a; k < b; k++) s += map.charCodeAt(k) - 48;
        fill = s / ((b - a) * 9);
      } else {
        fill = Math.max(0, Math.min(1, progress * LANES - i));
      }
      out.push({ fill, head: hs.includes(i) && status === "downloading" });
    }
    return out;
  });
</script>

<button class="stage" onclick={() => (open = !open)} aria-label={open ? "Close the lanes" : "Pull the lanes apart"} title="Click to fold or unfold">
  <span class="iso" class:open class:done={status === "completed"} aria-hidden="true">
    <span class="shadow"></span>
    <span class="lanes">
      {#each lanes as l, i (i)}
        <i class="lane" class:head={l.head} style:--i={i} style:--f="{(l.fill * 100).toFixed(1)}%"><b></b></i>
      {/each}
    </span>
  </span>
</button>

<style>
  .stage {
    width: 100%;
    height: 210px;
    display: grid;
    place-items: center;
    padding: 0;
    border: 0;
    background: none;
    cursor: pointer;
    perspective: 1800px;
    overflow: hidden;
  }
  .iso {
    --gap: 0px;
    --lift: 0px;
    position: relative;
    display: block;
    width: 250px;
    height: 150px;
    transform-style: preserve-3d;
    transform: rotateX(56deg) rotateZ(-40deg) translateY(-12px);
  }
  .iso.open {
    --gap: 5px;
    --lift: 14px;
  }
  .shadow {
    position: absolute;
    inset: 6px -6px -6px 6px;
    border-radius: var(--r);
    background: repeating-linear-gradient(45deg, var(--ib-ex) 0 2px, transparent 2px 8px);
    opacity: 0.3;
    transform: translateZ(-26px);
  }
  .lanes {
    position: absolute;
    inset: 0;
    display: flex;
    gap: var(--gap);
    transform-style: preserve-3d;
    transition: gap calc(0.5s * var(--motion, 1)) cubic-bezier(0.2, 0.9, 0.25, 1.15);
  }
  .lane {
    position: relative;
    flex: 1;
    display: block;
    border: calc(var(--bw) * 0.8) solid var(--ib-line);
    background: var(--ib-paper-2);
    transform-style: preserve-3d;
    transform: translateZ(var(--lift));
    transition: transform calc(0.5s * var(--motion, 1)) cubic-bezier(0.2, 0.9, 0.25, 1.15) calc(var(--i) * 14ms);
  }
  .lane:first-child {
    border-radius: var(--r-xs) 0 0 var(--r-xs);
  }
  .lane:last-child {
    border-radius: 0 var(--r-xs) var(--r-xs) 0;
  }
  .lane::after {
    content: "";
    position: absolute;
    left: -2px;
    right: -2px;
    top: 100%;
    height: 16px;
    background: var(--ib-ex);
    transform-origin: top;
    transform: rotateX(-90deg);
  }
  .lane::before {
    content: "";
    position: absolute;
    top: -2px;
    bottom: -2px;
    left: 100%;
    width: 16px;
    background: color-mix(in srgb, var(--ib-ex) 70%, var(--ib-paper));
    transform-origin: left;
    transform: rotateY(90deg);
  }
  .lane b {
    position: absolute;
    left: 0;
    right: 0;
    bottom: 0;
    height: var(--f);
    background: var(--ib-accent);
    transition: height 0.45s linear;
  }
  .lane.head b {
    border-top: 4px solid var(--ib-hot);
  }
  .done .lane {
    background: var(--ok);
  }
  .done .lane b {
    background: var(--ok);
  }
</style>
