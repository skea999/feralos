package desktops

import (
	"fmt"
	"path/filepath"

	"feralos/src/utils/shell"
)

// brandingAssetNames lists the tracked repo assets installed into the target
// (repo-root branding/ — also referenced by README).
const (
	brandingLogoName        = "feralos-logo.svg"
	brandingWallpaper1080p  = "feralos-wallpaper-1080p.png"
	brandingWallpaper1440p  = "feralos-wallpaper-1440p.png"
	brandingWallpaperTarget = "/usr/share/wallpapers/feralos-default.png"
	brandingWallpaperUW     = "/usr/share/wallpapers/feralos-default-3440x1440.png"
)

// brandingDirs are the candidate repo locations of branding/, relative to the
// installer working directory (same multi-candidate pattern as createValues'
// templatePaths: repo root, src/, src/utils/createConfig/createValues/).
var brandingDirs = []string{"branding", filepath.Join("..", "branding"), filepath.Join("..", "..", "branding")}

// brandingAsset reads one asset from the repo tree.
func brandingAsset(name string) ([]byte, error) {
	var lastErr error
	for _, dir := range brandingDirs {
		data, err := shell.ReadFile(filepath.Join(dir, name))
		if err == nil {
			return data, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("asset %q not found in %v: %w", name, brandingDirs, lastErr)
}

// installBrandingAssets installs the FeralOS branding into the target: the
// logo as a system hicolor icon (Wayle/Raffi launcher button + general
// desktop identity; Adwaita inherits hicolor so `icon-name = "feralos-logo"`
// resolves) and the default wallpapers — 1080p canonical (the regreet
// background points here) plus the 3440x1440 ultrawide variant beside it.
// WriteFile mkdirs both parents. Cosmetic: callers treat a failure as a
// WARNING, never fatal.
func installBrandingAssets(chroot string) error {
	logo, err := brandingAsset(brandingLogoName)
	if err != nil {
		return err
	}
	if err = shell.WriteFile(chroot, "/usr/share/icons/hicolor/scalable/apps/"+brandingLogoName, logo, 0o644); err != nil {
		return fmt.Errorf("install logo: %w", err)
	}

	wp, err := brandingAsset(brandingWallpaper1080p)
	if err != nil {
		return err
	}
	if err = shell.WriteFile(chroot, brandingWallpaperTarget, wp, 0o644); err != nil {
		return fmt.Errorf("install wallpaper (1080p): %w", err)
	}

	wpUW, err := brandingAsset(brandingWallpaper1440p)
	if err != nil {
		return err
	}
	if err = shell.WriteFile(chroot, brandingWallpaperUW, wpUW, 0o644); err != nil {
		return fmt.Errorf("install wallpaper (1440p): %w", err)
	}
	return nil
}
