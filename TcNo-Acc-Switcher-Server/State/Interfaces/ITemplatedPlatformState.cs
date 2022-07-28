using System.Collections.Generic;
using System.Threading.Tasks;
using Microsoft.JSInterop;
using TcNo_Acc_Switcher_Server.State.Classes.Templated;

namespace TcNo_Acc_Switcher_Server.State.Interfaces;

public interface ITemplatedPlatformState
{
    List<string> AvailablePlatforms { get; set; }
    TemplatedPlatformContextMenu ContextMenu { get; set; }
    Platform CurrentPlatform { get; set; }
    List<Platform> Platforms { get; set; }
    Dictionary<string, string> AccountIds { get; set; }
    void LoadTemplatedPlatformState(IJSRuntime jsRuntime, ITemplatedPlatformSettings templatedPlatformSettings, ITemplatedPlatformFuncs templatedPlatformFuncs);
    Task SetCurrentPlatform(IJSRuntime jsRuntime, ITemplatedPlatformSettings templatedPlatformSettings, string platformName);
    void SaveAccountOrder(string jsonString);
    void LoadAccountIds();
    void SaveAccountIds();
    string GetNameFromId(string accId);
}