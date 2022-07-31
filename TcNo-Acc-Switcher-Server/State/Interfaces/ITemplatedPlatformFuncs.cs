using System.Collections.Generic;
using System.Threading.Tasks;
using Microsoft.JSInterop;
using Newtonsoft.Json.Linq;

namespace TcNo_Acc_Switcher_Server.State.Interfaces;

public interface ITemplatedPlatformFuncs
{
    /// <summary>
    /// Restart Basic with a new account selected. Leave args empty to log into a new account.
    /// </summary>
    /// <param name="jsRuntime"></param>
    /// <param name="accId">(Optional) User's unique account ID</param>
    /// <param name="args">Starting arguments</param>
    Task SwapTemplatedAccounts(IJSRuntime jsRuntime, string accId = "", string args = "");

    /// <summary>
    /// Highlights the specified account
    /// </summary>
    void SetCurrentAccount(string accId);

    string GetCurrentAccountId();
    void ClearCache();

    /// <summary>
    /// Expands custom environment variables.
    /// </summary>
    /// <param name="path"></param>
    /// <param name="noIncludeBasicCheck">Whether to skip initializing BasicSettings - Useful for Steam and other hardcoded platforms</param>
    /// <returns></returns>
    string ExpandEnvironmentVariables(string path, bool noIncludeBasicCheck = false);

    bool TemplatedAddCurrent(string accName);

    /// <summary>
    /// Read a JSON file from provided path. Returns JObject
    /// </summary>
    JToken ReadJsonFile(string path);

    string GetUniqueId();
    Dictionary<string, string> ReadAllIds(string path = null);

    /// <summary>
    /// Swap to the current AppState.Switcher.SelectedAccountId.
    /// </summary>
    Task SwapToAccount(IJSRuntime jsRuntime);

    /// <summary>
    /// Swaps to an empty account, allowing the user to sign in.
    /// </summary>
    Task SwapToNewAccount(IJSRuntime jsRuntime);

    void ForgetAccount();
    void RunPlatform(bool admin, string args = "");
    void RunPlatform();
}