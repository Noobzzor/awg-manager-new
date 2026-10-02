// See https://svelte.dev/docs/kit/types#app.d.ts

// swagger-ui-dist ships a plain UMD bundle with no TypeScript types.
// `unknown` вместо `any`: единственный потребитель (routes/api-docs) и так
// сужает значение до вызываемой сигнатуры на месте использования.
declare module 'swagger-ui-dist/swagger-ui-bundle.js' {
	const SwaggerUIBundle: unknown;
	export = SwaggerUIBundle;
}

declare namespace App {}
