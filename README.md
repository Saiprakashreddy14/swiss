# Swiss Table Implementation in Go

This is a basic implementation of a Swiss Table data structure in Go. Swiss Tables are a high-performance hash table implementation that uses less memory than traditional hash tables while maintaining good performance characteristics.

## Features

- Open addressing with linear probing
- Metadata for control bytes
- Dynamic resizing
- Generic key-value storage
- Basic operations: Put, Get, Delete
- Efficient space usage

## Core Concepts  

1. **Metadata**: Each slot in the table has an associated metadata byte that indicates its state (empty, deleted, or filled).

2. **Linear Probing**: When collisions occur, the implementation uses linear probing to find the next available slot.

3. **Load Factor**: The table automatically resizes when the load factor exceeds 0.75 to maintain performance.

4. **Generic Storage**: The implementation supports any type of key-value pairs using Go's interface{}.

## Usage

```go
// Create a new Swiss Table
st := swisstable.New()

// Insert key-value pairs
st.Put("key1", "value1")
st.Put("key2", "value2")

// Retrieve values
value, exists := st.Get("key1")

// Delete entries
deleted := st.Delete("key1")

// Get size
size := st.Size()
```

## Running Tests

To run the tests:

```bash
go test -v
```

## Implementation Details

- Initial size: 16 slots
- Load factor threshold: 0.75
- Resizing strategy: Double the size when load factor is exceeded
- Hash function: Uses Go's maphash package for high-quality hashing

## Performance Characteristics

- Average case time complexity:
  - Insert: O(1)
  - Lookup: O(1)
  - Delete: O(1)
- Space complexity: O(n) where n is the number of elements

