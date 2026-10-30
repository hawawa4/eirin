package updater

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Install downloads rel, verifies it against the release's SHA256SUMS and puts
// it in place for inst. On success the new version is on disk; the running
// process keeps going until it is restarted (see Relaunch).
func (c Client) Install(ctx context.Context, rel Release, inst Install, report func(Progress)) error {
	if inst.Kind == KindUnsupported {
		return errors.New("self-update isn't available on this platform")
	}
	name := inst.assetName()
	asset, ok := rel.asset(name)
	if !ok {
		return fmt.Errorf("release %s has no %s asset", rel.Version, name)
	}
	want, err := c.checksumFor(ctx, rel, name)
	if err != nil {
		return err
	}

	switch inst.Kind {
	case KindAppImage:
		return c.installFile(ctx, asset, want, inst.Path, report)
	case KindBinary:
		return c.installZip(ctx, asset, want, inst.Path, report)
	case KindDeb:
		return c.installDeb(ctx, asset, want, report)
	}
	return fmt.Errorf("unknown install kind %q", inst.Kind)
}

// installFile downloads straight over target (AppImage).
func (c Client) installFile(ctx context.Context, asset Asset, want, target string, report func(Progress)) error {
	tmp, err := tempBeside(target)
	if err != nil {
		return err
	}
	if err := c.download(ctx, asset, tmp, want, report); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	report(Progress{Phase: "installing"})
	if err := replaceFile(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// installZip downloads the release archive and replaces target with the
// executable inside it.
func (c Client) installZip(ctx context.Context, asset Asset, want, target string, report func(Progress)) error {
	// Fail early, before downloading, if the install directory isn't writable.
	staged, err := tempBeside(target)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(staged) }()

	dir, err := os.MkdirTemp("", "eirin-update-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	zipPath := filepath.Join(dir, asset.Name)

	if err := c.download(ctx, asset, zipPath, want, report); err != nil {
		return err
	}
	report(Progress{Phase: "installing"})
	if err := extractBinary(zipPath, staged); err != nil {
		return err
	}
	return replaceFile(staged, target)
}

// installDeb downloads the package and lets apt install it via pkexec.
func (c Client) installDeb(ctx context.Context, asset Asset, want string, report func(Progress)) error {
	if why := DebPrerequisites(); why != "" {
		return errors.New(why)
	}
	dir, err := os.MkdirTemp("", "eirin-update-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	// apt's sandbox user needs to read the package; otherwise it only warns.
	_ = os.Chmod(dir, 0o755)
	debPath := filepath.Join(dir, asset.Name)

	if err := c.download(ctx, asset, debPath, want, report); err != nil {
		return err
	}
	report(Progress{Phase: "installing"})
	return runDebInstall(ctx, debPath)
}
