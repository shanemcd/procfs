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
	"os"
	"testing"
)

func TestMemoryTiering(t *testing.T) {
	fs, err := NewFS("testdata/memory_tiering/sys")
	if err != nil {
		t.Fatal(err)
	}
	got, err := fs.MemoryTiering()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 tiers, got %d: %v", len(got), got)
	}
	tier0, ok := got[0]
	if !ok {
		t.Fatal("tier 0 missing")
	}
	if want := []int{0, 1}; !equalInts(tier0.Nodes, want) {
		t.Errorf("tier 0 nodes: got %v, want %v", tier0.Nodes, want)
	}
	if tier0.AbstractDistance != 200 {
		t.Errorf("tier 0 abstract distance: got %d, want 200", tier0.AbstractDistance)
	}
	tier2, ok := got[2]
	if !ok {
		t.Fatal("tier 2 missing")
	}
	if want := []int{2}; !equalInts(tier2.Nodes, want) {
		t.Errorf("tier 2 nodes: got %v, want %v", tier2.Nodes, want)
	}
	if tier2.HasAbstractDistance {
		t.Error("tier 2 should not report an abstract distance (attribute absent)")
	}
}

func TestMemoryTieringMissing(t *testing.T) {
	// Point at the sysfs root itself: no memory_tiering subsystem is present.
	fs, err := NewFS("testdata/memory_tiering/sys/devices")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.MemoryTiering(); err != os.ErrNotExist {
		t.Fatalf("expected os.ErrNotExist, got %v", err)
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
