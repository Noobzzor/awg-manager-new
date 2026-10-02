import type { OutboundDisplay } from './types';

/** Полная подпись: «Main (meta)». */
export function outboundFullLabel(
  label: string,
  metaSuffix?: string,
): string {
  return metaSuffix ? `${label} (${metaSuffix})` : label;
}

export function outboundDisplayTitle(outbound: Pick<OutboundDisplay, 'label' | 'metaSuffix'>): string {
  return outboundFullLabel(outbound.label, outbound.metaSuffix);
}

/** «DE Frankfurt (t2s0)» → main + meta. */
export function splitParenMeta(text: string): { label: string; metaSuffix?: string } {
  const match = text.match(/^(.+?) \(([A-Za-z0-9._-]+)\)$/);
  if (!match) return { label: text };
  return { label: match[1], metaSuffix: match[2] };
}
