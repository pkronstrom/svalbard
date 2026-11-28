package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type volume struct{ mount, device string }

func systemCommand(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = "/"
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %s", name, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}
