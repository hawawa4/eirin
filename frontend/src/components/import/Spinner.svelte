<script lang="ts">
  interface Props {
    /** Diameter in px. */
    size?: number;
    label?: string;
  }
  let { size = 16, label = "Loading" }: Props = $props();
</script>

<span
  class="spinner"
  role="status"
  aria-label={label}
  style:width="{size}px"
  style:height="{size}px"
  style:border-width="{Math.max(2, Math.round(size / 10))}px"
></span>

<style>
  .spinner {
    display: inline-block;
    flex-shrink: 0;
    border-style: solid;
    border-color: var(--border-accent);
    border-top-color: var(--accent);
    border-radius: 50%;
    animation: spin 0.9s linear infinite;
    vertical-align: middle;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .spinner {
      animation-duration: 2.4s;
    }
  }
</style>
