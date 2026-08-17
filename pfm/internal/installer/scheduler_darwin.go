//go:build darwin

package installer

// schedulerIsLaunchd selects the launchd agent over the systemd units. It is a
// build-time constant rather than a runtime check so the unused branch's asset
// list is decided the same way the code is.
const schedulerIsLaunchd = true
