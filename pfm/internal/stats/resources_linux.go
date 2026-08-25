//go:build linux

package stats

func readHostResources(root string, _ int64, _ int) (
	uint64,
	uint64,
	Header,
	map[int]processSample,
	[]string,
	error,
) {
	if root == "" {
		root = "/proc"
	}
	return readLinuxHostResources(root)
}

func readDockerResources(root string) ([]Container, map[string]dockerSample, []string, error) {
	if root == "" {
		root = "/sys/fs/cgroup"
	}
	return readDocker(root)
}
