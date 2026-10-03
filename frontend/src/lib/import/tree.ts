// ── Import candidate tree ──────────────────────────────────────────────────

import type * as importer from "$models/importer";

export type Candidate = importer.Candidate;

interface FolderNode {
  name: string;
  folderPath: string;
  subfolders: FolderNode[];
  files: Candidate[];
}

export type FlatRow =
  | {
      kind: "folder";
      depth: number;
      name: string;
      folderPath: string;
      count: number;
      size: number;
      /** Library folder (relative to the root) that this folder's own files go to, if any. */
      destFolder: string | null;
    }
  | { kind: "file"; depth: number; name: string; candidate: Candidate; dest: string };

const SEP = /[\\/]/;

export function buildTree(items: Candidate[]): FolderNode {
  const root: FolderNode = { name: "", folderPath: "", subfolders: [], files: [] };
  for (const c of items) {
    const parts = c.relativePath.split(SEP);
    let node = root;
    for (let i = 0; i < parts.length - 1; i++) {
      const seg = parts[i];
      let child = node.subfolders.find((f) => f.name === seg);
      if (!child) {
        const fp = node.folderPath ? node.folderPath + "/" + seg : seg;
        child = { name: seg, folderPath: fp, subfolders: [], files: [] };
        node.subfolders.push(child);
      }
      node = child;
    }
    node.files.push(c);
  }
  return root;
}

function stats(node: FolderNode): { count: number; size: number } {
  let count = node.files.length;
  let size = node.files.reduce((s, c) => s + c.fileSize, 0);
  for (const sub of node.subfolders) {
    const s = stats(sub);
    count += s.count;
    size += s.size;
  }
  return { count, size };
}

/** Destination relative to the library root (falls back to the absolute path). */
export function relativeDest(destPath: string, rootFolder: string): string {
  if (!rootFolder) return destPath;
  const root = rootFolder.replace(/[\\/]+$/, "");
  if (destPath.startsWith(root) && SEP.test(destPath.charAt(root.length))) {
    return destPath.slice(root.length + 1);
  }
  return destPath;
}

function dirOf(rel: string): string {
  const i = Math.max(rel.lastIndexOf("/"), rel.lastIndexOf("\\"));
  return i === -1 ? "" : rel.slice(0, i);
}

export function flattenTree(
  node: FolderNode,
  collapsed: ReadonlySet<string>,
  rootFolder: string,
  depth = 0,
): FlatRow[] {
  const rows: FlatRow[] = [];
  for (const sub of node.subfolders) {
    const { count, size } = stats(sub);
    const first = sub.files[0];
    rows.push({
      kind: "folder",
      depth,
      name: sub.name,
      folderPath: sub.folderPath,
      count,
      size,
      destFolder: first ? dirOf(relativeDest(first.destPath, rootFolder)) : null,
    });
    if (!collapsed.has(sub.folderPath)) {
      rows.push(...flattenTree(sub, collapsed, rootFolder, depth + 1));
    }
  }
  for (const c of node.files) {
    rows.push({
      kind: "file",
      depth,
      name: c.relativePath.split(SEP).pop() ?? c.relativePath,
      candidate: c,
      dest: relativeDest(c.destPath, rootFolder),
    });
  }
  return rows;
}

export function allFolderPaths(node: FolderNode): string[] {
  const out: string[] = [];
  const walk = (n: FolderNode) => {
    for (const sub of n.subfolders) {
      out.push(sub.folderPath);
      walk(sub);
    }
  };
  walk(node);
  return out;
}
