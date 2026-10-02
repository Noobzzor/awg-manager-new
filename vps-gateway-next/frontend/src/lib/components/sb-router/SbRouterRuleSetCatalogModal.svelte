<script lang="ts">
  import { ServiceCatalogModal } from '$lib/components/dnsroutes';
  import { presetCatalog } from '$lib/stores/presets';
  import { singboxRouterCatalogPresetFilter } from '$lib/utils/catalog-preset';
  import type { CatalogPreset } from '$lib/types';
  import { fullyAddedPresetNames } from './rulesetCatalogActions';

  interface Props {
    open: boolean;
    existingRuleSetTags: string[];
    /** rule_set tag → number of referencing rules (route + DNS). With it the disabled
     *  «добавлено» tile differentiates «используется правилами» vs «без правил». */
    ruleSetUsage?: Map<string, number>;
    submitting?: boolean;
    onclose: () => void;
    onconfirm: (presets: CatalogPreset[]) => void;
  }

  let {
    open = false,
    existingRuleSetTags,
    ruleSetUsage = undefined,
    submitting = false,
    onclose,
    onconfirm,
  }: Props = $props();

  const existingNames = $derived(
    fullyAddedPresetNames($presetCatalog, new Set(existingRuleSetTags)),
  );
</script>

<ServiceCatalogModal
  {open}
  title="Каталог наборов"
  presetFilter={singboxRouterCatalogPresetFilter}
  footer="none"
  multiple
  warnLargeDnsLists={false}
  markExisting
  {existingNames}
  {existingRuleSetTags}
  {ruleSetUsage}
  confirmLabel="Добавить наборы"
  {submitting}
  {onclose}
  {onconfirm}
/>
