using System.Collections.Generic;
using System.Threading.Tasks;
using TcNo_Acc_Switcher_Server.State.DataTypes;

namespace TcNo_Acc_Switcher_Server.State;

public interface ISteamFuncs
{
    /// <summary>
    /// Returns string representation of Steam ePersonaState int
    /// </summary>
    /// <param name="ePersonaState">integer state to return string for</param>
    string PersonaStateToString(int ePersonaState);

    /// <summary>
    /// This relies on Steam updating loginusers.vdf. It could go out of sync assuming it's not updated reliably. There is likely a better way to do this.
    /// I am avoiding using the Steam API because it's another DLL to include, but is the next best thing - I assume.
    /// </summary>
    string GetCurrentAccountId(bool getNumericId = false);

    /// <summary>
    /// Swap to the current AppState.Switcher.SelectedAccountId.
    /// </summary>
    /// <param name="state">Optional profile state for Steam accounts</param>
    Task SwapToAccount(int state = -1);

    /// <summary>
    /// Swaps to an empty account, allowing the user to sign in.
    /// </summary>
    /// <param name="state">Optional profile state for Steam accounts</param>
    Task SwapToNewAccount(int state = -1);

    Task ForgetAccount();

    /// <summary>
    /// Restart Steam with a new account selected. Leave args empty to log into a new account.
    /// </summary>
    /// <param name="steamId">(Optional) User's SteamID</param>
    /// <param name="ePersonaState">(Optional) Persona state for user [0: Offline, 1: Online...]</param>
    /// <param name="args">Starting arguments</param>
    Task SwapSteamAccounts(string steamId = "", int ePersonaState = -1, string args = "");

    /// <summary>
    /// Highlights the specified account
    /// </summary>
    Task SetCurrentAccount(string accId);

    /// <summary>
    /// Removes "currently logged in" border from all accounts
    /// </summary>
    Task UnCurrentAllAccounts();

    /// <summary>
    /// Updates loginusers and registry to select an account as "most recent"
    /// </summary>
    /// <param name="selectedSteamId">Steam ID64 to switch to</param>
    /// <param name="pS">[PersonaState]0-7 custom persona state [0: Offline, 1: Online...]</param>
    Task UpdateLoginUsers(string selectedSteamId, int pS);

    /// <summary>
    /// Save updated list of Steamuser into loginusers.vdf, in vdf format.
    /// </summary>
    /// <param name="userAccounts">List of Steamuser to save into loginusers.vdf</param>
    Task SaveSteamUsersIntoVdf(List<SteamUser> userAccounts);

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