//go:build windows

package xray

import (
	"os/exec"
)

func setProcessUser(cmd *exec.Cmd) {
	// Windows 不支持 Unix 风格的用户切换
}

func setConfigFileOwner(path string) {
	// Windows 不需要 chown
}
