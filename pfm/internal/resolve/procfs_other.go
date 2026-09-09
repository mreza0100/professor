//go:build !linux && !darwin

package resolve

func newNativeProcFS(root string) ProcFS {
	if root == "" {
		root = "/proc"
	}
	return fileProcFS{root: root}
}
