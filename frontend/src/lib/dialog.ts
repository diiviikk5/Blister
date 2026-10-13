// Shared keyboard and focus behavior for stacked dialogs.
const dialogs: HTMLElement[] = [];
const selector = 'button:not(:disabled), a[href], input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex="0"]';

export function dialogFocus(node: HTMLElement, options: { close: () => void; initial?: string }) {
  const previous = document.activeElement instanceof HTMLElement ? document.activeElement : null;
  dialogs.push(node);
  node.tabIndex = -1;
  const focusable = () => Array.from(node.querySelectorAll<HTMLElement>(selector)).filter((el) => el.getClientRects().length > 0 && !el.closest('[inert]'));
  const frame = requestAnimationFrame(() => (node.querySelector<HTMLElement>(options.initial ?? "input, textarea") ?? focusable()[0] ?? node).focus());
  function keys(e: KeyboardEvent) {
    if (dialogs.at(-1) !== node) return;
    if (e.key === "Escape") {
      e.preventDefault();
      e.stopImmediatePropagation();
      options.close();
    } else if (e.key === "Tab") {
      const items = focusable();
      const first = items[0] ?? node;
      const last = items.at(-1) ?? node;
      if (!node.contains(document.activeElement) || document.activeElement === node || (e.shiftKey && document.activeElement === first) || (!e.shiftKey && document.activeElement === last)) {
        e.preventDefault();
        (e.shiftKey ? last : first).focus();
      }
    }
  }
  function keepFocus(e: FocusEvent) {
    if (dialogs.at(-1) === node && !node.contains(e.target as Node)) (focusable()[0] ?? node).focus();
  }
  document.addEventListener("keydown", keys, true);
  document.addEventListener("focusin", keepFocus);
  return { destroy() {
    cancelAnimationFrame(frame);
    dialogs.splice(dialogs.indexOf(node), 1);
    document.removeEventListener("keydown", keys, true);
    document.removeEventListener("focusin", keepFocus);
    if (previous?.isConnected) previous.focus();
    else dialogs.at(-1)?.focus();
  } };
}
