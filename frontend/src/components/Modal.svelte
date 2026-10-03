<script lang="ts" module>
  let uid = 0;
</script>

<script lang="ts">
  import { onMount, type Snippet } from "svelte";
  import { pushModal, popModal, isTopModal } from "../lib/modalStack";

  interface Props {
    title: string;
    onclose: () => void;
    /** While true, Escape and backdrop clicks do nothing. */
    busy?: boolean;
    dismissOnBackdrop?: boolean;
    /** CSS width of the dialog box (default `min(440px, 92vw)`). */
    width?: string;
    children: Snippet;
    /** Footer buttons, right-aligned. */
    actions?: Snippet;
    /** Red accent for destructive confirmations. */
    danger?: boolean;
  }

  let {
    title,
    onclose,
    busy = false,
    dismissOnBackdrop = true,
    width = "min(440px, 92vw)",
    children,
    actions,
    danger = false,
  }: Props = $props();

  const FOCUSABLE =
    'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

  const titleId = `modal-title-${++uid}`;

  let dialogEl: HTMLDivElement;
  let modalId: symbol | null = null;
  // Captured before mount so focus can return to the trigger on close.
  const previouslyFocused =
    document.activeElement instanceof HTMLElement ? document.activeElement : null;

  function focusables(): HTMLElement[] {
    return Array.from(dialogEl.querySelectorAll<HTMLElement>(FOCUSABLE)).filter(
      (el) => el.offsetParent !== null || el === document.activeElement,
    );
  }

  onMount(() => {
    modalId = pushModal();
    const initial =
      dialogEl.querySelector<HTMLElement>("[data-autofocus], [autofocus]") ?? focusables()[0];
    (initial ?? dialogEl).focus();

    return () => {
      if (modalId) popModal(modalId);
      if (previouslyFocused?.isConnected) previouslyFocused.focus();
    };
  });

  function onWindowKeydown(e: KeyboardEvent) {
    if (!modalId || !isTopModal(modalId)) return;

    if (e.key === "Escape") {
      e.preventDefault();
      e.stopPropagation();
      if (!busy) onclose();
      return;
    }

    if (e.key === "Tab") {
      const items = focusables();
      if (items.length === 0) {
        e.preventDefault();
        dialogEl.focus();
        return;
      }
      const first = items[0];
      const last = items[items.length - 1];
      const active = document.activeElement;
      const inside = active instanceof Node && dialogEl.contains(active);
      if (e.shiftKey && (active === first || !inside || active === dialogEl)) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && (active === last || !inside)) {
        e.preventDefault();
        first.focus();
      }
    }
  }

  function onBackdropMousedown(e: MouseEvent) {
    if (e.target !== e.currentTarget) return;
    if (dismissOnBackdrop && !busy) onclose();
  }
</script>

<!-- Capture phase so the top-most modal sees Escape/Tab before any view-level shortcut. -->
<svelte:window onkeydowncapture={onWindowKeydown} />

<div class="modal-root-backdrop" onmousedown={onBackdropMousedown} role="presentation">
  <div
    bind:this={dialogEl}
    class="modal-dialog"
    class:modal-danger={danger}
    style:width
    role="dialog"
    aria-modal="true"
    aria-labelledby={titleId}
    aria-busy={busy}
    tabindex="-1"
    onkeydown={(e) => e.stopPropagation()}
  >
    <h2 class="modal-dialog-title" id={titleId}>{title}</h2>
    <div class="modal-dialog-body">
      {@render children()}
    </div>
    {#if actions}
      <div class="modal-dialog-actions">
        {@render actions()}
      </div>
    {/if}
  </div>
</div>

<style>
  .modal-root-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.6);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 2000;
  }

  .modal-dialog {
    background: var(--bg-panel);
    border: 1px solid var(--border-accent);
    border-radius: 8px;
    padding: 24px;
    max-width: 92vw;
    max-height: 88vh;
    overflow: auto;
    display: flex;
    flex-direction: column;
    gap: 12px;
    box-shadow: 0 12px 40px rgba(0, 0, 0, 0.7);
    color: var(--text-primary);
    font-size: var(--fs-md);
  }
  .modal-dialog:focus {
    outline: none;
  }
  .modal-danger {
    border-top: 3px solid var(--danger);
  }

  .modal-dialog-title {
    margin: 0;
    font-size: var(--fs-lg);
    font-weight: 600;
    color: var(--text-primary);
  }
  .modal-danger .modal-dialog-title {
    color: var(--danger);
  }

  .modal-dialog-body {
    display: flex;
    flex-direction: column;
    gap: 10px;
    color: var(--text-secondary);
    font-size: var(--fs-sm);
    min-height: 0;
  }

  .modal-dialog-actions {
    display: flex;
    gap: 8px;
    justify-content: flex-end;
    margin-top: 4px;
  }
</style>
