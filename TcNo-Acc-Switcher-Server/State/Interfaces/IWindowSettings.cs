using System.Collections.Generic;

namespace TcNo_Acc_Switcher_Server.State;

public interface IWindowSettings
{
    string Language { get; set; }
    bool Rtl { get; set; }
    bool StreamerModeEnabled { get; set; }
    int ServerPort { get; set; }
    string WindowSize { get; set; }
    string Version { get; set; }
    List<object> DisabledPlatforms { get; }
    bool TrayMinimizeNotExit { get; set; }
    bool ShownMinimizedNotification { get; set; }
    bool StartCentered { get; set; }
    string ActiveTheme { get; set; }
    string ActiveBrowser { get; set; }
    string Background { get; set; }
    List<string> EnabledBasicPlatforms { get; }
    bool CollectStats { get; set; }
    bool ShareAnonymousStats { get; set; }
    bool MinimizeOnSwitch { get; set; }
    bool DiscordRpcEnabled { get; set; }
    bool DiscordRpcShareTotalSwitches { get; set; }
    void Save();
}