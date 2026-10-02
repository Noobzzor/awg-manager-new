<script lang="ts">
  import { Pencil, Globe, Database } from 'lucide-svelte';
  import type { RuleSetDisplayType } from '$lib/utils/ruleSetType';

  interface Props {
    type: RuleSetDisplayType;
    size?: number;
    strokeWidth?: number;
  }

  let { type, size = 10, strokeWidth = 2.25 }: Props = $props();
</script>

<span class="icon" aria-hidden="true">
  {#if type === 'inline'}
    <Pencil {size} {strokeWidth} />
  {:else if type === 'dat'}
    <Database {size} {strokeWidth} />
  {:else}
    <Globe {size} {strokeWidth} />
  {/if}
</span>

<style>
  .icon {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
    opacity: 0.92;
  }
</style>
