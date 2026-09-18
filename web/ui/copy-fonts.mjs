// Copies the Latin subsets of the theme typefaces into web/static/fonts so the
// server can serve them itself. Nothing is fetched from a CDN at runtime.
import { copyFileSync, mkdirSync } from "node:fs"
import { dirname, join } from "node:path"
import { fileURLToPath } from "node:url"

const here = dirname(fileURLToPath(import.meta.url))
const out = join(here, "..", "static", "fonts")
mkdirSync(out, { recursive: true })

const files = [
  ["@fontsource-variable/ibm-plex-sans/files/ibm-plex-sans-latin-wght-normal.woff2", "ibm-plex-sans-latin-wght-normal.woff2"],
  ["@fontsource-variable/ibm-plex-sans/files/ibm-plex-sans-latin-wght-italic.woff2", "ibm-plex-sans-latin-wght-italic.woff2"],
  ["@fontsource/ibm-plex-mono/files/ibm-plex-mono-latin-400-normal.woff2", "ibm-plex-mono-latin-400-normal.woff2"],
  ["@fontsource/ibm-plex-mono/files/ibm-plex-mono-latin-600-normal.woff2", "ibm-plex-mono-latin-600-normal.woff2"],
  ["@fontsource-variable/schibsted-grotesk/files/schibsted-grotesk-latin-wght-normal.woff2", "schibsted-grotesk-latin-wght-normal.woff2"],
  ["@fontsource-variable/schibsted-grotesk/files/schibsted-grotesk-latin-wght-italic.woff2", "schibsted-grotesk-latin-wght-italic.woff2"],
  ["@fontsource-variable/jetbrains-mono/files/jetbrains-mono-latin-wght-normal.woff2", "jetbrains-mono-latin-wght-normal.woff2"],
  ["@fontsource-variable/source-serif-4/files/source-serif-4-latin-wght-normal.woff2", "source-serif-4-latin-wght-normal.woff2"],
  ["@fontsource-variable/source-serif-4/files/source-serif-4-latin-wght-italic.woff2", "source-serif-4-latin-wght-italic.woff2"],
  ["@fontsource-variable/source-code-pro/files/source-code-pro-latin-wght-normal.woff2", "source-code-pro-latin-wght-normal.woff2"],
]

for (const [source, name] of files) {
  copyFileSync(join(here, "node_modules", source), join(out, name))
}
console.log(`copied ${files.length} font files to ${out}`)
