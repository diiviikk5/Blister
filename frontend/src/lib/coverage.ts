// One shared interpretation of the engine's 0..9 coverage telemetry.
export function coverageCells(map: string, progress: number, complete: boolean, count = 160): number[] {
  const size = Math.max(1, Math.min(320, Math.floor(count) || 160));
  if (complete) return new Array(size).fill(9);
  const usable = /^[0-9]+$/.test(map);
  const fraction = Number.isFinite(progress) ? Math.max(0, Math.min(1, progress)) : 0;
  return Array.from({ length: size }, (_, i) => {
    if (!usable) return Math.round(Math.max(0, Math.min(1, fraction * size - i)) * 9);
    const start = Math.floor(i / size * map.length);
    const end = Math.max(start + 1, Math.floor((i + 1) / size * map.length));
    let fill = 0;
    for (let k = start; k < end; k++) fill += Number(map[k]);
    return Math.round(fill / (end - start));
  });
}
