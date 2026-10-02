<!--
  Компактный бейдж типа rule_set с иконкой: inline → карандаш, remote/local → глобус, dat → база.
-->

<script lang="ts">
  import { Badge } from '$lib/components/ui';
  import RuleSetTypeIcon from './RuleSetTypeIcon.svelte';
  import {
    ruleSetDisplayLabel,
    ruleSetDisplayVariant,
    type RuleSetDisplayType,
  } from '$lib/utils/ruleSetType';

  interface Props {
    type: RuleSetDisplayType;
    size?: 'xs' | 'sm';
  }

  let { type, size = 'sm' }: Props = $props();

  const iconSize = $derived(size === 'xs' ? 9 : 10);
</script>

<Badge variant={ruleSetDisplayVariant(type)} {size} mono>
  <RuleSetTypeIcon {type} size={iconSize} />
  {ruleSetDisplayLabel(type)}
</Badge>
