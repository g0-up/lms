#!/usr/bin/env node
/**
 * Design-system audit for apps/web/src. Exits 1 when it finds:
 *  - raw hex colours outside the token files (styles/goup-tokens.css, styles/app.css);
 *  - emoji (icons come from lucide-react);
 *  - font sizes below 12px (`text-[Npx]`, `text-[Nrem]`, CSS `font-size`, inline `fontSize`);
 *  - interactive JSX elements whose own height/size class is below 44px without a 44px
 *    override (`min-h-11`, `size-11`, `min-h-[44px]`, optionally behind a breakpoint prefix).
 *
 * The hit-area rule is a heuristic over JSX opening tags; it cannot see sizes inherited from
 * parents. A deliberate exception is marked on the same or the preceding line with
 *   design-audit-allow <rule>: <reason>
 * where <rule> is one of hex, emoji, font-size, hit-area.
 */
import { readdirSync, readFileSync, statSync } from "node:fs";
import { extname, join, relative, sep } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = fileURLToPath(new URL("..", import.meta.url));
const SRC = join(ROOT, "src");
const TOKEN_FILES = new Set(["src/styles/goup-tokens.css", "src/styles/app.css"]);
const EXTENSIONS = new Set([".ts", ".tsx", ".css", ".js", ".jsx", ".mjs"]);
const MIN_FONT_PX = 12;
const MIN_HIT_PX = 44;
const SPACING_PX = 4;

const HEX = /#[0-9a-fA-F]{3,8}\b/g;
const EMOJI = /[\p{Extended_Pictographic}\u{FE0F}]/gu;
const FONT_SIZES = [
  { re: /text-\[(\d+(?:\.\d+)?)px\]/g, toPx: (n) => n },
  { re: /text-\[(\d+(?:\.\d+)?)rem\]/g, toPx: (n) => n * 16 },
  { re: /font-size:\s*(\d+(?:\.\d+)?)px/g, toPx: (n) => n },
  { re: /fontSize:\s*["']?(\d+(?:\.\d+)?)(?:px)?\b/g, toPx: (n) => n },
];

/** Opening tags that receive pointer input themselves. */
const INTERACTIVE_TAG =
  /<(button|a|Link|NavLink|input|select|textarea|Button|IconButton|[A-Z]\w*\.(?:Trigger|Item|CheckboxItem|RadioItem|Close)|(?:Checkbox|Switch|Toggle|Radio)\w*\.Root)\b/g;
/** Unprefixed height-defining utilities: `h-7`, `size-[18px]`, `min-h-9`. */
const HEIGHT_CLASS = /(?<![\w:[&-])(?:min-h|h|size)-(\d+(?:\.\d+)?|\[\d+(?:\.\d+)?px\])(?![\w.-])/g;
const HIT_OVERRIDE = /(?<![\w-])(?:[\w[\]-]+:)?(?:min-h|h|size)-(?:11|\[44px\])(?![\w.-])/;

function walk(dir) {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) return walk(path);
    return EXTENSIONS.has(extname(name)) ? [path] : [];
  });
}

function lineOf(text, index) {
  let line = 1;
  for (let i = 0; i < index; i += 1) if (text.charCodeAt(i) === 10) line += 1;
  return line;
}

function isAllowed(lines, line, rule) {
  const marker = new RegExp(`design-audit-allow ${rule}\\b`);
  return marker.test(lines[line - 1] ?? "") || marker.test(lines[line - 2] ?? "");
}

/** Returns the source of a JSX opening tag starting at `start`, honouring nested braces and strings. */
function openingTag(text, start) {
  let depth = 0;
  let quote = null;
  for (let i = start + 1; i < text.length; i += 1) {
    const ch = text[i];
    if (quote) {
      if (ch === "\\") i += 1;
      else if (ch === quote) quote = null;
      continue;
    }
    if (ch === '"' || ch === "'" || ch === "`") quote = ch;
    else if (ch === "{") depth += 1;
    else if (ch === "}") depth -= 1;
    else if (ch === ">" && depth === 0) return text.slice(start, i + 1);
  }
  return text.slice(start);
}

function heightPx(token) {
  return token.startsWith("[") ? Number.parseFloat(token.slice(1)) : Number.parseFloat(token) * SPACING_PX;
}

function audit(file) {
  const rel = relative(ROOT, file).split(sep).join("/");
  const text = readFileSync(file, "utf8");
  const lines = text.split("\n");
  const found = [];
  const report = (rule, index, message) => {
    const line = lineOf(text, index);
    if (!isAllowed(lines, line, rule)) found.push(`${rel}:${line}  [${rule}] ${message}`);
  };

  if (!TOKEN_FILES.has(rel)) {
    for (const m of text.matchAll(HEX)) report("hex", m.index, `raw colour ${m[0]}; use a token utility`);
  }
  for (const m of text.matchAll(EMOJI)) report("emoji", m.index, `emoji U+${m[0].codePointAt(0).toString(16)}; use a lucide icon`);
  for (const { re, toPx } of FONT_SIZES) {
    for (const m of text.matchAll(re)) {
      const px = toPx(Number.parseFloat(m[1]));
      if (px < MIN_FONT_PX) report("font-size", m.index, `${m[0]} is ${px}px (< ${MIN_FONT_PX}px)`);
    }
  }
  if (extname(file) === ".tsx" || extname(file) === ".jsx") {
    for (const m of text.matchAll(INTERACTIVE_TAG)) {
      const tag = openingTag(text, m.index);
      if (HIT_OVERRIDE.test(tag)) continue;
      for (const h of tag.matchAll(HEIGHT_CLASS)) {
        const px = heightPx(h[1]);
        if (px < MIN_HIT_PX) {
          report("hit-area", m.index + h.index, `<${m[1]}> ${h[0]} is ${px}px (< ${MIN_HIT_PX}px) without a 44px override`);
        }
      }
    }
  }
  return found;
}

const violations = walk(SRC).flatMap(audit);
if (violations.length > 0) {
  console.error(violations.join("\n"));
  console.error(`\ndesign-audit: ${violations.length} violation(s)`);
  process.exit(1);
}
console.log("design-audit: 0 violations");
