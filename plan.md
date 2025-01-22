# Headers-Only Freezer Implementation Plan

## Context

We need to implement a separate freezer instance for blockchain headers to enable pruning of historical data while maintaining headers. This is similar to how the state freezer works, but specifically for headers.

## Current Architecture

The chain freezer (ChainFreezerName) currently stores multiple tables:

- headers: Block headers
- hashes: Canonical hashes
- bodies: Block bodies
- receipts: Transaction receipts
- diffs: Total difficulties

Each freezer table consists of:

- Index files (.cidx/.ridx)
- Metadata file (.meta)
- Data files (.{N}.cdat/.rdat where N = 0000, 0001, etc.)

## Implementation Plan

### Create New Freezer Type

Create a new freezer type similar to state freezer, with its own directory and tables. This will store only headers and hashes, allowing us to maintain blockchain history while pruning other data.

### Integrate Header Freezer

The implementation will focus on abstracting the header freezer changes at the chainFreezer level to minimize downstream impacts:

1. Modify chainFreezer to handle separate header storage:
   - Add header-specific freezer instance
   - Route header/hash requests to appropriate freezer through wrapper
   - Maintain consistent interface for downstream code

### Implement Header Migration Function

Create a function to move headers from chain freezer to headers-only freezer:

1. Create new freezer instance for headers
2. Move files at filesystem level for efficiency
3. Maintain metadata consistency
4. Verify data integrity

Important Notes:
- Migration occurs while the node is offline during the pruneHistory command
- Headers will be completely moved (not copied) from the chain freezer to the new headers-only freezer
- The freezer should not be started until after migration is complete

### Migration Process Safety

1. Check if headers/ directory exists - fail if it does
2. Copy all header-related files to new location
3. Validate copied files for integrity/completeness
4. Only after validation passes, delete original files
5. If interrupted:
   - Process will fail on restart if headers/ directory exists
   - User must manually remove headers/ directory
   - Migration can be restarted from beginning

### Key Implementation Details

1. Chunked Processing: Process headers in chunks (e.g., 100k blocks) to avoid memory spikes
2. File-Level Operations: Use direct file operations where possible for efficiency
3. Atomic Updates: Ensure operations are atomic to prevent data corruption
4. Validation: Verify data integrity after migration
5. Metadata Handling: Carefully manage metadata files to maintain consistency

### Safety Considerations

1. Backup: Recommend users backup before migration
2. Validation: Implement thorough validation of migrated data
3. Rollback: Provide rollback mechanism in case of failure
4. Progress Tracking: Add progress indicators for long migrations
5. Clear error messages instructing users how to recover from interruption

### Testing Strategy

1. Create test cases with various chunk sizes
2. Test interrupted migrations
3. Verify data integrity post-migration
4. Test with different database sizes
5. Benchmark performance with different approaches
6. Test recovery from interrupted migrations

## Technical Notes

### File Structure

```
.
├── chain/
│ ├── headers.0000.cdat
│ ├── headers.cidx
│ ├── headers.meta
│ └── ...
└── headers/ (new)
├── headers.0000.cdat
├── headers.cidx
├── headers.meta
└── ...
```

### Key Files to Modify

1. core/rawdb/ancient_scheme.go: Add new freezer type
2. core/rawdb/freezer.go: Migration logic
3. core/rawdb/freezer_meta.go: Metadata handling
4. core/rawdb/accessors_chain.go: Update header access patterns
5. core/rawdb/database.go: Handle new freezer initialization

### Dependencies

- File locking (flock)
- Metadata management
- Freezer table operations

## Next Steps

[ ] Create detailed technical specification
[ ] Implement proof of concept
[ ] Add tests
[ ] Benchmark different approaches
[ ] Add migration command to geth
[ ] Document upgrade process

## Open Questions
1. What changes will be needed to header data access patterns in Geth?

### Implementation Strategy

The implementation will focus on abstracting the header freezer changes at the chainFreezer level to minimize downstream impacts:

1. Modify chainFreezer to handle separate header storage:
   - Add header-specific freezer instance
   - Route header/hash requests to appropriate freezer
   - Maintain consistent interface for downstream code

2. Keep existing interfaces intact:
   - No changes to HeaderChain access patterns
   - No changes to BlockChain access patterns
   - Maintain backward compatibility

3. Handle migration during pruning:
   - Move headers to separate freezer during prune operation
   - Validate consistency between freezers
   - Ensure atomic operations during migration

This approach allows us to:
- Minimize changes to downstream code
- Keep the complexity contained within chainFreezer
- Maintain existing access patterns and interfaces
- Enable separate pruning of non-header data