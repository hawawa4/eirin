<script lang="ts">
  import { SelectProjectsFolder, SetProjectsFolder } from "$app";
  import { toast } from "../../lib/toast.svelte";
  import { openFolder } from "../../lib/shell/fileActions";

  interface Props {
    projectsFolder: string;
    onprojectsfolderset: (path: string) => void;
  }

  let { projectsFolder, onprojectsfolderset }: Props = $props();

  async function browse() {
    let path: string;
    try {
      path = await SelectProjectsFolder();
    } catch (e) {
      toast.error(`Couldn't open the folder picker: ${String(e)}`);
      return;
    }
    if (!path) return;
    try {
      await SetProjectsFolder(path);
    } catch (e) {
      toast.error(`Couldn't set the projects folder: ${String(e)}`);
      return;
    }
    onprojectsfolderset(path);
    toast.success("Projects folder updated");
  }
</script>

<section class="card">
  <h2 class="section-title">Projects Folder</h2>
  <p class="section-desc">
    Local folder where Siril projects are stored. Each project gets its own subfolder containing a
    <code>lights/</code> directory with symlinks or copies of your frames.
  </p>

  <div class="path-row">
    <span class="path-value" title={projectsFolder || "Not set"}>
      {projectsFolder || "No folder selected"}
    </span>
    {#if projectsFolder}
      <button class="btn-ghost small" onclick={() => openFolder(projectsFolder)}>
        Show in folder
      </button>
    {/if}
    <button class="btn-secondary" onclick={browse}>
      {projectsFolder ? "Change…" : "Select…"}
    </button>
  </div>
</section>
