package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	anystore "github.com/anyproto/any-store"
	"github.com/anyproto/any-store/query"
	"github.com/anyproto/any-sync/commonspace/headsync/headstorage"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"google.golang.org/protobuf/encoding/protowire"
)

type TreeInfo struct {
	SpaceID        string
	TreeID         string
	SmartBlockType string
	ChangeCount    int
	RecentChanges  int // Changes within time filter (if --since provided)
	LastModified   time.Time
	HasTimeFilter  bool
}

func runAnalyzer() error {
	ctx := context.Background()

	// Parse time filter if provided
	var afterTime time.Time
	if since != "" {
		duration, err := time.ParseDuration(since)
		if err != nil {
			return fmt.Errorf("invalid duration format for --since: %w (use format like 10m, 1h, 24h)", err)
		}
		afterTime = time.Now().Add(-duration)
	}

	// Verify root path exists
	if _, err := os.Stat(rootPath); err != nil {
		return fmt.Errorf("path does not exist: %s", rootPath)
	}

	// Analyze all spaces
	trees, err := analyzeSpaces(ctx, rootPath, afterTime, spaceID, objectID)
	if err != nil {
		return err
	}

	// Enrich with smartblockType from objectstore
	enrichWithSmartBlockType(ctx, trees, rootPath)

	// Sort by change count (descending)
	// If time filter is active, sort by recent changes; otherwise by total changes
	sort.Slice(trees, func(i, j int) bool {
		if len(trees) > 0 && trees[0].HasTimeFilter {
			return trees[i].RecentChanges > trees[j].RecentChanges
		}
		return trees[i].ChangeCount > trees[j].ChangeCount
	})

	// Limit to top N
	if topN > 0 && len(trees) > topN {
		trees = trees[:topN]
	}

	// Display results with interactive Bubble Tea UI
	return runInteractiveUI(ctx, trees, rootPath)
}

func analyzeSpaces(ctx context.Context, rootPath string, afterTime time.Time, filterSpaceID string, filterObjectID string) ([]TreeInfo, error) {
	var results []TreeInfo

	// Read all directories in the root path
	entries, err := os.ReadDir(rootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", rootPath, err)
	}

	fmt.Printf("Scanning databases in: %s\n", rootPath)
	fmt.Println(strings.Repeat("-", 80))

	spacesProcessed := 0
	spacesSkipped := 0

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		spaceIDFromDir := entry.Name()

		// Apply space ID filter if specified
		if filterSpaceID != "" && spaceIDFromDir != filterSpaceID {
			continue
		}

		dbPath := filepath.Join(rootPath, spaceIDFromDir, "store.db")

		// Check if database file exists
		if _, err := os.Stat(dbPath); err != nil {
			spacesSkipped++
			fmt.Printf("Skipping %s: database file not found at %s\n", spaceIDFromDir, dbPath)
			continue
		}

		// Try to open the database
		db, err := anystore.Open(ctx, dbPath, &anystore.Config{
			SQLiteConnectionOptions: map[string]string{"synchronous": "off"},
		})
		if err != nil {
			spacesSkipped++
			fmt.Printf("Skipping %s: failed to open database: %v\n", spaceIDFromDir, err)
			continue
		}

		// Process the space
		spaceResults, err := processSpace(ctx, db, spaceIDFromDir, afterTime, filterObjectID)
		db.Close()

		if err != nil {
			spacesSkipped++
			fmt.Printf("Error processing %s: %v\n", spaceIDFromDir, err)
			continue
		}

		results = append(results, spaceResults...)
		spacesProcessed++
		fmt.Printf("Processed space: %s (found %d trees)\n", spaceIDFromDir, len(spaceResults))

		// If we found the specific object, we can stop searching
		if filterObjectID != "" && len(spaceResults) > 0 {
			break
		}
	}

	fmt.Println(strings.Repeat("-", 80))
	fmt.Printf("Total: %d spaces processed, %d skipped, %d trees found\n\n", spacesProcessed, spacesSkipped, len(results))

	return results, nil
}

func processSpecificTree(ctx context.Context, db anystore.DB, spaceIDFromDir string, treeID string, afterTime time.Time) (*TreeInfo, error) {
	// Open changes collection
	changesColl, err := db.OpenCollection(ctx, objecttree.CollName)
	if err != nil {
		return nil, fmt.Errorf("failed to open changes collection: %w", err)
	}
	defer changesColl.Close()

	// Check if this tree exists and count total changes
	totalCount, err := changesColl.Find(query.Key{
		Path:   []string{objecttree.TreeKey},
		Filter: query.NewComp(query.CompOpEq, treeID),
	}).Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count total changes: %w", err)
	}

	// If no changes found, the object doesn't exist in this space
	if totalCount == 0 {
		return nil, nil
	}

	// Count recent changes if time filter is active
	var recentCount int
	hasTimeFilter := !afterTime.IsZero()
	if hasTimeFilter {
		recentCount, err = changesColl.Find(query.And{
			query.Key{Path: []string{objecttree.TreeKey}, Filter: query.NewComp(query.CompOpEq, treeID)},
			query.Key{Path: []string{"a"}, Filter: query.NewComp(query.CompOpGte, float64(afterTime.Unix()))},
		}).Count(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to count recent changes: %w", err)
		}

		// Skip this tree if no recent changes when time filter is active
		if recentCount == 0 {
			return nil, nil
		}
	}

	// Get last modified time
	qry := changesColl.Find(query.Key{
		Path:   []string{objecttree.TreeKey},
		Filter: query.NewComp(query.CompOpEq, treeID),
	}).Sort("-a").Limit(1) // Sort by addedKey descending

	iter, err := qry.Iter(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query last change: %w", err)
	}
	defer iter.Close()

	var lastModified time.Time
	if iter.Next() {
		doc, err := iter.Doc()
		if err == nil {
			timestamp := doc.Value().GetFloat64("a") // addedKey
			lastModified = time.Unix(int64(timestamp), 0)
		}
	}

	return &TreeInfo{
		SpaceID:       spaceIDFromDir,
		TreeID:        treeID,
		ChangeCount:   totalCount,
		RecentChanges: recentCount,
		LastModified:  lastModified,
		HasTimeFilter: hasTimeFilter,
	}, nil
}

func processSpace(ctx context.Context, db anystore.DB, spaceIDFromDir string, afterTime time.Time, filterObjectID string) ([]TreeInfo, error) {
	var results []TreeInfo

	// Create space storage
	spaceStorage, err := spacestorage.New(ctx, spaceIDFromDir, db)
	if err != nil {
		return nil, fmt.Errorf("failed to create space storage: %w", err)
	}

	// Get head storage
	headStorage := spaceStorage.HeadStorage()

	// If objectID is specified, query directly for that object instead of iterating all
	if filterObjectID != "" {
		treeInfo, err := processSpecificTree(ctx, db, spaceIDFromDir, filterObjectID, afterTime)
		if err != nil {
			// Object not found in this space is not an error, just return empty results
			return results, nil
		}
		if treeInfo != nil {
			results = append(results, *treeInfo)
		}
		return results, nil
	}

	// Iterate through all trees
	err = headStorage.IterateEntries(ctx, headstorage.IterOpts{
		Deleted: false, // Skip deleted trees
	}, func(entry headstorage.HeadsEntry) (bool, error) {
		treeID := entry.Id

		// Open changes collection
		changesColl, err := db.OpenCollection(ctx, objecttree.CollName)
		if err != nil {
			return false, fmt.Errorf("failed to open changes collection: %w", err)
		}
		defer changesColl.Close()

		// Count total changes for this tree
		totalCount, err := changesColl.Find(query.Key{
			Path:   []string{objecttree.TreeKey},
			Filter: query.NewComp(query.CompOpEq, treeID),
		}).Count(ctx)
		if err != nil {
			return false, fmt.Errorf("failed to count total changes: %w", err)
		}

		// Count recent changes if time filter is active
		var recentCount int
		hasTimeFilter := !afterTime.IsZero()
		if hasTimeFilter {
			recentCount, err = changesColl.Find(query.And{
				query.Key{Path: []string{objecttree.TreeKey}, Filter: query.NewComp(query.CompOpEq, treeID)},
				query.Key{Path: []string{"a"}, Filter: query.NewComp(query.CompOpGte, float64(afterTime.Unix()))},
			}).Count(ctx)
			if err != nil {
				return false, fmt.Errorf("failed to count recent changes: %w", err)
			}

			// Skip trees with no recent changes when time filter is active
			if recentCount == 0 {
				return true, nil
			}
		}

		// Get last modified time
		qry := changesColl.Find(query.Key{
			Path:   []string{objecttree.TreeKey},
			Filter: query.NewComp(query.CompOpEq, treeID),
		}).Sort("-a").Limit(1) // Sort by addedKey descending

		iter, err := qry.Iter(ctx)
		if err != nil {
			return false, fmt.Errorf("failed to query last change: %w", err)
		}
		defer iter.Close()

		var lastModified time.Time
		if iter.Next() {
			doc, err := iter.Doc()
			if err == nil {
				timestamp := doc.Value().GetFloat64("a") // addedKey
				lastModified = time.Unix(int64(timestamp), 0)
			}
		}

		results = append(results, TreeInfo{
			SpaceID:       spaceIDFromDir,
			TreeID:        treeID,
			ChangeCount:   totalCount,
			RecentChanges: recentCount,
			LastModified:  lastModified,
			HasTimeFilter: hasTimeFilter,
		})

		return true, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to iterate trees: %w", err)
	}

	return results, nil
}

// smartBlockTypeNames maps protobuf enum values to human-readable names.
var smartBlockTypeNames = map[int]string{
	0:   "AccountOld",
	16:  "Page",
	17:  "ProfilePage",
	32:  "Home",
	48:  "Archive",
	112: "Widget",
	256: "File",
	288: "Template",
	289: "BundledTpl",
	512: "BundledRel",
	513: "SubObject",
	514: "BundledOT",
	515: "Profile",
	516: "Date",
	518: "Workspace",
	519: "Missing",
	521: "Relation",
	528: "Type",
	529: "RelOption",
	530: "SpaceView",
	532: "Identity",
	533: "FileObj",
	534: "Participant",
	535: "Notif",
	536: "Devices",
	537: "Chat",
	544: "ChatDerived",
	545: "Account",
}

func enrichWithSmartBlockType(ctx context.Context, trees []TreeInfo, rootPath string) {
	// Group trees by space
	bySpace := make(map[string][]int)
	for i := range trees {
		bySpace[trees[i].SpaceID] = append(bySpace[trees[i].SpaceID], i)
	}

	for spaceID, indices := range bySpace {
		dbPath := filepath.Join(rootPath, spaceID, "store.db")
		db, err := anystore.Open(ctx, dbPath, &anystore.Config{
			SQLiteConnectionOptions: map[string]string{"synchronous": "off"},
		})
		if err != nil {
			continue
		}

		coll, err := db.OpenCollection(ctx, objecttree.CollName)
		if err != nil {
			db.Close()
			continue
		}

		for _, idx := range indices {
			sbt := resolveSmartBlockType(ctx, coll, trees[idx].TreeID)
			if sbt != "" {
				trees[idx].SmartBlockType = sbt
			}
		}

		coll.Close()
		db.Close()
	}
}

// resolveSmartBlockType reads the root change for a tree and extracts the smartblock type
// from the protobuf chain: RawTreeChange → RootChange → ChangePayload (ObjectChangePayload field 1).
func resolveSmartBlockType(ctx context.Context, coll anystore.Collection, treeID string) string {
	doc, err := coll.FindId(ctx, treeID)
	if err != nil {
		return ""
	}

	rawBytes := doc.Value().GetBytes("r")
	if len(rawBytes) == 0 {
		return ""
	}

	// Unmarshal RawTreeChange to get Payload
	raw := &treechangeproto.RawTreeChange{}
	if err := raw.UnmarshalVT(rawBytes); err != nil {
		return ""
	}

	// Unmarshal RootChange to get ChangeType and ChangePayload
	root := &treechangeproto.RootChange{}
	if err := root.UnmarshalVT(raw.Payload); err != nil {
		return ""
	}

	if root.ChangeType != "anytype.object" {
		return root.ChangeType
	}

	// Parse ObjectChangePayload field 1 (SmartBlockType varint) manually
	// to avoid importing anytype-heart
	sbt := parseSmartBlockTypeFromPayload(root.ChangePayload)
	if name, ok := smartBlockTypeNames[sbt]; ok {
		return name
	}
	if sbt != 0 {
		return fmt.Sprintf("%d", sbt)
	}
	return ""
}

// parseSmartBlockTypeFromPayload extracts field 1 (varint) from the ObjectChangePayload protobuf.
func parseSmartBlockTypeFromPayload(b []byte) int {
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return 0
		}
		b = b[n:]
		if num == 1 && typ == protowire.VarintType {
			v, vn := protowire.ConsumeVarint(b)
			if vn < 0 {
				return 0
			}
			return int(v)
		}
		// Skip this field
		n = protowire.ConsumeFieldValue(num, typ, b)
		if n < 0 {
			return 0
		}
		b = b[n:]
	}
	return 0
}

func truncateID(id string, maxLen int) string {
	if len(id) <= maxLen {
		return id
	}
	if maxLen <= 3 {
		return id[:maxLen]
	}
	return id[:maxLen-3] + "..."
}

func formatTimestamp(t time.Time) string {
	if t.IsZero() {
		return "N/A"
	}

	// Show relative time for recent changes
	duration := time.Since(t)
	if duration < time.Hour {
		return fmt.Sprintf("%dm ago", int(duration.Minutes()))
	} else if duration < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(duration.Hours()))
	} else if duration < 7*24*time.Hour {
		return fmt.Sprintf("%dd ago", int(duration.Hours()/24))
	}

	// For older changes, show the actual date
	return t.Format("2006-01-02 15:04")
}
