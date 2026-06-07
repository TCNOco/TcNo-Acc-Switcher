package app

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"TcNo-Acc-Switcher/internal/basic"
	"TcNo-Acc-Switcher/internal/cli"
	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/shortcuts"
	"TcNo-Acc-Switcher/internal/steam"
	"TcNo-Acc-Switcher/internal/winutil"
)

type Dispatch struct {
	SteamSvc    *steam.SteamService
	BasicSvc    *basic.BasicService
	PlatformSvc *platform.PlatformService
}

type ListAccountRow struct {
	UniqueID     string `json:"uniqueId"`
	DisplayName  string `json:"displayName"`
	LastLoggedIn string `json:"lastLoggedIn,omitempty"`
}

type ListPlatformRow struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

func (d *Dispatch) RunList(p cli.Parsed, idx *cli.PlatformIndex) error {
	switch p.Kind {
	case cli.KindListPlatforms:
		if idx == nil {
			return fmt.Errorf("platforms file not loaded")
		}
		rows := make([]ListPlatformRow, 0, len(idx.OrderedNames))
		for _, name := range idx.OrderedNames {
			code := cli.ShortTokenForPlatform(idx, name)
			if code == "" {
				code = "?"
			}
			rows = append(rows, ListPlatformRow{Code: code, Name: name})
		}
		if p.OutputJSON {
			b, err := json.Marshal(struct {
				Platforms []ListPlatformRow `json:"platforms"`
			}{Platforms: rows})
			if err != nil {
				return err
			}
			fmt.Println(string(b))
			return nil
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(tw, "code:\tplatform name:\n")
		for _, row := range rows {
			fmt.Fprintf(tw, "%s\t%s\n", row.Code, row.Name)
		}
		_ = tw.Flush()
		return nil

	case cli.KindListAccounts:
		var platNames []string
		if strings.TrimSpace(p.ListAccountsPlatform) != "" {
			platNames = []string{p.ListAccountsPlatform}
		} else {
			if idx == nil {
				return fmt.Errorf("platforms file not loaded")
			}
			platNames = append([]string(nil), idx.OrderedNames...)
		}

		if p.OutputJSON {
			if len(platNames) == 1 {
				rows, err := d.accountRowsForPlatform(platNames[0])
				if err != nil {
					return err
				}
				b, err := json.Marshal(struct {
					Platform string            `json:"platform"`
					Accounts []ListAccountRow  `json:"accounts"`
				}{Platform: platNames[0], Accounts: rows})
				if err != nil {
					return err
				}
				fmt.Println(string(b))
				return nil
			}
			type platBlock struct {
				Platform string           `json:"platform"`
				Accounts []ListAccountRow `json:"accounts"`
			}
			blocks := make([]platBlock, 0, len(platNames))
			for _, pk := range platNames {
				rows, err := d.accountRowsForPlatform(pk)
				if err != nil {
					return fmt.Errorf("%s: %w", pk, err)
				}
				if len(rows) == 0 {
					continue
				}
				blocks = append(blocks, platBlock{Platform: pk, Accounts: rows})
			}
			b, err := json.Marshal(struct {
				Platforms []platBlock `json:"platforms"`
			}{Platforms: blocks})
			if err != nil {
				return err
			}
			fmt.Println(string(b))
			return nil
		}

		for _, pk := range platNames {
			rows, err := d.accountRowsForPlatform(pk)
			if err != nil {
				return fmt.Errorf("%s: %w", pk, err)
			}
			if len(rows) == 0 {
				continue
			}
			fmt.Printf("%s:\n", pk)
			tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(tw, "  ID\tname\tlast login\n")
			for _, r := range rows {
				last := r.LastLoggedIn
				if last == "" {
					last = "-"
				}
				fmt.Fprintf(tw, "  %s\t%s\t%s\n", r.UniqueID, r.DisplayName, last)
			}
			_ = tw.Flush()
		}
		return nil

	default:
		return fmt.Errorf("internal: not a list command")
	}
}

func (d *Dispatch) accountRowsForPlatform(platformKey string) ([]ListAccountRow, error) {
	if strings.EqualFold(strings.TrimSpace(platformKey), steam.PlatformKey) {
		accs, err := d.SteamSvc.GetSteamAccounts()
		if err != nil {
			return nil, err
		}
		out := make([]ListAccountRow, 0, len(accs))
		for _, a := range accs {
			out = append(out, ListAccountRow{
				UniqueID:     a.SteamID64,
				DisplayName:  strings.TrimSpace(a.DisplayName),
				LastLoggedIn: strings.TrimSpace(a.LastLogin),
			})
		}
		return out, nil
	}
	accs, err := d.BasicSvc.GetAccounts(platformKey)
	if err != nil {
		return nil, err
	}
	out := make([]ListAccountRow, 0, len(accs))
	for _, a := range accs {
		out = append(out, ListAccountRow{
			UniqueID:     a.UniqueID,
			DisplayName:  strings.TrimSpace(a.DisplayName),
			LastLoggedIn: strings.TrimSpace(a.LastUsed),
		})
	}
	return out, nil
}

func (d *Dispatch) RunHeadless(p cli.Parsed) error {
	switch p.Kind {
	case cli.KindSwapSteam:
		if err := d.SteamSvc.SwapToSteamAccount(p.SteamID64, p.PersonaState, p.PassthroughLaunchArgs); err != nil {
			return err
		}
		return d.LaunchAfterSwap(p)
	case cli.KindSwapBasic:
		if err := basic.SwapTo(basic.FlowDeps{PS: d.PlatformSvc}, p.PlatformKey, p.UniqueID, p.PassthroughLaunchArgs); err != nil {
			return err
		}
		return d.LaunchAfterSwap(p)
	case cli.KindLogout:
		return d.RunLogout(p)
	default:
		return nil
	}
}

func (d *Dispatch) LaunchAfterSwap(p cli.Parsed) error {
	if strings.TrimSpace(p.RunAppID) != "" {
		url := "steam://rungameid/" + strings.TrimSpace(p.RunAppID)
		return winutil.Start("cmd.exe", []string{"/c", "start", "", url}, winutil.StartOpts{})
	}
	fn := strings.TrimSpace(p.RunShortcutFile)
	pk := strings.TrimSpace(p.PlatformKey)
	if fn != "" && pk != "" {
		return shortcuts.RunShortcut(pk, fn, false)
	}
	return nil
}

func (d *Dispatch) RunLogout(p cli.Parsed) error {
	plat := strings.TrimSpace(p.LogoutPlatform)
	if plat == "" || strings.EqualFold(plat, "Steam") {
		return d.SteamSvc.SteamAddNew()
	}
	deps := basic.FlowDeps{PS: d.PlatformSvc}
	fc, err := basic.PrepareFlow(deps, plat)
	if err != nil {
		return err
	}
	return basic.ClearCurrentLogin(deps, fc)
}
