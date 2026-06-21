package basic

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/winutil"
)

func killPlatformExes(deps FlowDeps, fc FlowContext) error {
	closingMethod := winutil.ClosingMethod(fc.Settings.ClosingMethod)
	if err := winutil.ErrIfCannotKill(fc.Descriptor.ExesToEnd, closingMethod); err != nil {
		platform.EmitActionBarStatusI18nPlatform("Status_ClosingPlatformFailed", fc.PlatformKey)
		return err
	}
	platform.EmitActionBarStatusI18nPlatform("Status_ClosingPlatform", fc.PlatformKey)
	if err := winutil.KillByName(fc.Descriptor.ExesToEnd, closingMethod, electronBeforeKillSynth(deps, fc.PlatformKey, fc.Descriptor.ExesToEnd)); err != nil {
		platform.EmitActionBarStatusI18nPlatform("Status_ClosingPlatformFailed", fc.PlatformKey)
		return err
	}
	return nil
}

func primaryExeImageForKill(exes []string) string {
	const svc = "SERVICE:"
	for _, raw := range exes {
		e := strings.TrimSpace(raw)
		if e == "" || strings.HasPrefix(strings.ToUpper(e), strings.ToUpper(svc)) {
			continue
		}
		base := filepath.Base(e)
		if !strings.HasSuffix(strings.ToLower(base), ".exe") {
			base = strings.TrimSpace(e) + ".exe"
		}
		return base
	}
	return ""
}

func electronBeforeKillSynth(deps FlowDeps, platformKey string, exes []string) func() error {
	ps, err := platform.LoadPlatformSettings(platformKey)
	if err != nil || winutil.ClosingMethod(ps.ClosingMethod) != winutil.ClosingElectron {
		return nil
	}
	want := primaryExeImageForKill(exes)
	if want == "" {
		return nil
	}
	return func() error {
		if err := launchBasicNoStatus(deps, platformKey, nil); err != nil {
			return err
		}
		if !winutil.WaitForegroundForExe(want, electronKillForegroundWait) {
			logFlow().Warn("electron kill: foreground wait timeout", "image", want)
		}
		time.Sleep(electronKillForegroundSettle)
		return nil
	}
}

func launchBasicNoStatus(deps FlowDeps, platformKey string, extraLaunchArgs []string) error {
	return launchBasicNoStatusAs(deps, platformKey, false, extraLaunchArgs)
}

func launchBasicNoStatusAs(deps FlowDeps, platformKey string, forceAdmin bool, extraLaunchArgs []string) error {
	logFlow().Debug("launch begin", "platform", platformKey, "forceAdmin", forceAdmin, "extraArgs", len(extraLaunchArgs))
	fc, err := PrepareFlow(deps, platformKey)
	if err != nil {
		logFlow().Warn("launch read descriptor failed", "platform", platformKey, "err", err)
		return err
	}
	if deps.PS == nil {
		return fmt.Errorf("platform service not set")
	}
	exe, err := deps.PS.ResolvePlatformExeFullPath(platformKey)
	if err != nil || exe == "" {
		logFlow().Warn("launch resolve exe failed", "platform", platformKey, "exe", exe, "err", err)
		return fmt.Errorf("executable not found")
	}
	var args []string
	if strings.TrimSpace(fc.Descriptor.ExeExtraArgs) != "" {
		args = append(args, strings.Fields(fc.Descriptor.ExeExtraArgs)...)
	}
	args = append(args, platform.LaunchArgTokens(fc.Settings.LaunchArguments)...)
	if len(extraLaunchArgs) > 0 {
		args = append(args, extraLaunchArgs...)
	}
	admin := fc.Settings.RunAsAdmin
	if forceAdmin {
		admin = true
	}
	opts := winutil.StartOpts{
		Admin:         admin,
		Method:        winutil.StartingMethod(strings.TrimSpace(fc.Settings.StartingMethod)),
		HideWindow:    false,
		WorkingDir:    filepath.Dir(exe),
		AsDesktopUser: winutil.IsProcessElevated() && !admin,
	}
	logFlow().Debug("start request", "platform", platformKey, "exe", exe, "args", len(args), "method", opts.Method, "admin", opts.Admin)
	if err := winutil.Start(exe, args, opts); err != nil {
		logFlow().Warn("start failed", "platform", platformKey, "exe", exe, "err", err)
		return err
	}
	logFlow().Debug("start launched", "platform", platformKey, "exe", exe)
	return nil
}
