package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func handleGC(parts []string, ctx EngineContext) {
	if len(parts) < 2 {
		printGCUsage()
		return
	}
	// Extract optional -rm flag from the end
	removeCache := false
	if parts[len(parts)-1] == "-rm" {
		removeCache = true
		parts = parts[:len(parts)-1]
	}

	if len(parts) < 2 {
		fmt.Println("Invalid command. Try 'gc all -rm', 'gc old -rm', or 'gc <hash> -rm'")
		return
	}

	switch parts[1] {
	case "all":
		gcAll(ctx, removeCache)
	case "old":
		gcOld(parts, ctx, removeCache)
	default:
		gcByHash(parts[1], ctx, removeCache)
	}
}

func gcAll(ctx EngineContext, removeCache bool) {
	if err := ctx.EngineLedger.ClearLedgerOnly(); err != nil {
		fmt.Printf("Failed to clear ledger: %v\n", err)
		return
	}
	fmt.Print("Ledger history completely wiped.")

	if removeCache {
		ctx.EngineLedger.ClearCacheMemory()
		freed := deletePhysicalFiles(ctx.TempDir, "", time.Time{})
		fmt.Printf(" Internal cache reset and %d MB of physical data deleted.\n", freed)
	} else {
		fmt.Println()
	}
}

func gcOld(parts []string, ctx EngineContext, removeCache bool) {
	days := 7
	if len(parts) >= 3 {
		parsedDays, err := strconv.Atoi(parts[2])
		if err != nil || parsedDays < 0 {
			fmt.Println("Invalid timeline. Please provide a positive number of days.")
			return
		}
		days = parsedDays
	}

	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	count, err := ctx.EngineLedger.PruneHistoryOlderThan(cutoff)
	if err != nil {
		fmt.Printf("Failed to prune ledger: %v\n", err)
		return
	}
	fmt.Printf("Removed %d entries older than %d days.", count, days)

	if removeCache {
		freed := deletePhysicalFiles(ctx.TempDir, "", cutoff)
		fmt.Printf(" Deleted %d MB of old physical data.\n", freed)
	} else {
		fmt.Println()
	}
}

func gcByHash(targetHash string, ctx EngineContext, removeCache bool) {
	if err := ctx.EngineLedger.DeleteCommit(targetHash); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("Commit '%s' successfully deleted.", targetHash)

	if removeCache {
		freed := deletePhysicalFiles(ctx.TempDir, targetHash, time.Time{})
		fmt.Printf(" Deleted %d MB of associated physical data.\n", freed)
	} else {
		fmt.Println()
	}
}

// removes .tar files from dir matching the given hash filter
func deletePhysicalFiles(dir, hashFilter string, cutoff time.Time) int64 {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}

	var freedBytes int64
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".tar") {
			continue
		}
		if hashFilter != "" && !strings.Contains(entry.Name(), hashFilter) {
			continue
		}

		filePath := filepath.Join(dir, entry.Name())
		info, err := os.Stat(filePath)
		if err != nil {
			continue
		}
		if !cutoff.IsZero() && info.ModTime().After(cutoff) {
			continue
		}

		freedBytes += info.Size()
		os.RemoveAll(filePath)
	}

	return freedBytes / 1024 / 1024
}

func printGCUsage() {
	fmt.Println(" Usage: gc <all|old|short_hash> [-rm]")
	fmt.Println("   all            : Wipes the visual transfer history")
	fmt.Println("   old [days]     : Removes history older than 7 days (or specified days)")
	fmt.Println("   <short_hash>   : Removes a specific commit by its short hash")
	fmt.Println("   [-rm]          : Add this flag to the end to also delete physical files")
}
