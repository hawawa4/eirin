package app

import (
	"github.com/TaruDesigns/eirin/internal/prefs"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// StorageNode is one node in the storage treemap hierarchy.
type StorageNode struct {
	Label      string        `json:"label"`
	TotalBytes int64         `json:"totalBytes"`
	FrameCount int           `json:"frameCount"`
	Children   []StorageNode `json:"children,omitempty"`
}

type storageLeaf struct {
	bytes int64
	count int
}

// GetStorageStats returns a two-level tree of disk usage:
// root → objects → dates (YYYY-MM-DD), each leaf summing file_size and frame count.
func (a *App) GetStorageStats(rootPath string) StorageNode {
	frames, err := a.prefs.GetAllFramesUnder(rootPath)
	if err != nil {
		runtime.LogErrorf(a.ctx, "storage: get frames: %v", err)
		return StorageNode{Label: rootPath}
	}

	type dateKey struct{ object, date string }
	leaves := map[dateKey]*storageLeaf{}
	objectOrder := []string{}
	seenObjects := map[string]bool{}

	for _, f := range frames {
		obj := f.Object
		if obj == "" {
			obj = "(unknown)"
		}
		date := ""
		if len(f.DateObs) >= 10 {
			date = f.DateObs[:10]
		}
		if date == "" {
			date = "(no date)"
		}
		k := dateKey{obj, date}
		if _, ok := leaves[k]; !ok {
			leaves[k] = &storageLeaf{}
		}
		leaves[k].bytes += f.FileSize
		leaves[k].count++
		if !seenObjects[obj] {
			seenObjects[obj] = true
			objectOrder = append(objectOrder, obj)
		}
	}

	root := StorageNode{Label: rootPath}
	for _, obj := range objectOrder {
		objNode := StorageNode{Label: obj}
		for k, l := range leaves {
			if k.object != obj {
				continue
			}
			objNode.Children = append(objNode.Children, StorageNode{
				Label:      k.date,
				TotalBytes: l.bytes,
				FrameCount: l.count,
			})
			objNode.TotalBytes += l.bytes
			objNode.FrameCount += l.count
		}
		root.Children = append(root.Children, objNode)
		root.TotalBytes += objNode.TotalBytes
		root.FrameCount += objNode.FrameCount
	}

	return root
}

// GetFrameTypeSummary returns a breakdown of frame counts and total size by frame type under rootPath.
func (a *App) GetFrameTypeSummary(rootPath string) []StorageNode {
	frames, err := a.prefs.GetAllFramesUnder(rootPath)
	if err != nil {
		runtime.LogErrorf(a.ctx, "storage: frame type summary: %v", err)
		return nil
	}

	typeOrder := []string{
		prefs.FrameTypeLight, prefs.FrameTypeDark, prefs.FrameTypeFlat,
		prefs.FrameTypeBias, prefs.FrameTypeStacked, prefs.FrameTypeProcessed,
	}
	byType := map[string]*storageLeaf{}
	for _, t := range typeOrder {
		byType[t] = &storageLeaf{}
	}
	for _, f := range frames {
		if _, ok := byType[f.FrameType]; !ok {
			byType[f.FrameType] = &storageLeaf{}
		}
		byType[f.FrameType].bytes += f.FileSize
		byType[f.FrameType].count++
	}

	var result []StorageNode
	for _, t := range typeOrder {
		if l := byType[t]; l.count > 0 {
			result = append(result, StorageNode{Label: t, TotalBytes: l.bytes, FrameCount: l.count})
		}
	}
	return result
}
