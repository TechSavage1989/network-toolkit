package main

// readSharedFile reads a file of the data folder that another app of the
// folder may be replacing at this very moment.
func readSharedFile(path string, limit int) ([]byte, error) {
	return readCredentialFile(path, limit)
}
