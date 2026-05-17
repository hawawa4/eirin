import type { ColumnDef } from "./types";

export interface ColumnManager {
  startColResize: (e: MouseEvent, colId: string) => void;
  onColDragStart: (e: DragEvent, visIdx: number) => void;
  onColDragOver: (e: DragEvent, visIdx: number) => void;
  onColDrop: (e: DragEvent, targetVisIdx: number) => void;
  onColDragEnd: () => void;
}

export function makeColumnManager(
  getColumns: () => ColumnDef[],
  setDragOverIndex: (i: number) => void,
  onsavecolumns: () => void,
): ColumnManager {
  let dragSourceId = "";

  function startColResize(e: MouseEvent, colId: string) {
    e.preventDefault();
    e.stopPropagation();
    const startX = e.clientX;
    const col = getColumns().find((c) => c.id === colId)!;
    const startWidth = col.width;

    function onMove(ev: MouseEvent) {
      col.width = Math.max(48, startWidth + ev.clientX - startX);
    }
    function onUp() {
      onsavecolumns();
      document.removeEventListener("mousemove", onMove);
      document.removeEventListener("mouseup", onUp);
    }
    document.addEventListener("mousemove", onMove);
    document.addEventListener("mouseup", onUp);
  }

  function visibleColumns(columns: ColumnDef[]) {
    return [...columns].filter((c) => c.visible).sort((a, b) => a.order - b.order);
  }

  function onColDragStart(e: DragEvent, visIdx: number) {
    dragSourceId = visibleColumns(getColumns())[visIdx].id;
    e.dataTransfer!.effectAllowed = "move";
  }

  function onColDragOver(e: DragEvent, visIdx: number) {
    e.preventDefault();
    e.dataTransfer!.dropEffect = "move";
    setDragOverIndex(visIdx);
  }

  function onColDrop(e: DragEvent, targetVisIdx: number) {
    e.preventDefault();
    setDragOverIndex(-1);
    if (!dragSourceId) return;

    const cols = getColumns();
    const vis = visibleColumns(cols);
    const srcVisIdx = vis.findIndex((c) => c.id === dragSourceId);
    dragSourceId = "";
    if (srcVisIdx === -1 || srcVisIdx === targetVisIdx) return;

    const newVis = [...vis];
    const [moved] = newVis.splice(srcVisIdx, 1);
    newVis.splice(targetVisIdx, 0, moved);

    let order = 0;
    for (const col of newVis) cols.find((c) => c.id === col.id)!.order = order++;
    for (const col of cols.filter((c) => !c.visible)) cols.find((c) => c.id === col.id)!.order = order++;

    onsavecolumns();
  }

  function onColDragEnd() {
    dragSourceId = "";
    setDragOverIndex(-1);
  }

  return { startColResize, onColDragStart, onColDragOver, onColDrop, onColDragEnd };
}
