// +build !linux

package mock

// Find the total memory in the guest OS
func getSystemTotalMemory() uint64 {
	return 0
}
