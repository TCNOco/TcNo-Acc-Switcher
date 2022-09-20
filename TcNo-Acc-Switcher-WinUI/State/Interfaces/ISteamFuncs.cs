using System.Collections.Generic;
using System.Threading.Tasks;
using Microsoft.JSInterop;
using TcNo_Acc_Switcher.State.DataTypes;

namespace TcNo_Acc_Switcher.State.Interfaces;

public interface ISteamFuncs
{
    /// <summary>
    /// Returns string representation of Steam ePersonaState int
    /// </summary>
    /// <param name="ePersonaState">integer state to return string for</param>
    string PersonaStateToString(int ePersonaState);

    /// <summary>
    /// Switch to an account, and launch a game - instead of starting Steam.
    /// </summary>
    Task SwitchAndLaunch(IJSRuntime jsRuntime, string appId);

    /// <summary>
    /// Switch to a specified account, and launch a game - instead of starting Steam.
    /// This is to be used with the CLI.
    /// </summary>
    Task SwitchAndLaunch(IJSRuntime jsRuntime, string appId, string steamId);

    /// <summary>
    /// Swap to the current AppState.Switcher.SelectedAccountId.
    /// </summary>
    /// <param name="jsRuntime"></param>
    /// <param name="state">Optional profile state for Steam accounts</param>
    Task SwapToAccount(IJSRuntime jsRuntime, int state = -1);

    /// <summary>
    /// Switch to an account, and launch a game via the selected shortcut
    /// Launches Steam, but does not wait for it to start fully.
    /// </summary>
    Task SwitchAndLaunchShortcut(IJSRuntime jsRuntime);

    /// <summary>
    /// Swaps to an empty account, allowing the user to sign in.
    /// </summary>
    /// <param name="jsRuntime"></param>
    /// <param name="state">Optional profile state for Steam accounts</param>
    Task SwapToNewAccount(IJSRuntime jsRuntime, int state = -1);

    void ForgetAccount();

    /// <summary>
    /// Restart Steam with a new account selected. Leave args empty to log into a new account.
    /// </summary>
    /// <param name="jsRuntime"></param>
    /// <param name="steamId">(Optional) User's SteamID</param>
    /// <param name="ePersonaState">(Optional) Persona state for user [0: Offline, 1: Online...]</param>
    /// <param name="args">Starting arguments</param>
    /// <param name="canStartSteam">Whether steam should be allowed to launch (ignores settings)</param>
    Task SwapSteamAccounts(IJSRuntime jsRuntime, string steamId = "", int ePersonaState = -1, string args = "", bool canStartSteam = true);

    /// <summary>
    /// Highlights the specified account
    /// </summary>
    void SetCurrentAccount(string accId);

    /// <summary>
    /// Updates loginusers and registry to select an account as "most recent"
    /// </summary>
    /// <param name="selectedSteamId">Steam ID64 to switch to</param>
    /// <param name="pS">[PersonaState]0-7 custom persona state [0: Offline, 1: Online...]</param>
    void UpdateLoginUsers(string selectedSteamId, int pS);

    /// <summary>
    /// Save updated list of SteamUser into loginusers.vdf, in vdf format.
    /// </summary>
    /// <param name="userAccounts">List of SteamUser to save into loginusers.vdf</param>
    void SaveSteamUsersIntoVdf(List<SteamUser> userAccounts);

    /// <summary>
    /// Sets whether the user is invisible or not
    /// </summary>
    /// <param name="steamId">SteamID of user to update</param>
    /// <param name="ePersonaState">Persona state enum for user (0-7)</param>
    void SetPersonaState(string steamId, int ePersonaState);

    /// <summary>
    /// Copy settings from currently logged in account to selected game and account
    /// </summary>
    void CopySettingsFrom(string gameId);

    void RestoreSettingsTo(string gameId);
    void BackupGameData(string gameId);
}