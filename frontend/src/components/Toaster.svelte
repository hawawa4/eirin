<script lang="ts">
  import { toasts, toast, type Toast } from "../lib/toast.svelte";

  const ICONS: Record<Toast["kind"], string> = { error: "⚠", success: "✓", info: "ℹ" };

  async function runAction(t: Toast) {
    if (!t.action) return;
    toast.dismiss(t.id);
    try {
      await t.action.run();
    } catch (e) {
      toast.error(String(e));
    }
  }
</script>

<div class="toaster" role="region" aria-label="Notifications" aria-live="polite">
  {#each toasts as t (t.id)}
    <div
      class="toast toast-{t.kind}"
      role={t.kind === "error" ? "alert" : "status"}
      onmouseenter={() => toast.pause(t.id)}
      onmouseleave={() => toast.resume(t.id)}
      onfocusin={() => toast.pause(t.id)}
      onfocusout={() => toast.resume(t.id)}
    >
      <span class="toast-icon" aria-hidden="true">{ICONS[t.kind]}</span>
      <span class="toast-msg">{t.message}</span>
      {#if t.action}
        <button class="toast-action" onclick={() => runAction(t)}>{t.action.label}</button>
      {/if}
      <button
        class="toast-close"
        aria-label="Dismiss notification"
        title="Dismiss"
        onclick={() => toast.dismiss(t.id)}>✕</button
      >
    </div>
  {/each}
</div>

<style>
  .toaster {
    position: fixed;
    right: 16px;
    bottom: 16px;
    z-index: 3000;
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    gap: 8px;
    pointer-events: none;
    max-width: min(440px, calc(100vw - 32px));
  }

  .toast {
    pointer-events: auto;
    display: flex;
    align-items: flex-start;
    gap: 10px;
    background: var(--bg-panel);
    border: 1px solid var(--border-accent);
    border-left-width: 3px;
    border-radius: 6px;
    padding: 8px 10px 8px 12px;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.55);
    color: var(--text-primary);
    font-size: var(--fs-sm);
    user-select: text;
  }
  .toast-error {
    border-left-color: var(--danger);
  }
  .toast-success {
    border-left-color: var(--success);
  }
  .toast-info {
    border-left-color: var(--accent);
  }

  .toast-icon {
    flex-shrink: 0;
    font-size: var(--fs-md);
  }
  .toast-error .toast-icon {
    color: var(--danger);
  }
  .toast-success .toast-icon {
    color: var(--success);
  }
  .toast-info .toast-icon {
    color: var(--accent);
  }

  .toast-msg {
    flex: 1;
    min-width: 0;
    overflow-wrap: anywhere;
    padding-top: 1px;
  }

  .toast-action {
    flex-shrink: 0;
    background: transparent;
    border: 1px solid var(--accent);
    border-radius: 4px;
    color: var(--accent);
    font-size: var(--fs-xs);
    font-weight: 600;
    padding: 2px 8px;
    cursor: pointer;
  }
  .toast-action:hover {
    background: var(--accent);
    color: var(--accent-contrast);
  }

  .toast-close {
    flex-shrink: 0;
    background: transparent;
    border: none;
    color: var(--text-secondary);
    font-size: var(--fs-xs);
    line-height: 1;
    padding: 4px;
    cursor: pointer;
    border-radius: 4px;
  }
  .toast-close:hover {
    color: var(--text-primary);
    background: var(--bg-row-hover);
  }
</style>
