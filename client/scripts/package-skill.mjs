import { lstat, mkdir, readFile, writeFile } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { zipSync } from "fflate";

const clientRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const skillRoot = resolve(clientRoot, "../skills/fileflow");
const outputPath = resolve(clientRoot, "public/assets/fileflow-skill.zip");

// Explicitly package installable sources; never include tests, runtime state,
// credentials, editor files, or Python caches from the skill working directory.
const sources = [
  "SKILL.md",
  "agents/openai.yaml",
  "references/operations.md",
  "scripts/fileflow.py",
];
const entries = {};
for (const name of sources) {
  const sourcePath = resolve(skillRoot, name);
  const info = await lstat(sourcePath);
  if (!info.isFile()) {
    throw new Error(`Skill source must be a regular file: ${name}`);
  }
  const contents = await readFile(sourcePath);
  if (!contents.length) {
    throw new Error(`Skill source must not be empty: ${name}`);
  }
  const mode = name === "scripts/fileflow.py" ? 0o100755 : 0o100644;
  entries[`fileflow/${name}`] = [contents, {
    level: 6,
    mtime: new Date(1980, 0, 1),
    os: 3,
    attrs: (mode << 16) >>> 0,
  }];
}

const archive = zipSync(entries);
await mkdir(dirname(outputPath), { recursive: true });
await writeFile(outputPath, archive);
process.stdout.write(`Packaged FileFlow skill: ${sources.length} files, ${archive.length} bytes.\n`);
