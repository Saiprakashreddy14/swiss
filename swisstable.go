package swisstable

import (
	"fmt"
	"hash/maphash"
	"math/bits"
	"strings"
	"unsafe"
)

const (
	// Size of each SIMD group (16 bytes for SSE)
	groupSize = 16
	// Initial size of the table (must be multiple of groupSize)
	initialSize = 16
	// Load factor threshold for resizing
	loadFactor = 0.75
	// Number of bits used for the H2 hash
	h2Bits = 7
	// Mask for extracting H2 hash
	h2Mask = (1 << h2Bits) - 1
)

// metadata represents a SIMD-friendly group of control bytes
type metadata struct {
	bytes [groupSize]uint8
}

// SwissTable implements a Swiss Table hash map with SIMD operations
type SwissTable struct {
	// The actual key-value pairs
	entries []entry
	// Control bytes for metadata, organized in SIMD-friendly groups
	metadata []metadata
	// Number of elements in the table
	size int
	// Hash seed for the hash function
	hashSeed maphash.Seed
	// Number of groups (len(metadata))
	groupCount int
}

// entry represents a key-value pair in the table
type entry struct {
	key   any
	value any
	// H2 hash helps in SIMD comparison
	h2Hash uint8
}

// New creates a new SwissTable with initial capacity
func New() *SwissTable {
	groupCount := initialSize / groupSize
	st := &SwissTable{
		entries:    make([]entry, initialSize),
		metadata:   make([]metadata, groupCount),
		size:       0,
		hashSeed:   maphash.MakeSeed(),
		groupCount: groupCount,
	}
	// Initialize all metadata bytes to empty
	for i := range st.metadata {
		for j := range st.metadata[i].bytes {
			st.metadata[i].bytes[j] = 0
		}
	}
	return st
}

// hashKey generates both H1 (group index) and H2 (metadata) hashes
func (st *SwissTable) hashKey(key any) (h1 uint64, h2 uint8) {
	var h maphash.Hash
	h.SetSeed(st.hashSeed)
	fmt.Fprintf(&h, "%v", key)
	hash := h.Sum64()

	// H1 determines the group (high bits)
	h1 = hash >> h2Bits

	// H2 is used for SIMD matching (low bits)
	h2 = uint8(hash & h2Mask)

	// Ensure h2 is never zero (zero is used for empty slots)
	if h2 == 0 {
		h2 = 1
	}

	return h1, h2
}

// Simulating SIMD operations
// Returns a bitmask where each bit represents a matching position
func (st *SwissTable) matchGroup(group *metadata, h2 uint8) uint16 {
	// Create a vector with the target H2 hash
	target := uint64(h2) * 0x0101010101010101

	// Load the group's bytes into a 64-bit integer
	group1 := *(*uint64)(unsafe.Pointer(&group.bytes[0]))
	group2 := *(*uint64)(unsafe.Pointer(&group.bytes[8]))

	// Compare with target to find matches
	matches1 := group1
	matches2 := group2
	if h2 == 0 {
		// Looking for empty slots
		matches1 = ^matches1
		matches2 = ^matches2
	} else {
		// Looking for matching H2 hashes
		matches1 = ^(matches1 ^ target)
		matches2 = ^(matches2 ^ target)
	}

	// Create match mask (1 bit per matching byte)
	mask1 := uint8(0)
	mask2 := uint8(0)

	// Check each byte
	for i := 0; i < 8; i++ {
		if (matches1>>(i*8))&0xFF == 0xFF {
			mask1 |= 1 << i
		}
		if (matches2>>(i*8))&0xFF == 0xFF {
			mask2 |= 1 << i
		}
	}

	return uint16(mask1) | (uint16(mask2) << 8)
}

// findSlot finds the appropriate slot for a key using SIMD
func (st *SwissTable) findSlot(key any) (int, bool) {
	// Get both hashes
	h1, h2 := st.hashKey(key)

	// Find initial group
	groupIdx := h1 % uint64(st.groupCount)
	originalGroup := groupIdx

	// First try to find the key
	for {
		// Get matches within the current group
		matches := st.matchGroup(&st.metadata[groupIdx], h2)

		// Check each matching position
		for matches != 0 {
			// Find next match (rightmost 1 bit)
			pos := bits.TrailingZeros16(matches)
			// Clear the bit
			matches &= matches - 1

			// Calculate actual index
			idx := int(groupIdx)*groupSize + pos

			// Check if keys match
			if st.entries[idx].key == key {
				return idx, true
			}
		}

		// Move to next group
		groupIdx = (groupIdx + 1) % uint64(st.groupCount)
		if groupIdx == originalGroup {
			break
		}
	}

	// Key not found, look for an empty slot starting from h1's group
	groupIdx = h1 % uint64(st.groupCount)
	for i := uint64(0); i < uint64(st.groupCount); i++ {
		currGroup := (groupIdx + i) % uint64(st.groupCount)
		matches := st.matchGroup(&st.metadata[currGroup], 0) // Find empty slots
		if matches != 0 {
			pos := bits.TrailingZeros16(matches)
			return int(currGroup)*groupSize + pos, false
		}
	}

	return -1, false // Table is full
}

// resize grows the table when it becomes too full
func (st *SwissTable) resize() {
	oldEntries := st.entries
	oldMetadata := st.metadata

	// Double the size
	newSize := len(st.entries) * 2
	newGroupCount := newSize / groupSize
	st.entries = make([]entry, newSize)
	st.metadata = make([]metadata, newGroupCount)
	st.groupCount = newGroupCount

	// Reset size as we'll reinsert everything
	st.size = 0

	// Initialize metadata bytes to empty
	for i := range st.metadata {
		for j := range st.metadata[i].bytes {
			st.metadata[i].bytes[j] = 0
		}
	}

	// Reinsert all existing entries
	for groupIdx, group := range oldMetadata {
		for byteIdx, h2 := range group.bytes {
			if h2 != 0 { // If not empty
				oldIdx := groupIdx*groupSize + byteIdx
				st.Put(oldEntries[oldIdx].key, oldEntries[oldIdx].value)
			}
		}
	}
}

// Put inserts or updates a key-value pair
func (st *SwissTable) Put(key, value any) {
	// Check if we need to resize
	if float64(st.size+1)/float64(len(st.entries)) > loadFactor {
		st.resize()
	}

	idx, found := st.findSlot(key)
	if idx == -1 {
		// Table is full after resize (shouldn't happen)
		panic("table is full")
	}

	if !found {
		st.size++
	}

	// Calculate group and byte index
	groupIdx := idx / groupSize
	byteIdx := idx % groupSize

	// Get H2 hash for the key
	_, h2 := st.hashKey(key)

	// Update entry and metadata
	st.entries[idx] = entry{key: key, value: value, h2Hash: h2}
	st.metadata[groupIdx].bytes[byteIdx] = h2
}

// Get retrieves a value by key
func (st *SwissTable) Get(key any) (any, bool) {
	idx, found := st.findSlot(key)
	if !found || idx == -1 {
		return nil, false
	}
	return st.entries[idx].value, true
}

// Delete removes a key-value pair
func (st *SwissTable) Delete(key any) bool {
	idx, found := st.findSlot(key)
	if !found || idx == -1 {
		return false
	}

	// Calculate group and byte index
	groupIdx := idx / groupSize
	byteIdx := idx % groupSize

	// Clear metadata and entry
	st.metadata[groupIdx].bytes[byteIdx] = 0
	st.entries[idx] = entry{}
	st.size--
	return true
}

// Size returns the number of elements in the table
func (st *SwissTable) Size() int {
	return st.size
}

// Visualize returns a pretty-printed string representation of the table
func (st *SwissTable) Visualize() string {
	var result strings.Builder

	// Print header
	result.WriteString("Swiss Table State\n")
	result.WriteString(strings.Repeat("=", 50) + "\n")

	// Print summary
	fmt.Fprintf(&result, "Size: %d\n", st.size)
	fmt.Fprintf(&result, "Groups: %d\n", st.groupCount)
	fmt.Fprintf(&result, "Total Slots: %d\n", len(st.entries))
	result.WriteString(strings.Repeat("-", 50) + "\n\n")

	// Print metadata groups
	result.WriteString("Metadata Groups (h2 hashes, 0 = empty):\n")
	for i := 0; i < st.groupCount; i++ {
		fmt.Fprintf(&result, "Group %2d: [", i)
		for j := 0; j < groupSize; j++ {
			if j > 0 {
				result.WriteString("|")
			}
			h2 := st.metadata[i].bytes[j]
			if h2 == 0 {
				fmt.Fprintf(&result, "%3s", "·")
			} else {
				fmt.Fprintf(&result, "%3d", h2)
			}
		}
		result.WriteString(" ]\n")
	}

	// Print entries
	result.WriteString("\nEntries:\n")
	result.WriteString("Index |  H2  | Key:Value\n")
	result.WriteString(strings.Repeat("-", 50) + "\n")

	for i := 0; i < len(st.entries); i++ {
		entry := st.entries[i]
		if entry.key != nil {
			fmt.Fprintf(&result, "%4d  | %4d | %v:%v\n", 
				i, entry.h2Hash, entry.key, entry.value)
		}
	}

	return result.String()
}
