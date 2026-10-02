/** Serializes async read-modify-write ingress settings updates to prevent lost writes. */
export function createIngressMutationLock() {
	let chain: Promise<unknown> = Promise.resolve();

	return function withIngressLock<T>(fn: () => Promise<T>): Promise<T> {
		const run = chain.then(fn, fn);
		chain = run.then(
			() => undefined,
			() => undefined,
		);
		return run;
	};
}
