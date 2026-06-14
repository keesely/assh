// Package mount 实现远程文件系统挂载的基础设施层。
//
// Phase 12: 远程服务器挂载 (sshfs + WebDAV)
package mount

import (
	"os/exec"
	"runtime"
	"strings"

	"assh/asshc/port"
)

// CheckDependencies 检查所有挂载类型所需的依赖状态。
func CheckDependencies() map[port.MountType]port.DependencyStatus {
	return map[port.MountType]port.DependencyStatus{
		port.MountTypeSSHFS:  checkSSHFSDependencies(),
		port.MountTypeWebDAV: checkWebDAVDependencies(),
	}
}

// checkSSHFSDependencies 检查 sshfs 相关依赖。
func checkSSHFSDependencies() port.DependencyStatus {
	var items []port.DependencyItem
	allAvailable := true

	// 检查 sshfs
	sshfsItem := checkCommand("sshfs", "--version")
	items = append(items, sshfsItem)
	if !sshfsItem.Found {
		allAvailable = false
	}

	// 检查 ssh (必需)
	sshItem := checkCommand("ssh", "-V")
	items = append(items, sshItem)
	if !sshItem.Found {
		allAvailable = false
	}

	// 检查 unmount 命令
	if runtime.GOOS == "darwin" {
		items = append(items, checkCommand("umount", ""))
	} else {
		items = append(items, checkCommand("fusermount", "--version"))
	}

	return port.DependencyStatus{
		Available: allAvailable,
		Details:   items,
	}
}

// checkWebDAVDependencies 检查 WebDAV 相关依赖。
func checkWebDAVDependencies() port.DependencyStatus {
	var items []port.DependencyItem
	allAvailable := true

	// 检查 ssh (必需)
	sshItem := checkCommand("ssh", "-V")
	items = append(items, sshItem)
	if !sshItem.Found {
		allAvailable = false
	}

	// 检查挂载客户端
	if runtime.GOOS == "darwin" {
		// macOS: mount_webdav 内置
		items = append(items, port.DependencyItem{
			Name:    "mount_webdav",
			Found:   true,
			Path:    "/sbin/mount_webdav",
			Version: "system built-in",
			Install: "",
		})
	} else {
		// Linux: davfs2
		davItem := checkCommand("mount.davfs", "--version")
		items = append(items, davItem)
		if !davItem.Found {
			allAvailable = false
		}
	}

	return port.DependencyStatus{
		Available: allAvailable,
		Details:   items,
	}
}

// checkCommand 检查单个命令是否可用。
func checkCommand(name string, versionFlag string) port.DependencyItem {
	item := port.DependencyItem{
		Name:    name,
		Found:   false,
		Install: getInstallHint(name),
	}

	path, err := exec.LookPath(name)
	if err != nil {
		return item
	}
	item.Path = path
	item.Found = true

	// 尝试获取版本
	if versionFlag != "" {
		cmd := exec.Command(name, versionFlag)
		output, err := cmd.CombinedOutput()
		if err == nil {
			item.Version = parseVersion(string(output))
		}
	}

	return item
}

// parseVersion 从命令输出中提取版本号。
func parseVersion(output string) string {
	lines := strings.Split(output, "\n")
	if len(lines) > 0 {
		// 返回第一行（通常是版本信息）
		return strings.TrimSpace(lines[0])
	}
	return ""
}

// getInstallHint 获取安装提示。
func getInstallHint(name string) string {
	switch name {
	case "sshfs":
		if runtime.GOOS == "darwin" {
			return "brew install macfuse sshfs"
		}
		return "apt install sshfs / yum install sshfs"
	case "ssh":
		return "apt install openssh-client / yum install openssh-clients"
	case "fusermount":
		return "apt install fuse / yum install fuse"
	case "mount.davfs":
		return "apt install davfs2 / yum install davfs2"
	default:
		return ""
	}
}
