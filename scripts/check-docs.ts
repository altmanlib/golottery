// 校验 docs/README.md「文档约定」中可机器检查的条款，以及仓库内 Markdown 的相对链接。
// 用法：bun scripts/check-docs.ts（仓库根目录执行，失败时退出码为 1）

import { existsSync, readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, join, relative, resolve } from "node:path";

const root = resolve(import.meta.dir, "..");
const docsDir = join(root, "docs");

// 不参与校验的目录：reference/ 为第三方手册原文，plan/ 为阶段内部临时笔记
const docsExcluded = ["docs/reference", "docs/plan"];
const skipDirs = new Set(["node_modules", ".git", "dist", "build", "src/api-gen"]);

const types = new Set(["guide", "runbook", "reference", "design", "record", "prd"]);
const statuses = new Set(["draft", "published", "deprecated"]);
const phaseStates = new Set(["未开始", "进行中", "已通过", "阻塞"]);

const errors: string[] = [];
const fail = (file: string, msg: string) => errors.push(`${relative(root, file)}: ${msg}`);

function walk(dir: string, out: string[] = []): string[] {
  for (const name of readdirSync(dir)) {
    const path = join(dir, name);
    const rel = relative(root, path);
    if (skipDirs.has(name) || skipDirs.has(rel)) continue;
    if (statSync(path).isDirectory()) walk(path, out);
    else if (name.endsWith(".md")) out.push(path);
  }
  return out;
}

const isExcluded = (file: string) => docsExcluded.some((d) => relative(root, file).startsWith(`${d}/`));

// 去掉围栏代码块，避免把示例当成标题或链接
function stripFences(text: string): string {
  return text.replace(/^(```|~~~)[^\n]*\n[\s\S]*?^\1[^\n]*$/gm, "");
}

function parseFrontMatter(text: string): Record<string, string> | null {
  const m = text.match(/^---\n([\s\S]*?)\n---\n/);
  if (!m) return null;
  const fields: Record<string, string> = {};
  for (const line of m[1].split("\n")) {
    const kv = line.match(/^([A-Za-z_]+):\s*(.*)$/);
    if (kv) fields[kv[1]] = kv[2].trim().replace(/^["']|["']$/g, "");
  }
  return fields;
}

function headings(text: string): { level: number; text: string }[] {
  return [...stripFences(text).matchAll(/^(#{1,6})\s+(.+?)\s*#*\s*$/gm)].map((m) => ({
    level: m[1].length,
    text: m[2],
  }));
}

// 与 GitHub 生成标题锚点的规则保持一致：小写、去标点、空格转连字符、重名追加 -n
function anchors(text: string): Set<string> {
  const seen = new Map<string, number>();
  const result = new Set<string>();
  for (const h of headings(text)) {
    const plain = h.text
      .replace(/!?\[([^\]]*)\]\([^)]*\)/g, "$1")
      .replace(/[`*_]/g, "")
      .toLowerCase();
    const base = plain.replace(/[^\p{L}\p{M}\p{N}\p{Pc} -]/gu, "").replace(/ /g, "-");
    const n = seen.get(base) ?? 0;
    seen.set(base, n + 1);
    result.add(n === 0 ? base : `${base}-${n}`);
  }
  return result;
}

const anchorCache = new Map<string, Set<string>>();
function anchorsOf(file: string): Set<string> {
  let set = anchorCache.get(file);
  if (!set) {
    set = anchors(readFileSync(file, "utf8"));
    anchorCache.set(file, set);
  }
  return set;
}

function checkLinks(file: string, text: string) {
  const body = stripFences(text).replace(/`[^`\n]*`/g, "");
  for (const m of body.matchAll(/!?\[[^\]]*\]\(([^)\s]+)(?:\s+"[^"]*")?\)/g)) {
    const href = m[1];
    if (/^[a-z][a-z0-9+.-]*:/i.test(href)) continue;
    const [pathPart, rawAnchor] = href.split("#", 2);
    const target = pathPart ? resolve(dirname(file), decodeURIComponent(pathPart)) : file;
    if (!existsSync(target)) {
      fail(file, `链接目标不存在：${href}`);
      continue;
    }
    if (rawAnchor !== undefined && target.endsWith(".md")) {
      const anchor = decodeURIComponent(rawAnchor).toLowerCase();
      if (!anchorsOf(target).has(anchor)) fail(file, `锚点不存在：${href}`);
    }
  }
}

function checkFrontMatter(file: string, text: string) {
  const fm = parseFrontMatter(text);
  if (!fm) {
    fail(file, "缺少 YAML front matter");
    return;
  }
  for (const key of ["title", "type", "status", "updated"]) {
    if (!fm[key]) fail(file, `front matter 缺少 ${key}`);
  }
  if (fm.type && !types.has(fm.type)) fail(file, `type 取值无效：${fm.type}`);
  const isPrd = relative(docsDir, file) === "PRD.md";
  if (fm.type === "prd" && !isPrd) fail(file, "type: prd 只用于 PRD.md");
  if (fm.status && !statuses.has(fm.status)) fail(file, `status 取值无效：${fm.status}`);
  if (fm.updated && !/^\d{4}-\d{2}-\d{2}$/.test(fm.updated)) fail(file, `updated 应为 YYYY-MM-DD：${fm.updated}`);

  const h1 = headings(text).filter((h) => h.level === 1);
  if (h1.length !== 1) fail(file, `应有且只有一个一级标题，实际 ${h1.length} 个`);
  else if (fm.title && h1[0].text !== fm.title) fail(file, `title「${fm.title}」与一级标题「${h1[0].text}」不一致`);

  const phase = relative(docsDir, file).match(/^phases\/phase-(\d+)-[a-z0-9-]+\.md$/);
  if (relative(docsDir, file).startsWith("phases/phase-") && !phase) {
    fail(file, "阶段方案文件名应为 phase-<n>-<slug>.md");
  }
  if (phase && fm.title && !fm.title.startsWith(`阶段 ${phase[1]}：`)) {
    fail(file, `阶段方案标题应为「阶段 ${phase[1]}：名称」`);
  }
}

// 表格中取某一列的所有单元格
function tableColumn(text: string, column: string): string[] {
  const values: string[] = [];
  const lines = text.split("\n");
  for (let i = 0; i < lines.length; i++) {
    const header = splitRow(lines[i]);
    const idx = header?.indexOf(column) ?? -1;
    if (idx < 0 || !lines[i + 1]?.match(/^\|\s*-/)) continue;
    for (let j = i + 2; j < lines.length && lines[j].startsWith("|"); j++) {
      values.push(splitRow(lines[j])?.[idx] ?? "");
    }
  }
  return values;
}

function splitRow(line: string): string[] | null {
  if (!line.startsWith("|")) return null;
  return line
    .replace(/^\|/, "")
    .replace(/\|\s*$/, "")
    .split("|")
    .map((c) => c.trim());
}

function checkPhaseStates() {
  const file = join(docsDir, "phases/README.md");
  for (const state of tableColumn(readFileSync(file, "utf8"), "状态")) {
    if (!phaseStates.has(state)) fail(file, `阶段状态取值无效：${state}`);
  }
}

function checkRoadmapIds() {
  const file = join(docsDir, "ROADMAP.md");
  const seen = new Set<string>();
  for (const id of tableColumn(readFileSync(file, "utf8"), "编号")) {
    if (!/^P-\d+$/.test(id)) fail(file, `编号格式无效：${id}`);
    else if (seen.has(id)) fail(file, `编号重复：${id}`);
    seen.add(id);
  }
}

const files = walk(root);
for (const file of files) {
  const text = readFileSync(file, "utf8");
  const inDocs = file.startsWith(`${docsDir}/`);
  if (inDocs && isExcluded(file)) continue;
  if (inDocs) checkFrontMatter(file, text);
  checkLinks(file, text);
}
checkPhaseStates();
checkRoadmapIds();

if (errors.length > 0) {
  for (const e of errors) console.error(e);
  console.error(`\n文档检查失败：${errors.length} 处`);
  process.exit(1);
}
console.log(`文档检查通过：${files.length} 个 Markdown 文件`);
