package swisstable

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

type operation int

const (
	opPut operation = iota
	opGet
	opDelete
)

func TestVisualize(t *testing.T) {
	st := New()
	
	// Insert some values
	st.Put(1, "one")
	st.Put(2, "two")
	st.Put(3, "three")
	
	// Print the visualization
	fmt.Printf("\nTable after inserting 1, 2, 3:\n%s\n", st.Visualize())
	
	// Delete a value
	st.Delete(2)
	
	// Print the visualization again
	fmt.Printf("\nTable after deleting 2:\n%s\n", st.Visualize())
}

func TestSwissTableBasic(t *testing.T) {
	st := New()

	// Test simple put and get
	st.Put(1, "one")
	if val, ok := st.Get(1); !ok || val != "one" {
		t.Errorf("Expected (one, true), got (%v, %v)", val, ok)
	}

	// Test overwrite
	st.Put(1, "new-one")
	if val, ok := st.Get(1); !ok || val != "new-one" {
		t.Errorf("Expected (new-one, true), got (%v, %v)", val, ok)
	}

	// Test delete
	if !st.Delete(1) {
		t.Error("Delete should return true for existing key")
	}
	if val, ok := st.Get(1); ok {
		t.Errorf("Expected not found after delete, got (%v, %v)", val, ok)
	}
}

func TestSwissTableVsMap(t *testing.T) {
	testCases := []struct {
		name     string
		numOps   int
		keyRange int
		valRange int
	}{
		{"Small_Few_Ops", 100, 10, 10},      // Small range, few operations
		{"Medium_Many_Ops", 1000, 100, 100}, // Medium range, more operations
		{"Large_Collisions", 500, 20, 1000}, // Many collisions (few keys, many values)
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Initialize both maps
			st := New()
			gm := make(map[any]any)

			// Set up random number generator with fixed seed for reproducibility
			rnd := rand.New(rand.NewSource(1234))

			// Track operations for debugging
			var ops []string

			// Perform random operations
			for i := 0; i < tc.numOps; i++ {
				// Choose random operation
				op := operation(rnd.Intn(3))

				// Generate random key and value
				key := rnd.Intn(tc.keyRange)
				value := rnd.Intn(tc.valRange)

				fmt.Printf("\nOperation %d:\n", i+1)
				fmt.Printf("Current state before op:\n")
				fmt.Printf("SwissTable: %v\n", mapToString(st))
				fmt.Printf("Go Map: %v\n", gm)

				switch op {
				case opPut:
					fmt.Printf("PUT: key=%v, value=%v\n", key, value)
					st.Put(key, value)
					gm[key] = value
					fmt.Printf("✅ Put completed\n")

				case opGet:
					stVal, stOk := st.Get(key)
					gmVal, gmOk := gm[key]
					fmt.Printf("GET: key=%v\n", key)
					fmt.Printf("SwissTable result: (value=%v, found=%v)\n", stVal, stOk)
					fmt.Printf("Go Map result: (value=%v, found=%v)\n", gmVal, gmOk)

					if stOk != gmOk {
						t.Errorf("Inconsistent existence for key %v after operations:\n%s",
							key, formatOps(ops))
					}
					if stOk && gmOk && stVal != gmVal {
						t.Errorf("Different values for key %v: swiss=%v, map=%v after operations:\n%s",
							key, stVal, gmVal, formatOps(ops))
					}
					ops = append(ops, fmt.Sprintf("Get(%v) = (%v, %v)", key, stVal, stOk))

				case opDelete:
					fmt.Printf("DELETE: key=%v\n", key)
					stDeleted := st.Delete(key)
					_, gmExists := gm[key]
					fmt.Printf("SwissTable deleted=%v, Go Map existed=%v\n", stDeleted, gmExists)
					delete(gm, key)

					if stDeleted != gmExists {
						t.Errorf("❌ Inconsistent deletion for key %v", key)
					} else {
						fmt.Printf("✅ Results match\n")
					}
					ops = append(ops, fmt.Sprintf("Delete(%v) = %v", key, stDeleted))
				}

				fmt.Printf("\nSize check: SwissTable=%d, Go Map=%d\n", st.Size(), len(gm))
				if st.Size() != len(gm) {
					t.Errorf("❌ Size mismatch")
				} else {
					fmt.Printf("✅ Sizes match\n")
				}

				fmt.Printf("\nVerifying all keys...\n")
				allMatch := true
				for k, gmVal := range gm {
					stVal, ok := st.Get(k)
					if !ok {
						t.Errorf("❌ Key %v missing from swiss table", k)
						allMatch = false
					} else if stVal != gmVal {
						t.Errorf("❌ Value mismatch for key %v: swiss=%v, map=%v",
							k, stVal, gmVal)
						allMatch = false
					}
				}
				if allMatch {
					fmt.Printf("✅ All keys and values match\n")
				}
				fmt.Printf("\n" + strings.Repeat("-", 50) + "\n")
			}
		})
	}
}

// Helper to format operations for error messages
func formatOps(ops []string) string {
	if len(ops) > 20 {
		// Only show last 20 operations to keep error messages readable
		ops = ops[len(ops)-20:]
		return fmt.Sprintf("Last 20 operations:\n%s", formatOpList(ops))
	}
	return fmt.Sprintf("Operations:\n%s", formatOpList(ops))
}

func formatOpList(ops []string) string {
	result := ""
	for i, op := range ops {
		result += fmt.Sprintf("%d: %s\n", i+1, op)
	}
	return result
}

// Helper to convert SwissTable to string for printing
func mapToString(st *SwissTable) string {
	result := "{"
	first := true
	for i := 0; i < len(st.entries); i++ {
		if st.entries[i].key != nil {
			if !first {
				result += ", "
			}
			result += fmt.Sprintf("%v:%v", st.entries[i].key, st.entries[i].value)
			first = false
		}
	}
	return result + "}"
}
