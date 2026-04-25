package lumoCmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/signal"
	"strings"

	"github.com/major0/proton-cli/api/lumo"
	lumoClient "github.com/major0/proton-cli/api/lumo/client"
)

// ChatSession holds the state for a single interactive chat session.
type ChatSession struct {
	Client       *lumoClient.Client
	Space        *lumo.Space
	Conversation *lumo.Conversation
	SpaceID      string
	Turns        []lumo.Turn
	Writer       io.Writer
	Reader       io.Reader
}

// IsEmptyInput reports whether the input is empty or whitespace-only.
func IsEmptyInput(s string) bool {
	return strings.TrimSpace(s) == ""
}

// Run executes the interactive chat loop. It reads lines from Reader,
// persists messages, calls Generate, and streams responses to Writer.
//
// The loop exits on EOF, /exit, or context cancellation. Ctrl+C during
// generation cancels only the current request — the loop continues.
// Generation and persistence errors are non-fatal.
func (s *ChatSession) Run(ctx context.Context) error {
	scanner := bufio.NewScanner(s.Reader)

	s.printStatusBar()
	s.prompt()

	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}

		line := scanner.Text()

		if IsEmptyInput(line) {
			s.prompt()
			continue
		}

		if cmd, _, ok := ParseSlashCommand(line); ok {
			switch ClassifyCommand(cmd) {
			case CmdExit:
				return nil
			case CmdHelp:
				_, _ = fmt.Fprintln(s.Writer, HelpText())
				s.prompt()
				continue
			default:
				_, _ = fmt.Fprintf(s.Writer, "Unknown command: /%s\n", cmd)
				s.prompt()
				continue
			}
		}

		// Persist user message.
		_, err := s.Client.CreateMessage(ctx, s.Space, s.Conversation, lumoClient.RoleUser, line)
		if err != nil {
			_, _ = fmt.Fprintf(s.Writer, "Warning: failed to save message: %v\n", err)
		}

		s.Turns = append(s.Turns, lumo.Turn{
			Role:    lumo.RoleUser,
			Content: line,
		})

		response, err := s.generate(ctx)
		if err != nil {
			s.handleGenerateError(err)
		}

		if response != "" {
			_, err = s.Client.CreateMessage(ctx, s.Space, s.Conversation, lumoClient.RoleAssistant, response)
			if err != nil {
				_, _ = fmt.Fprintf(s.Writer, "Warning: failed to save response: %v\n", err)
			}

			s.Turns = append(s.Turns, lumo.Turn{
				Role:    lumo.RoleAssistant,
				Content: response,
			})
		}

		_, _ = fmt.Fprintln(s.Writer)
		s.printStatusBar()
		s.prompt()
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read input: %w", err)
	}

	return nil
}

// generate calls the Lumo Generate API with Ctrl+C cancellation on a
// child context. Returns the accumulated response text.
func (s *ChatSession) generate(ctx context.Context) (string, error) {
	genCtx, stop := signal.NotifyContext(ctx, signalInterrupt()...)
	defer stop()

	var response strings.Builder

	targets := []lumo.GenerationTarget{lumo.TargetMessage}

	err := s.Client.Generate(genCtx, s.Turns, lumoClient.GenerateOpts{
		Targets: targets,
		ChunkCallback: func(msg lumo.GenerationResponseMessage) {
			if msg.Type == "token_data" && msg.Target == lumo.TargetMessage && msg.Content != "" {
				_, _ = fmt.Fprint(s.Writer, msg.Content)
				response.WriteString(msg.Content)
			}
		},
	})

	return response.String(), err
}

// handleGenerateError prints a user-friendly message for generation errors.
func (s *ChatSession) handleGenerateError(err error) {
	switch {
	case errors.Is(err, context.Canceled):
		_, _ = fmt.Fprintln(s.Writer, "\nCancelled.")
	case errors.Is(err, lumo.ErrRejected):
		_, _ = fmt.Fprintln(s.Writer, "\nRequest rejected.")
	case errors.Is(err, lumo.ErrHarmful):
		_, _ = fmt.Fprintln(s.Writer, "\nContent flagged.")
	case errors.Is(err, lumo.ErrTimeout):
		_, _ = fmt.Fprintln(s.Writer, "\nGeneration timed out.")
	default:
		_, _ = fmt.Fprintf(s.Writer, "\nError: %v\n", err)
	}
}

// printStatusBar writes the status bar to the writer.
func (s *ChatSession) printStatusBar() {
	bar := FormatStatusBar(s.Conversation.ID, "lumo", 60)
	_, _ = fmt.Fprintln(s.Writer, bar)
}

// prompt writes the input prompt.
func (s *ChatSession) prompt() {
	_, _ = fmt.Fprint(s.Writer, "> ")
}
