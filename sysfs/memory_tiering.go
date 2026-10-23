// Copyright The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build linux

package sysfs

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// memoryTieringPath is the sysfs root of the Linux memory-tiering subsystem.
const memoryTieringPath = "devices/virtual/memory_tiering"

// MemoryTier describes one Linux memory tier: the set of NUMA nodes assigned
// to it and the kernel's abstract distance for that tier.
type MemoryTier struct {
	// Nodes lists the NUMA node IDs that are members of this tier.
	Nodes []int
	// AbstractDistance is the tier's abstract distance as reported by the
	// kernel (higher values are slower tiers; tier 0, typically DRAM, is the
	// reference at 200). It is 0 when the kernel does not expose the
	// abstract_distance attribute (HasAbstractDistance reports that case).
	AbstractDistance int
	// HasAbstractDistance reports whether the kernel exposed the
	// abstract_distance attribute for this tier.
	HasAbstractDistance bool
}

// MemoryTiering maps tier IDs to their description.
type MemoryTiering map[int]MemoryTier

// MemoryTiering reads /sys/devices/virtual/memory_tiering/memory_tierN/nodelist
// and .../abstract_distance for every discovered tier.
func (fs FS) MemoryTiering() (MemoryTiering, error) {
	matches, err := filepath.Glob(filepath.Join(fs.sys.Path(memoryTieringPath), "memory_tier[0-9]*"))
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, os.ErrNotExist
	}

	tiering := MemoryTiering{}
	for _, dir := range matches {
		tierID, err := parseMemoryTierID(dir)
		if err != nil {
			return nil, err
		}

		nodes, err := readTierNodeList(dir)
		if err != nil {
			return nil, err
		}

		distance, hasDistance, err := readTierAbstractDistance(dir)
		if err != nil {
			return nil, err
		}

		tiering[tierID] = MemoryTier{
			Nodes:               nodes,
			AbstractDistance:    distance,
			HasAbstractDistance: hasDistance,
		}
	}
	return tiering, nil
}

func parseMemoryTierID(dir string) (int, error) {
	base := filepath.Base(dir)
	id, err := strconv.Atoi(strings.TrimPrefix(base, "memory_tier"))
	if err != nil {
		return 0, fmt.Errorf("unexpected memory tier directory %q: %w", dir, err)
	}
	return id, nil
}

func readTierNodeList(dir string) ([]int, error) {
	data, err := os.ReadFile(filepath.Join(dir, "nodelist"))
	if err != nil {
		return nil, err
	}
	nodes, err := parseNodeList(bytes.TrimSpace(data))
	if err != nil {
		return nil, fmt.Errorf("invalid nodelist for %q: %w", dir, err)
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("empty nodelist for %q", dir)
	}
	sort.Ints(nodes)
	return nodes, nil
}

// parseNodeList parses a kernel "nodelist" such as "0" or "2-3,7" into the
// sorted list of NUMA node IDs it designates.
func parseNodeList(data []byte) ([]int, error) {
	var nodes []int
	for _, part := range strings.Split(string(data), ",") {
		if part == "" {
			continue
		}
		lo, hi, found := strings.Cut(part, "-")
		first, err := strconv.Atoi(strings.TrimSpace(lo))
		if err != nil {
			return nil, fmt.Errorf("invalid node list element %q: %w", part, err)
		}
		if !found {
			nodes = append(nodes, first)
			continue
		}
		last, err := strconv.Atoi(strings.TrimSpace(hi))
		if err != nil {
			return nil, fmt.Errorf("invalid node list element %q: %w", part, err)
		}
		for n := first; n <= last; n++ {
			nodes = append(nodes, n)
		}
	}
	return nodes, nil
}

// readTierAbstractDistance reads the tier's abstract_distance attribute.
// Kernels that do not implement the attribute report ok=false; the distance
// value is then 0. Any other read or parse failure is an error.
func readTierAbstractDistance(dir string) (int, bool, error) {
	data, err := os.ReadFile(filepath.Join(dir, "abstract_distance"))
	if errors.Is(err, os.ErrNotExist) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	distance, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, false, fmt.Errorf("invalid abstract_distance for %q: %w", dir, err)
	}
	return distance, true, nil
}
