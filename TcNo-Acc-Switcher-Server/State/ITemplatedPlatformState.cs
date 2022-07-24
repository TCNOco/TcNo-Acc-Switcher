using System.Collections.Generic;
using System.Threading.Tasks;
using TcNo_Acc_Switcher_Server.State.Classes.Templated;

namespace TcNo_Acc_Switcher_Server.State;

public interface ITemplatedPlatformState
{
    List<string> AvailablePlatforms { get; set; }
    TemplatedPlatformContextMenu ContextMenu { get; set; }
    Platform CurrentPlatform { get; set; }
    List<Platform> Platforms { get; set; }
    Task SetCurrentPlatform(string platformName);
    void LoadAccountIds();
    void SaveAccountIds();
    string GetNameFromId(string accId);
    void RunPlatform(bool admin, string args = "");
    void RunPlatform();
    void HandleShortcutAction(string shortcut, string action);
}