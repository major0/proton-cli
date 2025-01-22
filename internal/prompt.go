package internal

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"syscall"

	"golang.org/x/term"
)

/* Prompt user for inputs. If `password` is true then the method will
 * disable echo'ing the password to the terminal. */
func UserPrompt(prompt string, password bool) (string, error) {
	var err error
	var input string
	var bytePasswd []byte
	reader := bufio.NewReader(os.Stdin)

	for input == "" {
		fmt.Print(prompt + ": ")
		if password {
			bytePasswd, err = term.ReadPassword(int(syscall.Stdin))
			input = string(bytePasswd)
			fmt.Println("")
		} else {
			input, err = reader.ReadString('\n')
		}

		if err != nil {
			return "", err
		}
	}
	if password {
		slog.Debug("UserPrompt", prompt, "<hidden>")
	} else {
		slog.Debug("UserPrompt", prompt, input)
	}
	return input, nil
}
