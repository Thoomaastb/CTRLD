package updater

func IsNewerVersionExported(latest, current string) bool {
	return isNewerVersion(latest, current)
}
