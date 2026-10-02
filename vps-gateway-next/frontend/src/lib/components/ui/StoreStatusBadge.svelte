<script lang="ts" generics="T">
    import type { PollingStore } from '$lib/stores/polling';
    import Badge from './Badge.svelte';

    interface Props {
        // Generic over the store's payload T: the badge only reads status/error/
        // age/lastFetchedAt/consecutiveFailures, but T sits in a contravariant
        // position on applyMutationResponse, so `PollingStore<unknown>` would
        // reject typed stores. The component parameter keeps callers type-safe
        // without a widening cast.
        store: PollingStore<T>;
        /**
         * Must match the `errorThreshold` passed to createPollingStore for this store.
         * Default 3 matches the createPollingStore default. If the store was created
         * with a custom errorThreshold, pass the same value here or the badge will
         * never render (or will render early).
         */
        threshold?: number;
    }

    let { store, threshold = 3 }: Props = $props();

    let s = $derived($store);

    function humanAge(ms: number): string {
        if (ms === 0) return 'никогда';
        const secs = Math.floor((Date.now() - ms) / 1000);
        if (secs < 60) return `${secs}с назад`;
        return `${Math.floor(secs / 60)}мин назад`;
    }

    async function retry() {
        await store.refetch();
    }
</script>

{#if s.status === 'error' && s.consecutiveFailures >= threshold}
    <span role="status" aria-live="polite">
        <Badge variant="error">
            <span>обновлено {humanAge(s.lastFetchedAt)}</span>
            <button type="button" onclick={retry}>повторить</button>
        </Badge>
    </span>
{/if}

<style>
    button {
        background: transparent;
        border: none;
        color: inherit;
        cursor: pointer;
        padding: 0;
        text-decoration: underline;
        font: inherit;
    }
    button:hover {
        opacity: 0.8;
    }
</style>
