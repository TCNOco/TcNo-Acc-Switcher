using System.Collections.Generic;
using System.Threading.Tasks;
using Newtonsoft.Json.Linq;

namespace TcNo_Acc_Switcher_Server.State;

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
}