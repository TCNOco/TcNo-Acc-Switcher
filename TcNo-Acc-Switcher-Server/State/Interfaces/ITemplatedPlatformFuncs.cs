// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2022 TechNobo (Wesley Pyburn)
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

using System.Collections.Generic;
using System.Threading.Tasks;
using Newtonsoft.Json.Linq;

namespace TcNo_Acc_Switcher_Server.State.Interfaces;

public interface ITemplatedPlatformFuncs
{
    /// <summary>
    /// Restart Basic with a new account selected. Leave args empty to log into a new account.
    /// </summary>
    /// <param name="accId">(Optional) User's unique account ID</param>
    /// <param name="args">Starting arguments</param>
    void SwapTemplatedAccounts(string accId = "", string args = "");

    /// <summary>
    /// Highlights the specified account
    /// </summary>
    Task SetCurrentAccount(string accId);

    /// <summary>
    /// Removes "currently logged in" border from all accounts
    /// </summary>
    Task UnCurrentAllAccounts();

    Task<string> GetCurrentAccountId();
    Task ClearCache();

    /// <summary>
    /// Expands custom environment variables.
    /// </summary>
    /// <param name="path"></param>
    /// <param name="noIncludeBasicCheck">Whether to skip initializing BasicSettings - Useful for Steam and other hardcoded platforms</param>
    /// <returns></returns>
    string ExpandEnvironmentVariables(string path, bool noIncludeBasicCheck = false);

    Task<bool> TemplatedAddCurrent(string accName);

    /// <summary>
    /// Read a JSON file from provided path. Returns JObject
    /// </summary>
    Task<JToken> ReadJsonFile(string path);

    Task<string> GetUniqueId();
    Dictionary<string, string> ReadAllIds(string path = null);

    /// <summary>
    /// Swap to the current AppState.Switcher.SelectedAccountId.
    /// </summary>
    void SwapToAccount();

    /// <summary>
    /// Swaps to an empty account, allowing the user to sign in.
    /// </summary>
    void SwapToNewAccount();

    Task ForgetAccount();
    void RunPlatform(bool admin, string args = "");
    void RunPlatform();
    void HandleShortcutAction(string shortcut, string action);
}