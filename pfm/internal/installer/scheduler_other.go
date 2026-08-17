//go:build !darwin

package installer

// schedulerIsLaunchd is false everywhere systemd is the user-session manager.
const schedulerIsLaunchd = false
