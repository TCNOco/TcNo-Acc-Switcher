using System.Collections.Generic;

namespace TcNo_Acc_Switcher.State.Interfaces;

public interface ISharedFunctions
{
    void RunShortcut(string s, string shortcutFolder, string platform = "", bool admin = false);
    bool CanKillProcess(List<string> procNames, string closingMethod = "Combined", bool showModal = true);
    bool CanKillProcess(string processName, string closingMethod = "Combined", bool showModal = true);
    bool CloseProcesses(string procName, string closingMethod);
    bool CloseProcesses(List<string> procNames, string closingMethod);

    /// <summary>
    /// Waits for a program to close, and returns true if not running anymore.
    /// </summary>
    /// <param name="procName">Name of process to lookup</param>
    /// <returns>Whether it was closed before this function returns or not.</returns>
    bool WaitForClose(string procName);

    bool WaitForClose(List<string> procNames, string closingMethod);
}