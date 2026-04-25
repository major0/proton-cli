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

var spaceListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List spaces (projects only; -A for all)",
	RunE:    runSpaceList,
}

func init() {
	spaceListCmd.Flags().BoolVarP(&spaceShowAll, "all", "A", false, "Show all spaces including simple chat spaces")
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

	if !spaceShowAll {
		spaces = filterProjectSpaces(ctx, client, spaces)
	}

	rows := buildSpaceRows(ctx, client, spaces)
	_, _ = fmt.Fprint(os.Stdout, FormatSpaceList(rows))
	return nil
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
		s := &spaces[i]
		if s.Encrypted == "" {
			continue
		}
		dek, err := client.DeriveSpaceDEK(ctx, s)
		if err != nil {
			continue
		}
		ad := lumo.SpaceAD(s.SpaceTag)
		plainJSON, err := lumo.DecryptString(s.Encrypted, dek, ad)
		if err != nil {
			continue
		}
		var priv lumo.SpacePriv
		if err := json.Unmarshal([]byte(plainJSON), &priv); err != nil {
			continue
		}
		if priv.IsProject != nil && *priv.IsProject {
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
