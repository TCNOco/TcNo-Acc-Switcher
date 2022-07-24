using System.Collections.Generic;
using System.Threading.Tasks;

namespace TcNo_Acc_Switcher_Server.State;

public interface ISharedFunctions
{
    /// <summary>
    /// Runs jQueryProcessAccListSize, initContextMenu and initAccListSortable - Final init needed for account switcher to work.
    /// </summary>
    Task FinaliseAccountList();

    void RunShortcut(string s, string shortcutFolder, string platform = "", bool admin = false);
    bool CanKillProcess(List<string> procNames, string closingMethod = "Combined", bool showModal = true);
    bool CanKillProcess(string processName, string closingMethod = "Combined", bool showModal = true);
    Task<bool> CloseProcesses(string procName, string closingMethod);
    Task<bool> CloseProcesses(List<string> procNames, string closingMethod);

    /// <summary>
    /// Waits for a program to close, and returns true if not running anymore.
    /// </summary>
    /// <param name="procName">Name of process to lookup</param>
    /// <returns>Whether it was closed before this function returns or not.</returns>
    Task<bool> WaitForClose(string procName);

    Task<bool> WaitForClose(List<string> procNames, string closingMethod);
}