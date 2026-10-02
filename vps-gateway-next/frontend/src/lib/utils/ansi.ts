// stripAnsi removes ECMA-48 CSI escape sequences (ESC `[` … <final byte>)
// from a string. Covers SGR colour codes (`\x1b[36m`, `\x1b[0m`) plus any
// other CSI form sing-box's logger or its dependencies might emit.
//
// Sing-box writes coloured log output to stdout/stderr even when piped;
// its config schema has no key to suppress colour, and the CLI flag
// --disable-color is reportedly buggy (SagerNet/sing-box#423). Raw ESC
// bytes therefore flow through Operator.LastError into the page-header
// tooltip and the /logs view, where they render as unprintable glyphs.
// Backend keeps the bytes intact; the frontend strips at render time.
const CSI_RE = /\x1b\[[0-?]*[ -/]*[@-~]/g;

export function stripAnsi(s: string | null | undefined): string {
	if (!s) return '';
	return s.replace(CSI_RE, '');
}
