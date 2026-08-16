//go:build windows

package serverpicker

import (
	"errors"
	"fmt"
	"runtime"
	"strings"

	"TcNo-Acc-Switcher/internal/winutil"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

// rulePrefix namespaces every rule this feature owns, so enumeration can tell
// ours from the ~1000 rules a normal Windows install already carries, and an
// uninstall or reset only ever touches ours.
const rulePrefix = "TcNo Account Switcher - Server Picker - "

const (
	fwRuleDirectionOut = 2
	fwActionBlock      = 0
	fwProtocolAny      = 256
	fwProfileAll       = 0x7FFFFFFF
)

func ruleName(groupID string) string { return rulePrefix + groupID }

// withFirewallPolicy runs fn against INetFwPolicy2. COM is apartment-threaded,
// so the goroutine is pinned for the duration.
func withFirewallPolicy(fn func(rules *ole.IDispatch) error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// CoInitialize: S_OK (0), S_FALSE (1) both require CoUninitialize; RPC_E_CHANGED_MODE means skip uninit.
	const rpcEChangedMode = uintptr(0x80010106)
	var needUninit bool
	if err := ole.CoInitialize(0); err != nil {
		oe, ok := err.(*ole.OleError)
		if !ok {
			return fmt.Errorf("com init: %w", err)
		}
		switch oe.Code() {
		case 1:
			needUninit = true
		case rpcEChangedMode:
			needUninit = false
		default:
			return fmt.Errorf("com init: %w", err)
		}
	} else {
		needUninit = true
	}
	if needUninit {
		defer ole.CoUninitialize()
	}

	unk, err := oleutil.CreateObject("HNetCfg.FwPolicy2")
	if err != nil {
		return fmt.Errorf("create HNetCfg.FwPolicy2: %w", err)
	}
	defer unk.Release()

	policy, err := unk.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return fmt.Errorf("query IDispatch: %w", err)
	}
	defer policy.Release()

	rulesVar, err := oleutil.GetProperty(policy, "Rules")
	if err != nil {
		return fmt.Errorf("firewall rules: %w", err)
	}
	defer rulesVar.Clear()
	rules := rulesVar.ToIDispatch()
	if rules == nil {
		return errors.New("firewall rules unavailable")
	}
	return fn(rules)
}

// listBlockedGroupIDs reads back which groups are blocked right now. Reading
// works without elevation, which is what lets the page show honest state before
// the user restarts as admin - and catches rules changed outside the app.
func listBlockedGroupIDs() ([]string, error) {
	var ids []string
	err := withFirewallPolicy(func(rules *ole.IDispatch) error {
		return oleutil.ForEach(rules, func(v *ole.VARIANT) error {
			rule := v.ToIDispatch()
			if rule == nil {
				return nil
			}
			defer rule.Release()
			nameVar, err := oleutil.GetProperty(rule, "Name")
			if err != nil {
				return nil
			}
			defer nameVar.Clear()
			name := nameVar.ToString()
			if !strings.HasPrefix(name, rulePrefix) {
				return nil
			}
			if id := strings.TrimSpace(strings.TrimPrefix(name, rulePrefix)); id != "" {
				ids = append(ids, id)
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return normalizeGroupIDs(ids), nil
}

// applyBlock creates or removes the rule for one group. Creation always removes
// first, so re-applying after Valve reshuffles a POP's relay IPs replaces the
// rule rather than leaving a stale address list beside a new one.
func applyBlock(groupID string, ips []string, blocked bool) error {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return errors.New("empty group id")
	}
	if blocked && len(ips) == 0 {
		return fmt.Errorf("group %q has no relay addresses to block", groupID)
	}

	err := withFirewallPolicy(func(rules *ole.IDispatch) error {
		name := ruleName(groupID)
		removeRule(rules, name)
		if !blocked {
			return nil
		}

		unk, err := oleutil.CreateObject("HNetCfg.FWRule")
		if err != nil {
			return fmt.Errorf("create HNetCfg.FWRule: %w", err)
		}
		defer unk.Release()
		rule, err := unk.QueryInterface(ole.IID_IDispatch)
		if err != nil {
			return fmt.Errorf("query IDispatch: %w", err)
		}
		defer rule.Release()

		for _, set := range []struct {
			prop  string
			value any
		}{
			{"Name", name},
			{"Description", "Blocked by TcNo Account Switcher: " + groupID},
			{"Direction", fwRuleDirectionOut},
			{"Action", fwActionBlock},
			{"Protocol", fwProtocolAny},
			{"RemoteAddresses", strings.Join(ips, ",")},
			{"Profiles", fwProfileAll},
			{"Enabled", true},
		} {
			if _, err := oleutil.PutProperty(rule, set.prop, set.value); err != nil {
				return fmt.Errorf("firewall rule %s: %w", set.prop, err)
			}
		}
		if _, err := oleutil.CallMethod(rules, "Add", rule); err != nil {
			return fmt.Errorf("add firewall rule: %w", err)
		}
		return nil
	})
	if err != nil {
		return wrapAccessDenied(err)
	}
	return nil
}

// removeRule drops every rule with this name. A loop because the API allows
// duplicates under one name and Remove only takes the first.
func removeRule(rules *ole.IDispatch, name string) {
	for i := 0; i < 32; i++ {
		if _, err := oleutil.CallMethod(rules, "Item", name); err != nil {
			return
		}
		if _, err := oleutil.CallMethod(rules, "Remove", name); err != nil {
			return
		}
	}
}

// eAccessDenied is what the firewall COM object returns to an unelevated caller.
const eAccessDenied = uintptr(0x80070005)

// wrapAccessDenied turns the COM refusal into the marker the frontend already
// knows how to act on, so the picker reuses the app's restart-as-admin flow.
//
// The refusal arrives as DISP_E_EXCEPTION whose message is the useless
// "Exception occurred."; the real status is the EXCEPINFO's SCODE, so match on
// the code rather than on any text.
func wrapAccessDenied(err error) error {
	if err == nil {
		return nil
	}
	if hresultIs(err, eAccessDenied) {
		return winutil.NewNeedsAdminError("")
	}
	if strings.Contains(strings.ToLower(err.Error()), "access is denied") {
		return winutil.NewNeedsAdminError("")
	}
	return err
}

func hresultIs(err error, want uintptr) bool {
	var ex ole.EXCEPINFO
	if errors.As(err, &ex) && uintptr(ex.SCODE()) == want {
		return true
	}
	var oe *ole.OleError
	for errors.As(err, &oe) {
		if oe.Code() == want {
			return true
		}
		if sub, isEx := oe.SubError().(ole.EXCEPINFO); isEx && uintptr(sub.SCODE()) == want {
			return true
		}
		err = oe.SubError()
		oe = nil
	}
	return false
}
