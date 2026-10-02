#!/usr/bin/env node
// Generates valibot runtime schemas for API responses from the backend's
// swagger spec (internal/openapi/swagger.yaml, produced by swaggo from Go
// annotations). Output: src/lib/api/schemas.gen.ts.
//
//   npm run gen:api
//
// Design decisions (keep in sync with src/lib/api/validate.ts):
//  - Only definitions REACHABLE from 2xx response schemas are emitted, so
//    request DTOs never reach the bundle.
//  - Every object property is optional AND nullable: Go's `omitempty` drops
//    fields, nil slices/maps/pointers marshal to `null`. Strict-mode failures
//    must mean "present value of the wrong type" (real drift), never
//    "backend omitted a field".
//  - Objects are loose (unknown keys allowed): a newer backend adding fields
//    must not break an older cached frontend.
//  - Enums are emitted as their base type, not picklist: the backend adding
//    an enum value must not break an older frontend.
//  - $refs are emitted as v.lazy(() => X) so declaration order and cycles
//    never matter.

import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import yaml from 'js-yaml';

const here = path.dirname(fileURLToPath(import.meta.url));
const specPath = path.resolve(here, '../../internal/openapi/swagger.yaml');
const outPath = path.resolve(here, '../src/lib/api/schemas.gen.ts');

const spec = yaml.load(fs.readFileSync(specPath, 'utf8'));
if (!spec || spec.swagger !== '2.0') {
	console.error(`Ожидался swagger 2.0 в ${specPath}`);
	process.exit(1);
}

const definitions = spec.definitions ?? {};

/** 'api.AccessPolicyDTO' -> 'api_AccessPolicyDTO' (valid TS identifier). */
function ident(defName) {
	return defName.replace(/[^A-Za-z0-9_]/g, '_');
}

function refName(ref) {
	const m = /^#\/definitions\/(.+)$/.exec(ref);
	if (!m) throw new Error(`Неподдерживаемый $ref: ${ref}`);
	return m[1];
}

// ── Reachability: collect definitions used by 2xx response schemas ──────────

const reachable = new Set();

function markSchema(schema) {
	if (!schema || typeof schema !== 'object') return;
	if (schema.$ref) {
		const name = refName(schema.$ref);
		if (!reachable.has(name)) {
			reachable.add(name);
			markSchema(definitions[name]);
		}
		return;
	}
	if (Array.isArray(schema.allOf)) schema.allOf.forEach(markSchema);
	if (schema.items) markSchema(schema.items);
	if (schema.properties) Object.values(schema.properties).forEach(markSchema);
	if (schema.additionalProperties && typeof schema.additionalProperties === 'object') {
		markSchema(schema.additionalProperties);
	}
}

// ── Schema expression emission ───────────────────────────────────────────────

/** Bare value schema (no optional/nullable wrapper). */
function emitValue(schema) {
	if (!schema || typeof schema !== 'object') return 'v.unknown()';
	if (schema.$ref) {
		return `v.lazy(() => ${ident(refName(schema.$ref))})`;
	}
	if (Array.isArray(schema.allOf)) {
		// swaggo emits property-level allOf:[$ref] as "ref with description";
		// multi-member allOf is a real intersection (envelope + typed data).
		const parts = schema.allOf.map(emitValue);
		return parts.length === 1 ? parts[0] : `v.intersect([${parts.join(', ')}])`;
	}
	switch (schema.type) {
		case 'string':
			return 'v.string()'; // enum намеренно не сужаем — см. шапку файла
		case 'integer':
		case 'number':
			return 'v.number()';
		case 'boolean':
			return 'v.boolean()';
		case 'array':
			return `v.array(${emitValue(schema.items)})`;
		case 'file':
			return 'v.unknown()';
		case 'object': {
			if (schema.properties) return emitObject(schema);
			if (schema.additionalProperties && typeof schema.additionalProperties === 'object') {
				return `v.record(v.string(), ${emitValue(schema.additionalProperties)})`;
			}
			if (schema.additionalProperties === true) {
				return 'v.record(v.string(), v.unknown())';
			}
			return 'v.unknown()'; // нетипизированный object (например, envelope data)
		}
		default:
			return 'v.unknown()';
	}
}

function emitObject(schema) {
	const props = Object.entries(schema.properties ?? {})
		.map(([key, propSchema]) => {
			const safeKey = /^[A-Za-z_$][A-Za-z0-9_$]*$/.test(key) ? key : JSON.stringify(key);
			return `\t${safeKey}: v.optional(v.nullable(${emitValue(propSchema)})),`;
		})
		.join('\n');
	return props ? `v.looseObject({\n${props}\n})` : 'v.looseObject({})';
}

// ── Walk paths → response schema map ─────────────────────────────────────────

const METHODS = ['get', 'post', 'put', 'delete', 'patch'];
const responseEntries = []; // {key: 'GET /path', expr}

for (const [rawPath, ops] of Object.entries(spec.paths ?? {})) {
	for (const method of METHODS) {
		const op = ops?.[method];
		if (!op) continue;
		// SSE/бинарные ответы не проходят через ApiClient.request() и не
		// являются JSON-конвертами — им нечего валидировать.
		const produces = op.produces ?? spec.produces;
		if (Array.isArray(produces) && !produces.includes('application/json')) continue;
		const okResponse = op.responses?.['200'] ?? op.responses?.['201'] ?? op.responses?.['202'];
		const schema = okResponse?.schema;
		if (!schema) continue;
		if (schema.type === 'file') continue; // бинарные скачивания не проходят через request()
		markSchema(schema);
		responseEntries.push({
			key: `${method.toUpperCase()} ${rawPath}`,
			expr: emitValue(schema),
		});
	}
}

// ── Emit ─────────────────────────────────────────────────────────────────────

const defLines = [...reachable]
	.sort()
	.map((name) => {
		const schema = definitions[name];
		if (!schema) {
			throw new Error(`$ref на несуществующий definition: ${name}`);
		}
		return `const ${ident(name)}: v.GenericSchema = ${emitValue(schema)};`;
	})
	.join('\n\n');

const mapLines = responseEntries
	.sort((a, b) => a.key.localeCompare(b.key))
	.map(({ key, expr }) => `\t${JSON.stringify(key)}: ${expr},`)
	.join('\n');

const out = `// GENERATED FILE — do not edit by hand.
// Source: internal/openapi/swagger.yaml (swaggo). Regenerate: npm run gen:api
// Generator: scripts/gen-api-schemas.mjs (см. там же дизайн-решения).
/* eslint-disable */
import * as v from 'valibot';

${defLines}

/**
 * 2xx response envelope schema per "METHOD /path" (path как в swagger,
 * без basePath /api; шаблонные сегменты — {param}).
 */
export const RESPONSE_SCHEMAS: Record<string, v.GenericSchema> = {
${mapLines}
};
`;

fs.writeFileSync(outPath, out);
console.log(
	`schemas.gen.ts: ${responseEntries.length} эндпоинтов, ${reachable.size} DTO (из ${Object.keys(definitions).length} в спеке)`,
);
