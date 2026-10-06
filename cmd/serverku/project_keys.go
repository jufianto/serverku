package main

// projectSSHKeyPath resolves the same recorded identity used by provisioning.
func projectSSHKeyPath(name string) (string, error) {
	cfg, err := store.LoadProject(name)
	if err != nil {
		return "", err
	}
	state, err := store.LoadState(name)
	if err != nil {
		return "", err
	}
	key, err := store.ResolveProjectSSHKey(cfg, state, false)
	if err != nil {
		return "", err
	}
	return key.PrivatePath, nil
}
