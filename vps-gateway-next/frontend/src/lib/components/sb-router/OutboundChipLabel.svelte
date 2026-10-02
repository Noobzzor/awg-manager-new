<script lang="ts">
  import { outboundFullLabel } from './outboundLabelFormat';

  interface Props {
    label: string;
    metaSuffix?: string;
    title?: string;
  }

  let { label, metaSuffix, title }: Props = $props();

  const fullTitle = $derived(title ?? outboundFullLabel(label, metaSuffix));
</script>

<span class="chip-label" title={fullTitle}>
  <span class="chip-main">{label}</span>
  {#if metaSuffix}
    <span class="chip-meta">({metaSuffix})</span>
  {/if}
</span>

<style>
  .chip-label {
    display: inline-flex;
    align-items: center;
    min-width: 0;
    max-width: 100%;
    font-size: inherit;
    line-height: inherit;
  }

  .chip-main {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: inherit;
    line-height: inherit;
  }

  .chip-meta {
    flex-shrink: 0;
    margin-left: 0.3em;
    font-size: inherit;
    line-height: inherit;
    letter-spacing: 0;
  }
</style>
