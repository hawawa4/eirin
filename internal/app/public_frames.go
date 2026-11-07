package app

// SetFilesPublic marks (or unmarks) final images as shown in the public
// gallery. Only marked images ever reach the headless server: the snapshot
// the server loads is pruned to them.
func (a *App) SetFilesPublic(paths []string, public bool) error {
	if err := a.requireWritable(); err != nil {
		return err
	}
	if a.store() == nil {
		return nil
	}
	return a.store().SetFramesPublic(paths, public)
}
