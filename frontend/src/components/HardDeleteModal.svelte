<script lang="ts">
  interface Props {
    target: { name: string };
    onconfirm: () => void;
    oncancel: () => void;
  }

  let { target, onconfirm, oncancel }: Props = $props();
</script>

<div
  class="modal-backdrop"
  onclick={oncancel}
  onkeydown={(e) => e.key === "Escape" && oncancel()}
  role="presentation"
>
  <div
    class="modal"
    role="dialog"
    aria-modal="true"
    tabindex="-1"
    onclick={(e) => e.stopPropagation()}
    onkeydown={(e) => e.stopPropagation()}
  >
    <div class="modal-icon">⚠</div>
    <p class="modal-title">Delete file permanently?</p>
    <p class="modal-filename">{target.name}</p>
    <p class="modal-sub">This cannot be undone. The file will be removed from disk.</p>
    <div class="modal-btns">
      <button class="tool-btn" onclick={oncancel}>Cancel</button>
      <button class="btn-danger" onclick={onconfirm}>Delete Forever</button>
    </div>
  </div>
</div>

<style>
  :global(.modal-backdrop) {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.6);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 2000;
  }

  :global(.modal) {
    background: var(--bg-panel);
    border: 1px solid var(--border-accent);
    border-radius: 8px;
    padding: 24px 28px;
    max-width: 380px;
    width: 90%;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
    box-shadow: 0 12px 40px rgba(0, 0, 0, 0.7);
  }

  :global(.modal-icon) {
    font-size: 2rem;
    color: var(--danger);
    margin-bottom: 4px;
  }
  :global(.modal-title) {
    font-size: 1rem;
    font-weight: 600;
    color: var(--text-primary);
    margin: 0;
  }
  :global(.modal-filename) {
    font-family: "Consolas", "Fira Code", monospace;
    font-size: 0.82rem;
    color: var(--accent);
    word-break: break-all;
    text-align: center;
    margin: 0;
  }
  :global(.modal-sub) {
    font-size: 0.8rem;
    color: var(--text-secondary);
    text-align: center;
    margin: 0;
  }
  :global(.modal-btns) {
    display: flex;
    gap: 10px;
    margin-top: 10px;
  }
</style>
