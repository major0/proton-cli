package lumoCmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/major0/proton-cli/api/lumo"
	lumoClient "github.com/major0/proton-cli/api/lumo/client"
	"github.com/spf13/cobra"
)

var spaceCmd = &cobra.Command{
	Use:   "space",
	Short: "Manage Lumo spaces",
	Run: func(cmd *cobra.Command, _ []string) {
		_ = cmd.Help()
	},
}

func init() {
	AddCommand(spaceCmd)
	spaceCmd.AddCommand(spaceListCmd)
	spaceCmd.AddCommand(spaceDeleteCmd)
}

// --- space list ---

var spaceShowAll bool
var spaceShowEmpty bool
var spaceShowSimple bool

var spaceListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List spaces (projects only; -A all; --simple chats; --is-empty empty)",
	RunE:    runSpaceList,
}

func init() {
	spaceListCmd.Flags().BoolVarP(&spaceShowAll, "all", "A", false, "Show all spaces including simple chat spaces")
	spaceListCmd.Flags().BoolVar(&spaceShowEmpty, "is-empty", false, "Find and verify empty spaces")
	spaceListCmd.Flags().BoolVar(&spaceShowSimple, "simple", false, "Show simple chat spaces only")
}

func runSpaceList(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	client, err := restoreClient(cmd)
	if err != nil {
		return err
	}

	spaces, err := client.ListSpaces(ctx)
	if err != nil {
		return fmt.Errorf("listing spaces: %w", err)
	}

	if spaceShowEmpty {
		return runSpaceListEmpty(ctx, client, spaces)
	}

	// Filter by type: -A shows all, --simple shows simple, default shows projects.
	if !spaceShowAll {
		if spaceShowSimple {
			spaces = filterSimpleSpaces(ctx, client, spaces)
		} else {
			spaces = filterProjectSpaces(ctx, client, spaces)
		}
	}

	rows := buildSpaceRows(ctx, client, spaces)
	_, _ = fmt.Fprint(os.Stdout, FormatSpaceList(rows))
	return nil
}

// runSpaceListEmpty identifies, verifies, and lists empty spaces.
func runSpaceListEmpty(ctx context.Context, client *lumoClient.Client, spaces []lumo.Space) error {
	total := len(spaces)

	// Phase 1: identify candidates (0 embedded conversations).
	var candidates []lumo.Space
	for _, s := range spaces {
		if len(s.Conversations) == 0 {
			candidates = append(candidates, s)
		}
	}

	_, _ = fmt.Fprintf(os.Stderr, "Scanning %d spaces, %d candidates with 0 embedded conversations...\n", total, len(candidates))

	// Phase 2: verify each candidate and filter by type.
	type emptySpace struct {
		space     lumo.Space
		spaceType string // "simple" or "project" or "unencrypted"
	}
	var verified []emptySpace

	for _, s := range candidates {
		// Verify no assets either.
		if len(s.Assets) > 0 {
			return fmt.Errorf("space %s has 0 conversations but %d assets — not empty", s.ID, len(s.Assets))
		}

		stype := classifySpace(ctx, client, &s)

		// Filter by type flags: --simple shows simple only,
		// default (no -A, no --simple) shows projects only,
		// -A shows all.
		if !spaceShowAll {
			if spaceShowSimple && stype != "simple" {
				continue
			}
			if !spaceShowSimple && stype != "project" {
				continue
			}
		}

		verified = append(verified, emptySpace{space: s, spaceType: stype})
	}

	// Phase 3: list verified empty spaces.
	if len(verified) == 0 {
		_, _ = fmt.Fprintf(os.Stdout, "No empty spaces found (%d total).\n", total)
		return nil
	}

	// Sort newest first.
	sort.Slice(verified, func(i, j int) bool {
		return verified[i].space.CreateTime > verified[j].space.CreateTime
	})

	var b strings.Builder
	fmt.Fprintf(&b, "%-12s  %-20s  %s\n", "TYPE", "CREATED", "ID")
	for _, v := range verified {
		fmt.Fprintf(&b, "%-12s  %-20s  %s\n", v.spaceType, v.space.CreateTime, v.space.ID)
	}
	fmt.Fprintf(&b, "\n%d empty spaces found out of %d total.\n", len(verified), total)

	// Detailed breakdown for cross-checking against the webapp.
	nonEmpty := total - len(verified)
	simpleWithConvs := 0
	projectWithConvs := 0
	totalConvs := 0
	deletedConvs := 0
	simpleConvs := 0
	projectConvs := 0
	for _, s := range spaces {
		convs := s.Conversations
		if len(convs) == 0 {
			continue
		}
		stype := classifySpace(ctx, client, &s)
		activeConvs := 0
		for _, c := range convs {
			totalConvs++
			if c.DeleteTime != "" {
				deletedConvs++
			} else {
				activeConvs++
			}
		}
		switch stype {
		case "project":
			projectWithConvs++
			projectConvs += activeConvs
		default:
			simpleWithConvs++
			simpleConvs += activeConvs
		}
	}
	fmt.Fprintf(&b, "\nNon-empty spaces: %d\n", nonEmpty)
	fmt.Fprintf(&b, "  Simple chat spaces: %d (%d active conversations)\n", simpleWithConvs, simpleConvs)
	fmt.Fprintf(&b, "  Project spaces:     %d (%d active conversations)\n", projectWithConvs, projectConvs)
	fmt.Fprintf(&b, "  Total conversations: %d (active: %d, deleted: %d)\n", totalConvs, totalConvs-deletedConvs, deletedConvs)
	fmt.Fprintf(&b, "\nBrowser History should show: %d simple chats\n", simpleConvs)
	_, _ = fmt.Fprint(os.Stdout, b.String())
	return nil
}

// classifySpace returns "project" or "simple" based on the space's
// encrypted metadata. Unencrypted spaces are treated as simple.
func classifySpace(ctx context.Context, client *lumoClient.Client, s *lumo.Space) string {
	if s.Encrypted == "" {
		return "simple"
	}
	dek, err := client.DeriveSpaceDEK(ctx, s)
	if err != nil {
		return "simple"
	}
	ad := lumo.SpaceAD(s.SpaceTag)
	plainJSON, err := lumo.DecryptString(s.Encrypted, dek, ad)
	if err != nil {
		return "simple"
	}
	var priv lumo.SpacePriv
	if err := json.Unmarshal([]byte(plainJSON), &priv); err != nil {
		return "simple"
	}
	if priv.IsProject != nil && *priv.IsProject {
		return "project"
	}
	return "simple"
}

// SpaceRow is a display-only struct for the space list table.
type SpaceRow struct {
	ID         string
	CreateTime string
	ConvCount  int
	Encrypted  bool
	Name       string // decrypted name, "(empty)", or "(encrypted)"
}

// buildSpaceRows converts API spaces into display rows, decrypting
// names where possible. The decrypted name is used only for display
// and discarded after rendering.
func buildSpaceRows(ctx context.Context, client *lumoClient.Client, spaces []lumo.Space) []SpaceRow {
	rows := make([]SpaceRow, len(spaces))
	for i, s := range spaces {
		rows[i] = SpaceRow{
			ID:         s.ID,
			CreateTime: s.CreateTime,
			ConvCount:  len(s.Conversations),
			Encrypted:  s.Encrypted != "",
			Name:       decryptSpaceName(ctx, client, &s),
		}
	}
	return rows
}

// filterProjectSpaces returns only spaces that are project spaces
// (isProject=true in encrypted metadata). Simple chat spaces and
// spaces that can't be decrypted are excluded.
func filterProjectSpaces(ctx context.Context, client *lumoClient.Client, spaces []lumo.Space) []lumo.Space {
	var result []lumo.Space
	for i := range spaces {
		if classifySpace(ctx, client, &spaces[i]) == "project" {
			result = append(result, spaces[i])
		}
	}
	return result
}

// filterSimpleSpaces returns only simple chat spaces (not projects,
// not unencrypted orphans).
func filterSimpleSpaces(ctx context.Context, client *lumoClient.Client, spaces []lumo.Space) []lumo.Space {
	var result []lumo.Space
	for i := range spaces {
		if classifySpace(ctx, client, &spaces[i]) == "simple" {
			result = append(result, spaces[i])
		}
	}
	return result
}

// decryptSpaceName resolves a display name for a space. For project
// spaces, uses the ProjectName from encrypted metadata. For simple
// spaces (no ProjectName), uses the title of the first conversation.
// Returns "(empty)" when nothing is available, "(encrypted)" on
// decryption failure.
func decryptSpaceName(ctx context.Context, client *lumoClient.Client, s *lumo.Space) string {
	if s.Encrypted == "" {
		return "(empty)"
	}

	dek, err := client.DeriveSpaceDEK(ctx, s)
	if err != nil {
		return "(encrypted)"
	}

	ad := lumo.SpaceAD(s.SpaceTag)
	plainJSON, err := lumo.DecryptString(s.Encrypted, dek, ad)
	if err != nil {
		return "(encrypted)"
	}

	var priv lumo.SpacePriv
	if err := json.Unmarshal([]byte(plainJSON), &priv); err != nil {
		return "(encrypted)"
	}

	if priv.ProjectName != "" {
		return priv.ProjectName
	}

	// Simple space — derive name from the first conversation's title.
	for _, c := range s.Conversations {
		if c.Encrypted == "" || c.DeleteTime != "" {
			continue
		}
		title := decryptConversationTitle(c, dek, s.SpaceTag)
		if title != "" {
			return title
		}
	}

	return "Untitled"
}

// FormatSpaceList renders a tab-aligned table of spaces sorted by
// creation time descending (newest first).
func FormatSpaceList(rows []SpaceRow) string {
	if len(rows) == 0 {
		return "No spaces found.\n"
	}

	sorted := make([]SpaceRow, len(rows))
	copy(sorted, rows)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].CreateTime > sorted[j].CreateTime
	})

	var b strings.Builder
	fmt.Fprintf(&b, "%-36s  %-20s  %5s  %s\n", "ID", "CREATED", "CONVS", "NAME")
	for _, r := range sorted {
		fmt.Fprintf(&b, "%-36s  %-20s  %5d  %s\n", r.ID, r.CreateTime, r.ConvCount, r.Name)
	}
	return b.String()
}

// --- space delete ---

var spaceDeleteCmd = &cobra.Command{
	Use:   "delete <space-id>",
	Short: "Delete a space",
	Args:  cobra.ExactArgs(1),
	RunE:  runSpaceDelete,
}

func runSpaceDelete(cmd *cobra.Command, args []string) error {
	client, err := restoreClient(cmd)
	if err != nil {
		return err
	}

	spaceID := args[0]
	if err := client.DeleteSpace(cmd.Context(), spaceID); err != nil {
		return fmt.Errorf("deleting space: %w", err)
	}

	_, _ = fmt.Fprintf(os.Stderr, "Space %s deleted.\n", spaceID)
	return nil
}
