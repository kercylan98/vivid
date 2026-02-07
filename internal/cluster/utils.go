package cluster

import "github.com/kercylan98/vivid/internal/utils"

var discoverTickMessage = new(discoverTick)

type discoverTick struct{}

func normalizeSeeds(addrs []string) []string {
	if len(addrs) == 0 {
		return nil
	}
	var out []string
	for _, addr := range addrs {
		if n, ok := utils.NormalizeAddress(addr); ok {
			out = append(out, n)
		}
	}
	return out
}
