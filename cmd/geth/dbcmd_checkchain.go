// Copyright 2026 The go-ethereum Authors
// This file is part of go-ethereum.
//
// go-ethereum is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-ethereum is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-ethereum. If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/urfave/cli/v2"
)

var (
	checkChainJSONFlag = &cli.BoolFlag{
		Name:  "json",
		Usage: "Emit findings as JSON Lines on stdout (one object per line)",
	}

	dbCheckChainCmd = &cli.Command{
		Action:    dbCheckChain,
		Name:      "check-chain",
		ArgsUsage: "",
		Flags: slices.Concat([]cli.Flag{
			checkChainJSONFlag,
		}, utils.NetworkFlags, utils.DatabaseFlags),
		Usage: "Report chain database health and any indications of corruption",
		Description: `This command opens the chain database read-only and reports:

  - Node configuration (state scheme, history mode, tx-index range, log-index range,
    chain id, fork timings, database version).
  - Indications of unclean shutdowns recorded in the database.
  - Head pointer resolution (LastHeader, LastBlock, LastFast, LastFinalized).

The command does not modify the database. Exit code is 0 if no findings, 1 if any
warnings were reported, and 2 if any errors were reported.`,
	}
)

// severity is the severity level of a finding.
type severity string

const (
	sevInfo  severity = "info"
	sevWarn  severity = "warn"
	sevError severity = "error"
)

// finding describes a single observation made by check-chain. The shape is
// stable and used for both human and JSON output.
type finding struct {
	Severity severity       `json:"severity"`
	Section  string         `json:"section"`           // e.g. "config", "unclean_shutdown"
	Type     string         `json:"type"`              // machine-readable subtype
	Subject  string         `json:"subject,omitempty"` // short human label
	Detail   string         `json:"detail,omitempty"`  // longer human prose
	Data     map[string]any `json:"data,omitempty"`    // structured payload
}

// reporter collects findings and emits them in the chosen format.
type reporter struct {
	out      io.Writer
	jsonMode bool
	findings []finding
}

func newReporter(out io.Writer, jsonMode bool) *reporter {
	return &reporter{out: out, jsonMode: jsonMode}
}

// add records a finding and prints it immediately. In human mode, info findings
// are printed plainly so they double as the config report.
func (r *reporter) add(f finding) {
	r.findings = append(r.findings, f)
	if r.jsonMode {
		enc, _ := json.Marshal(f)
		fmt.Fprintln(r.out, string(enc))
		return
	}
	// Human mode formatting.
	tag := map[severity]string{
		sevInfo:  "  ",
		sevWarn:  "WARN",
		sevError: "ERR ",
	}[f.Severity]
	if f.Subject != "" {
		fmt.Fprintf(r.out, "%s %-22s %s\n", tag, f.Section+"/"+f.Type, f.Subject)
	} else {
		fmt.Fprintf(r.out, "%s %s\n", tag, f.Section+"/"+f.Type)
	}
	if f.Detail != "" {
		fmt.Fprintf(r.out, "       %s\n", f.Detail)
	}
}

// section prints a section header in human mode (no-op in JSON mode).
func (r *reporter) section(name string) {
	if r.jsonMode {
		return
	}
	fmt.Fprintf(r.out, "\n== %s ==\n", name)
}

// exitCode returns 2 if any error finding was recorded, 1 if any warning, else 0.
func (r *reporter) exitCode() int {
	code := 0
	for _, f := range r.findings {
		switch f.Severity {
		case sevError:
			return 2
		case sevWarn:
			code = 1
		}
	}
	return code
}

// dbCheckChain is the entry point for the `geth db check-chain` command.
func dbCheckChain(ctx *cli.Context) error {
	stack, cfg := makeConfigNode(ctx)
	defer stack.Close()

	db := utils.MakeChainDatabase(ctx, stack, true)
	defer db.Close()

	rep := newReporter(os.Stdout, ctx.Bool(checkChainJSONFlag.Name))

	rep.section("Node configuration")
	reportConfig(db, &cfg, rep)

	rep.section("Unclean shutdown indicators")
	checkUncleanShutdown(db, rep)

	rep.section("Head pointers")
	checkHeadPointers(db, rep)

	rep.section("Freezer / KV boundary")
	checkFreezerBoundary(db, rep)

	if !rep.jsonMode {
		fmt.Fprintln(os.Stdout)
		switch rep.exitCode() {
		case 0:
			fmt.Fprintln(os.Stdout, "No issues found.")
		case 1:
			fmt.Fprintln(os.Stdout, "Completed with warnings.")
		case 2:
			fmt.Fprintln(os.Stdout, "Completed with errors.")
		}
	}
	if code := rep.exitCode(); code != 0 {
		// urfave/cli treats a returned cli.Exit as the process exit code.
		return cli.Exit("", code)
	}
	return nil
}

// reportConfig emits info findings describing how this node is configured.
// Some settings come from the database (state scheme, chain id, database
// version); others come from the resolved geth config (gcmode, history mode,
// state-history retention) since those are runtime-only and not persisted.
func reportConfig(db ethdb.Database, cfg *gethConfig, rep *reporter) {
	// Database version.
	if v := rawdb.ReadDatabaseVersion(db); v != nil {
		rep.add(finding{
			Severity: sevInfo, Section: "config", Type: "database_version",
			Subject: fmt.Sprintf("%d", *v),
		})
	}

	// State scheme: path vs hash. Resolved by reading the on-disk state;
	// blank when no state is on disk yet.
	scheme := rawdb.ReadStateScheme(db)
	if scheme == "" {
		scheme = "none on disk"
	}
	schemeSubject := scheme
	if scheme == rawdb.PathScheme {
		// For path scheme the persistent state id is a monotonically
		// increasing counter set when state is flushed to disk; surface it
		// alongside the scheme so operators can tell at a glance whether
		// state has actually been persisted.
		if id := rawdb.ReadPersistentStateID(db); id != 0 {
			schemeSubject = fmt.Sprintf("path  (persistent state id %d)", id)
		}
	}
	rep.add(finding{
		Severity: sevInfo, Section: "config", Type: "state_scheme",
		Subject: schemeSubject,
		Detail:  "set on first init; cannot be changed without resync",
	})

	// GC mode (archive vs full) from the resolved eth config. The runtime
	// flag --gcmode=archive sets cfg.Eth.NoPruning=true. When neither flag
	// nor config file specifies, the default is full.
	gcmode := "full"
	if cfg.Eth.NoPruning {
		gcmode = "archive"
	}
	rep.add(finding{
		Severity: sevInfo, Section: "config", Type: "gcmode",
		Subject: gcmode,
		Detail:  "from resolved eth config (--gcmode flag or TOML config)",
	})

	// State history retention (path scheme only).
	if scheme == rawdb.PathScheme {
		rep.add(finding{
			Severity: sevInfo, Section: "config", Type: "state_history",
			Subject: fmt.Sprintf("%d blocks", cfg.Eth.StateHistory),
			Detail:  "path-scheme: maximum blocks from head whose state histories are reserved",
		})
		if cfg.Eth.TrienodeHistory >= 0 {
			rep.add(finding{
				Severity: sevInfo, Section: "config", Type: "trienode_history",
				Subject: fmt.Sprintf("%d blocks", cfg.Eth.TrienodeHistory),
				Detail:  "path-scheme: blocks from head for which trienode histories are retained",
			})
		}
	}

	// Chain history retention mode (keep all / keep post-merge / keep post-prague).
	rep.add(finding{
		Severity: sevInfo, Section: "config", Type: "history_mode",
		Subject: cfg.Eth.HistoryMode.String(),
		Detail:  "from resolved eth config (--history.chain flag or TOML config)",
	})

	// Chain config (chain id + fork schedule).
	if genesisHash := rawdb.ReadCanonicalHash(db, 0); genesisHash != (common.Hash{}) {
		if chainCfg := rawdb.ReadChainConfig(db, genesisHash); chainCfg != nil {
			rep.add(finding{
				Severity: sevInfo, Section: "config", Type: "chain_id",
				Subject: chainCfg.ChainID.String(),
			})
		}
	}

	// Snap-sync status flag - reported here because it's a config-shaped
	// observation (where in the snap-sync lifecycle is this node).
	switch rawdb.ReadSnapSyncStatusFlag(db) {
	case rawdb.StateSyncRunning:
		rep.add(finding{
			Severity: sevWarn, Section: "config", Type: "snap_sync_status",
			Subject: "running",
			Detail:  "state snap sync was in progress at last shutdown; will resume on next start",
		})
	case rawdb.StateSyncFinished:
		rep.add(finding{
			Severity: sevInfo, Section: "config", Type: "snap_sync_status",
			Subject: "finished",
		})
	}
}

// checkUncleanShutdown reports any unclean shutdowns recorded in the DB and
// flags leftover snap-sync markers that suggest a sync was interrupted.
func checkUncleanShutdown(db ethdb.Database, rep *reporter) {
	// The unclean-shutdown marker is a list of timestamps recorded each time
	// geth starts up; the most recent one is removed on a clean shutdown.
	// Anything remaining is therefore a prior crash / kill / power loss.
	if raw, err := db.Get([]byte("unclean-shutdown")); err == nil && len(raw) > 0 {
		var list crashList
		if err := rlp.DecodeBytes(raw, &list); err != nil {
			rep.add(finding{
				Severity: sevWarn, Section: "unclean_shutdown", Type: "marker_decode_failed",
				Subject: "could not decode unclean-shutdown marker",
				Detail:  err.Error(),
			})
		} else if len(list.Recent) > 0 {
			times := make([]string, 0, len(list.Recent))
			for _, ts := range list.Recent {
				times = append(times, time.Unix(int64(ts), 0).UTC().Format(time.RFC3339))
			}
			sev := sevWarn
			subject := fmt.Sprintf("%d unclean shutdown(s) recorded", len(list.Recent))
			if list.Discarded > 0 {
				subject += fmt.Sprintf("; %d older entries discarded", list.Discarded)
				sev = sevError
			}
			rep.add(finding{
				Severity: sev, Section: "unclean_shutdown", Type: "marker_present",
				Subject: subject,
				Detail:  "most recent first: " + joinReverse(times),
				Data: map[string]any{
					"timestamps_unix":     list.Recent,
					"timestamps_iso":      times,
					"discarded":           list.Discarded,
					"most_recent_unix":    list.Recent[len(list.Recent)-1],
					"most_recent_iso8601": times[len(times)-1],
				},
			})
		}
	}

	// Last-pivot marker: if present, a snap-sync cycle recorded its pivot.
	// On a fully-completed snap-sync the marker is consulted on next start
	// but kept around; presence alone isn't a smoking gun. We still surface
	// it because it's the marker we'd want to cross-reference against the
	// chain head when investigating boundary issues.
	if p := rawdb.ReadLastPivotNumber(db); p != nil {
		rep.add(finding{
			Severity: sevInfo, Section: "unclean_shutdown", Type: "last_pivot",
			Subject: fmt.Sprintf("block %d", *p),
			Detail:  "last snap-sync pivot recorded",
		})
	}

	// Snapshot recovery flag: only present mid-rebuild.
	if n := rawdb.ReadSnapshotRecoveryNumber(db); n != nil {
		rep.add(finding{
			Severity: sevWarn, Section: "unclean_shutdown", Type: "snapshot_recovery",
			Subject: fmt.Sprintf("recovery in progress through block %d", *n),
			Detail:  "snapshot rebuild was running at last shutdown",
		})
	}

	// Snapshot disabled flag: should be cleared after snap-sync completes.
	if rawdb.ReadSnapshotDisabled(db) {
		rep.add(finding{
			Severity: sevWarn, Section: "unclean_shutdown", Type: "snapshot_disabled",
			Subject: "snapshot maintenance disabled",
			Detail:  "set during snap-sync; if no sync is in progress, this is leftover state",
		})
	}

	// Fast-trie-progress marker: snap-sync trie download progress. Should be
	// absent after snap-sync completes.
	if raw, err := db.Get([]byte("TrieSync")); err == nil && len(raw) > 0 {
		rep.add(finding{
			Severity: sevWarn, Section: "unclean_shutdown", Type: "fast_trie_progress",
			Subject: fmt.Sprintf("non-empty (%d bytes)", len(raw)),
			Detail:  "snap-sync trie progress recorded; if no sync is in progress, this is leftover state",
		})
	}

	// Skeleton sync status: more than one subchain indicates a previous beacon
	// sync didn't link cleanly. This is exactly the pattern we saw in the
	// real-world `Cleaning spurious beacon sync leftovers` incidents.
	//
	// Note the on-disk format is JSON, not RLP — see saveSyncStatus in
	// eth/downloader/skeleton.go.
	if raw := rawdb.ReadSkeletonSyncStatus(db); len(raw) > 0 {
		// Decode using a structure compatible with eth/downloader's
		// skeletonProgress without depending on that package.
		var prog skeletonProgressLite
		if err := json.Unmarshal(raw, &prog); err != nil {
			rep.add(finding{
				Severity: sevWarn, Section: "unclean_shutdown", Type: "skeleton_decode_failed",
				Subject: "could not decode skeleton sync status",
				Detail:  err.Error(),
			})
		} else {
			n := len(prog.Subchains)
			data := map[string]any{
				"subchain_count": n,
			}
			if prog.Finalized != nil {
				data["finalized"] = *prog.Finalized
			}
			if n > 1 {
				details := make([]string, 0, n)
				for i, c := range prog.Subchains {
					details = append(details, fmt.Sprintf(
						"#%d head=%d tail=%d next=%s",
						i, c.Head, c.Tail, c.Next.TerminalString()))
				}
				data["subchains"] = details
				rep.add(finding{
					Severity: sevWarn, Section: "unclean_shutdown", Type: "skeleton_multi_subchain",
					Subject: fmt.Sprintf("%d subchains present (expected 1)", n),
					Detail:  "leftover from previous beacon sync that did not link cleanly: " + joinReverse(details),
					Data:    data,
				})
			} else if n == 1 {
				c := prog.Subchains[0]
				rep.add(finding{
					Severity: sevInfo, Section: "unclean_shutdown", Type: "skeleton_state",
					Subject: fmt.Sprintf("1 subchain head=%d tail=%d", c.Head, c.Tail),
				})
			}
		}
	}
}

// checkHeadPointers verifies that each head-pointer key resolves to an existing
// block at a consistent height, and that the canonical hash at that height
// matches.
func checkHeadPointers(db ethdb.Database, rep *reporter) {
	headers := []struct {
		name string
		hash common.Hash
	}{
		{"LastHeader", rawdb.ReadHeadHeaderHash(db)},
		{"LastBlock", rawdb.ReadHeadBlockHash(db)},
		{"LastFast", rawdb.ReadHeadFastBlockHash(db)},
		{"LastFinalized", rawdb.ReadFinalizedBlockHash(db)},
	}

	var (
		headerNum, blockNum *uint64
	)
	for _, h := range headers {
		if h.hash == (common.Hash{}) {
			rep.add(finding{
				Severity: sevInfo, Section: "head_pointers", Type: "absent",
				Subject: h.name,
			})
			continue
		}
		num, ok := rawdb.ReadHeaderNumber(db, h.hash)
		if !ok {
			rep.add(finding{
				Severity: sevError, Section: "head_pointers", Type: "no_number_mapping",
				Subject: fmt.Sprintf("%s = %s", h.name, h.hash.TerminalString()),
				Detail:  "H<hash> -> number entry missing; head pointer is dangling",
			})
			continue
		}
		if rawdb.ReadHeader(db, h.hash, num) == nil {
			rep.add(finding{
				Severity: sevError, Section: "head_pointers", Type: "no_header",
				Subject: fmt.Sprintf("%s = %s at #%d", h.name, h.hash.TerminalString(), num),
				Detail:  "h<num><hash> -> header entry missing",
			})
			continue
		}
		canon := rawdb.ReadCanonicalHash(db, num)
		if canon != h.hash {
			detail := fmt.Sprintf("canonical hash at #%d is %s, expected %s",
				num, canon.TerminalString(), h.hash.TerminalString())
			rep.add(finding{
				Severity: sevError, Section: "head_pointers", Type: "canonical_mismatch",
				Subject: fmt.Sprintf("%s @ #%d", h.name, num),
				Detail:  detail,
			})
			continue
		}
		rep.add(finding{
			Severity: sevInfo, Section: "head_pointers", Type: "ok",
			Subject: fmt.Sprintf("%-13s #%d  %s", h.name, num, h.hash.TerminalString()),
		})
		// Remember a couple of numbers for cross-checks below.
		switch h.name {
		case "LastHeader":
			n := num
			headerNum = &n
		case "LastBlock":
			n := num
			blockNum = &n
		}
	}

	// Sanity: header height should be >= block height. A node that's past
	// snap-sync should have them equal.
	if headerNum != nil && blockNum != nil {
		if *blockNum > *headerNum {
			rep.add(finding{
				Severity: sevError, Section: "head_pointers", Type: "block_above_header",
				Subject: fmt.Sprintf("LastBlock #%d > LastHeader #%d", *blockNum, *headerNum),
				Detail:  "head full block ahead of head header (impossible in steady state)",
			})
		}
	}
}

// boundaryWindowAbove is how many blocks past `frozen` we check for
// consistency. The freezer iterates from `frozen` upward, so this catches the
// most common breakage spot. A full-range scan is the job of Phase 3 (--range).
const boundaryWindowAbove = 10

// checkFreezerBoundary inspects the join between the ancient store and the KV
// store: blocks at and just above `frozen` (the next-to-freeze block) must be
// fully present in the KV store, and blocks at `frozen-1` must be readable
// from every chain freezer table.
//
// This is the highest-value check in the suite — every reported instance of
// "canonical hash missing" / "block body missing" / "block receipts missing"
// in production logs has been a failure of one of these invariants.
func checkFreezerBoundary(db ethdb.Database, rep *reporter) {
	frozen, err := db.Ancients()
	if err != nil {
		// A KV-only database (no freezer attached) returns "this operation is
		// not supported" here. That isn't an error — boundary checks just
		// don't apply.
		if strings.Contains(err.Error(), "not supported") {
			rep.add(finding{
				Severity: sevInfo, Section: "freezer", Type: "absent",
				Subject: "no ancient store attached",
			})
			return
		}
		rep.add(finding{
			Severity: sevError, Section: "freezer", Type: "ancients_failed",
			Subject: err.Error(),
		})
		return
	}
	tail, err := db.Tail()
	if err != nil {
		rep.add(finding{
			Severity: sevError, Section: "freezer", Type: "tail_failed",
			Subject: err.Error(),
		})
		return
	}

	rep.add(finding{
		Severity: sevInfo, Section: "freezer", Type: "head",
		Subject: fmt.Sprintf("%d", frozen),
		Detail:  "next block to freeze; freezer reads start here",
	})
	rep.add(finding{
		Severity: sevInfo, Section: "freezer", Type: "tail",
		Subject: fmt.Sprintf("%d", tail),
		Detail:  "first block kept in prunable tables (bodies, receipts); non-prunable tables retain from genesis",
	})

	if frozen == 0 {
		rep.add(finding{
			Severity: sevInfo, Section: "freezer", Type: "empty",
			Subject: "no blocks frozen yet; boundary checks skipped",
		})
		return
	}

	// If the freezer has caught up to the chain head, there is no block at
	// `frozen` yet — the freezer is waiting for the chain to advance. The
	// KV-side boundary probe would false-positive in this case, so we skip
	// it. Determined by looking up the head header's number.
	headHash := rawdb.ReadHeadHeaderHash(db)
	headNum, headKnown := rawdb.ReadHeaderNumber(db, headHash)
	if headKnown && frozen > headNum {
		rep.add(finding{
			Severity: sevInfo, Section: "freezer", Type: "caught_up",
			Subject: fmt.Sprintf("frozen #%d > head #%d (freezer is current; KV-side boundary probe skipped)",
				frozen, headNum),
		})
		// Still verify the ancient-side invariants at frozen-1 before we exit.
		checkFreezerAncientHead(db, frozen, rep)
		return
	}

	// 1. Verify the four chain tables agree at the head boundary.
	checkFreezerAncientHead(db, frozen, rep)

	// 2. Verify the KV store has the components the freezer expects to read on
	//    its next iteration. The check below mirrors the read pattern used in
	//    core/rawdb/chain_freezer.go's freezeRange.
	hash := rawdb.ReadCanonicalHash(db, frozen)
	if hash == (common.Hash{}) {
		rep.add(finding{
			Severity: sevError, Section: "freezer", Type: "kv_canonical_hash_missing",
			Subject: fmt.Sprintf("block #%d", frozen),
			Detail: "next-to-freeze block has no canonical hash in KV — the freezer " +
				"will fail at the next freeze cycle with 'canonical hash missing'",
		})
		// Without the canonical hash we can't probe header/body/receipts here.
	} else {
		checkBlockComponentsAtBoundary(db, hash, frozen, rep)
	}

	// 3. Probe a small window above the boundary so we catch gaps that affect
	//    the freezer's next batch (not just the very next block).
	gapsFound := 0
	for offset := uint64(1); offset <= boundaryWindowAbove; offset++ {
		n := frozen + offset
		h := rawdb.ReadCanonicalHash(db, n)
		if h == (common.Hash{}) {
			// Chain may not have reached this height yet. Stop the scan rather
			// than emit a false-positive gap.
			break
		}
		if checkBlockComponentsAtBoundary(db, h, n, rep) {
			gapsFound++
		}
	}

	if gapsFound == 0 && hash != (common.Hash{}) {
		rep.add(finding{
			Severity: sevInfo, Section: "freezer", Type: "boundary_ok",
			Subject: fmt.Sprintf("KV components at #%d..#%d present and non-empty",
				frozen, frozen+boundaryWindowAbove),
		})
	}
}

// checkFreezerAncientHead verifies each of the four chain freezer tables has
// data at frozen-1 and is out-of-bounds at frozen. Callers must have already
// confirmed frozen > 0.
func checkFreezerAncientHead(db ethdb.Database, frozen uint64, rep *reporter) {
	tables := []string{
		rawdb.ChainFreezerHashTable,
		rawdb.ChainFreezerHeaderTable,
		rawdb.ChainFreezerBodiesTable,
		rawdb.ChainFreezerReceiptTable,
	}
	for _, kind := range tables {
		data, err := db.Ancient(kind, frozen-1)
		switch {
		case err != nil:
			rep.add(finding{
				Severity: sevError, Section: "freezer", Type: "table_underrun",
				Subject: fmt.Sprintf("%s @ #%d (head-1)", kind, frozen-1),
				Detail:  "read failed at the last-frozen position: " + err.Error(),
			})
		case len(data) == 0 && kind != rawdb.ChainFreezerBodiesTable && kind != rawdb.ChainFreezerReceiptTable:
			// An empty hash/header at head-1 is unexpected; empty body/receipts
			// is fine (empty block).
			rep.add(finding{
				Severity: sevError, Section: "freezer", Type: "table_empty_at_head",
				Subject: fmt.Sprintf("%s @ #%d (head-1)", kind, frozen-1),
				Detail:  "read returned zero bytes",
			})
		}
		if _, err := db.Ancient(kind, frozen); err == nil {
			rep.add(finding{
				Severity: sevError, Section: "freezer", Type: "table_overrun",
				Subject: fmt.Sprintf("%s has data at #%d (should be out-of-bounds)", kind, frozen),
			})
		}
	}
}

// checkBlockComponentsAtBoundary verifies header/body/receipts are non-empty
// in the KV store for the given (n, hash). Returns true if any error finding
// was emitted.
func checkBlockComponentsAtBoundary(db ethdb.Database, hash common.Hash, n uint64, rep *reporter) bool {
	bad := false
	if data := rawdb.ReadHeaderRLP(db, hash, n); len(data) == 0 {
		rep.add(finding{
			Severity: sevError, Section: "freezer", Type: "kv_header_missing",
			Subject: fmt.Sprintf("block #%d (%s)", n, hash.TerminalString()),
			Detail:  "header absent or stored as empty bytes",
		})
		bad = true
	}
	if data := rawdb.ReadBodyRLP(db, hash, n); len(data) == 0 {
		rep.add(finding{
			Severity: sevError, Section: "freezer", Type: "kv_body_missing",
			Subject: fmt.Sprintf("block #%d (%s)", n, hash.TerminalString()),
			Detail:  "body absent or stored as empty bytes",
		})
		bad = true
	}
	if data := rawdb.ReadReceiptsRLP(db, hash, n); len(data) == 0 {
		rep.add(finding{
			Severity: sevError, Section: "freezer", Type: "kv_receipts_missing",
			Subject: fmt.Sprintf("block #%d (%s)", n, hash.TerminalString()),
			Detail:  "receipts absent or stored as empty bytes (matches PR #31952 corruption pattern)",
		})
		bad = true
	}
	return bad
}

// joinReverse joins entries with "; " for human-readable output, reversed so
// the most recent comes first.
func joinReverse(items []string) string {
	if len(items) == 0 {
		return ""
	}
	rev := make([]string, len(items))
	for i, s := range items {
		rev[len(items)-1-i] = s
	}
	return joinSemi(rev)
}

func joinSemi(items []string) string {
	out := ""
	for i, s := range items {
		if i > 0 {
			out += "; "
		}
		out += s
	}
	return out
}

// crashList mirrors core/rawdb's internal crashList type so we can decode the
// unclean-shutdown marker without exporting it.
type crashList struct {
	Discarded uint64
	Recent    []uint64
}

// skeletonProgressLite mirrors eth/downloader's skeletonProgress for RLP
// decoding without taking a dependency on the downloader package.
type skeletonProgressLite struct {
	Subchains []*subchainLite
	Finalized *uint64
}

type subchainLite struct {
	Head uint64
	Tail uint64
	Next common.Hash
}

// Compile-time assertion that we always import what we declare.
var _ = errors.New
