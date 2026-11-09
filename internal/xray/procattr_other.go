//go:build !linux

package xray

import "os/exec"

func setProcAttr(*exec.Cmd) {}
