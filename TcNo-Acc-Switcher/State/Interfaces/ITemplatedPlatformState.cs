using System.Collections.Generic;
using System.Threading.Tasks;
using Microsoft.JSInterop;
using TcNo_Acc_Switcher.State.Classes.Templated;

namespace TcNo_Acc_Switcher.State.Interfaces;

public interface ITemplatedPlatformState
{
    List<string> AvailablePlatforms { get; set; }
    TemplatedPlatformContextMenu ContextMenu { get; set; }
    Classes.Templated.Platform CurrentPlatform { get; set; }
    List<Classes.Templated.Platform> Platforms { get; set; }
    Dictionary<string, string> AccountIds { get; set; }
    void LoadTemplatedPlatformState(IJSRuntime jsRuntime, ITemplatedPlatformSettings templatedPlatformSettings);
    Task SetCurrentPlatform(IJSRuntime jsRuntime, ITemplatedPlatformSettings templatedPlatformSettings, ITemplatedPlatformFuncs templatedPlatformFuncs, IStatistics statistics, string platformName);
    void SaveAccountOrder(string jsonString);
    void ImportAccountImage(string uniqueId);
    void LoadAccountIds();
    void SaveAccountIds();
    string GetNameFromId(string accId);
}