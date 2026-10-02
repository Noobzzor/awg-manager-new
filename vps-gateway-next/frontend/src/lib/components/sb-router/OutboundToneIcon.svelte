<script lang="ts">
  import {
    Ban,
    GitBranch,
    GitCommitHorizontal,
    Globe,
    Network,
    Rss,
    OctagonAlert,
    ScanEye,
    Timer,
    TriangleAlert,
    Waypoints,
  } from 'lucide-svelte';
  import type { OutboundTileTone } from './outboundTileTone';
  import type { OutboundKind } from './types';

  interface Props {
    tone: OutboundTileTone;
    kind?: OutboundKind;
    size?: number;
  }

  let { tone, kind, size = 14 }: Props = $props();

  const iconProps = $derived({ size, 'aria-hidden': true as const, strokeWidth: 2 });
</script>

{#if tone === 'proxy'}
  <GitBranch {...iconProps} />
{:else if tone === 'awg'}
  <svg
    viewBox="0 0 24 24"
    width={size}
    height={size}
    fill="none"
    stroke="currentColor"
    stroke-width="2"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <path d="M2 22V12a10 10 0 1 1 20 0v10" />
    <path d="M8 22V12a4 4 0 1 1 8 0v10" />
    <line x1="2" y1="22" x2="22" y2="22" />
  </svg>
{:else if tone === 'subscription'}
  <Rss {...iconProps} />
{:else if tone === 'composite'}
  <Waypoints {...iconProps} />
{:else if tone === 'block'}
  <Ban {...iconProps} />
{:else if tone === 'direct'}
  <Globe {...iconProps} />
{:else if tone === 'via-route'}
  <GitCommitHorizontal {...iconProps} />
{:else if tone === 'invalid'}
  <OctagonAlert {...iconProps} />
{:else if tone === 'system'}
  {#if kind === 'hijack-dns'}
    <Network {...iconProps} />
  {:else if kind === 'udp-timeout'}
    <Timer {...iconProps} />
  {:else}
    <ScanEye {...iconProps} />
  {/if}
{:else}
  <TriangleAlert {...iconProps} />
{/if}
