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

var spaceListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all spaces",
	RunE:    runSpaceList,
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

// decryptSpaceName attempts to decrypt a space's project name.
// Returns "(empty)" when no encrypted payload exists, the project name
// on success, or "(encrypted)" when decryption fails.
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

	if priv.ProjectName == "" {
		return "(empty)"
	}
	return priv.ProjectName
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
	fmt.Fprintf(&b, "%-36s  %-20s  %5s  %-9s  %s\n", "ID", "CREATED", "CONVS", "ENCRYPTED", "NAME")
	for _, r := range sorted {
		enc := "no"
		if r.Encrypted {
			enc = "yes"
		}
		fmt.Fprintf(&b, "%-36s  %-20s  %5d  %-9s  %s\n", r.ID, r.CreateTime, r.ConvCount, enc, r.Name)
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
