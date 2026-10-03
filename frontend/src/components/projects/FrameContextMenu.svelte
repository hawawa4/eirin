<script lang="ts" module>
  import type * as app from "$models/app";

  /** What was right-clicked: a frame, plus the paths the menu acts on. */
  export interface FrameMenuTarget {
    x: number;
    y: number;
    frame: app.LibraryFrame;
    paths: string[];
  }
</script>

<script lang="ts">
  import {
    BatchHardDeleteFiles,
    HardDeleteFile,
    OpenWithSiril,
    RemoveFramesFromProject,
  } from "$app";
  import type { CtxMenuState } from "../../lib/types";
  import { attempt, toast } from "../../lib/toast.svelte";
  import { copyPaths } from "../../lib/clipboard";
  import { revealPath } from "../../lib/shell/fileActions";
  import { plural } from "../../lib/utils";
  import ContextMenu from "../ContextMenu.svelte";
  import HardDeleteModal from "../HardDeleteModal.svelte";

  interface Props {
    target: FrameMenuTarget | null;
    project: app.Project;
    sirilAvailable: boolean;
    onclose: () => void;
    /** Removes frames from the project (the library files stay). */
    onremove: (paths: string[]) => Promise<void>;
    /** Called after files were deleted from disk, to reload the project. */
    ondeleted: () => void;
    onopeninlibrary?: (nasPath: string) => void;
  }

  let { target, project, sirilAvailable, onclose, onremove, ondeleted, onopeninlibrary }: Props =
    $props();

  let menu = $derived<CtxMenuState | null>(
    target && {
      x: target.x,
      y: target.y,
      entry: {
        path: target.frame.nasPath,
        name: target.frame.fileName,
        isRejected: target.frame.isRejected,
        frameType: target.frame.frameType,
      },
      sirilAvailable: sirilAvailable && target.paths.length === 1,
      selectionCount: target.paths.length,
    },
  );

  let confirmDel = $state<{ paths: string[]; name: string } | null>(null);

  function askDelete() {
    if (!target) return;
    const { paths, frame } = target;
    onclose();
    confirmDel = { paths, name: paths.length === 1 ? frame.fileName : `${paths.length} frames` };
  }

  /** Removes the frames from the project first, so no dangling link is left behind. */
  async function deleteFromDisk() {
    const del = confirmDel;
    confirmDel = null;
    if (!del) return;
    const { paths } = del;
    const ok = await attempt(
      async () => {
        await RemoveFramesFromProject(project.folder, paths);
        if (paths.length === 1) await HardDeleteFile(paths[0]);
        else await BatchHardDeleteFiles(paths);
        return true;
      },
      `Could not delete ${plural(paths.length, "frame")}`,
    );
    ondeleted();
    if (ok) toast.success(`Deleted ${plural(paths.length, "frame")} from disk`);
  }
</script>

{#if menu && target}
  {@const paths = target.paths}
  <ContextMenu
    {menu}
    {onclose}
    onopensiril={(entry) => {
      onclose();
      toast.info("Opening in Siril…");
      attempt(() => OpenWithSiril(entry.path), "Could not open the file in Siril");
    }}
    onopeninlibrary={onopeninlibrary ? (entry) => onopeninlibrary(entry.path) : undefined}
    onreveal={(entry) => revealPath(entry.path)}
    oncopypath={() => copyPaths(paths)}
    onremovefromproject={() => void onremove(paths).catch(() => {})}
    onharddelete={askDelete}
  />
{/if}

{#if confirmDel}
  <HardDeleteModal
    target={confirmDel}
    onconfirm={deleteFromDisk}
    oncancel={() => (confirmDel = null)}
  />
{/if}
