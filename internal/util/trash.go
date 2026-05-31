package util

// MoveFileToTrash removes a user-visible file in the least destructive way
// available on the current platform.
func MoveFileToTrash(path string) error {
	return moveFileToTrash(path)
}
