package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"golottery/api/internal/auth"
)

func runHashPassword() error {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}
	password := strings.TrimRight(string(raw), "\r\n")
	if password == "" {
		return fmt.Errorf("password is empty")
	}
	encoded, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	fmt.Println(encoded)
	return nil
}
