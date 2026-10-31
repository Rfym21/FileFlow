import type { ZipPreviewEntry } from "@/lib/api";

export interface ZipDirectoryItem extends ZipPreviewEntry {
  id: string;
  label: string;
  path: string[];
}

export interface NormalizedZipEntry {
  entry: ZipPreviewEntry;
  id: string;
  path: string;
  isDir: boolean;
}

// These paths are only names inside the archive. In particular, '..' is kept
// as a literal directory name and is never resolved against the filesystem.
export function normalizeZipDirectoryEntries(entries: ZipPreviewEntry[]): NormalizedZipEntry[] {
  return entries.map((entry, index) => {
    let path = entry.name.replace(/\\/g, "/").replace(/\/+/g, "/").replace(/^\//, "");
    const isDir = entry.isDir || path.endsWith("/");
    while (path.startsWith("./")) path = path.slice(2);
    if (isDir && path.endsWith("/")) path = path.slice(0, -1);
    return { entry, id: `file:${index}`, path, isDir };
  });
}

// Only the visible level is assembled. Even an unusually deep ZIP filename
// does not create every parent directory or store every path prefix at once.
export function getZipDirectoryItems(entries: NormalizedZipEntry[], currentPath: string[]): ZipDirectoryItem[] {
  const prefix = currentPath.length > 0 ? `${currentPath.join("/")}/` : "";
  const directories = new Map<string, ZipDirectoryItem>();
  const files: ZipDirectoryItem[] = [];
  entries.forEach(({ entry, id, path, isDir }) => {
    if (!path.startsWith(prefix)) return;
    const relative = path.slice(prefix.length);
    if (relative === "" && isDir) return;
    const separator = relative.indexOf("/");
    if (separator >= 0 || isDir) {
      const label = separator >= 0 ? relative.slice(0, separator) : relative;
      const folderPath = [...currentPath, label];
      const folder = directories.get(label) || {
        id: `directory:${JSON.stringify(folderPath)}`,
        label,
        path: folderPath,
        name: `${prefix}${label}/`,
        size: 0,
        compressedSize: 0,
        isDir: true,
      };
      if (separator < 0) {
        folder.name = entry.name;
        folder.modifiedAt = entry.modifiedAt;
      }
      directories.set(label, folder);
    } else {
      files.push({ ...entry, id, label: relative || entry.name || "（未命名）", path: [...currentPath, relative], isDir: false });
    }
  });
  const compareNames = (a: ZipDirectoryItem, b: ZipDirectoryItem) => a.label.localeCompare(b.label, "zh-CN", { numeric: true });
  return [...[...directories.values()].sort(compareNames), ...files.sort(compareNames)];
}
