using System.Collections;
using System.Collections.Generic;
using System.IO;
using System.Threading.Tasks;
using Newtonsoft.Json.Linq;

namespace TcNo_Acc_Switcher_Server.Data;

public interface IGeneralFuncs
{
    Task<bool> CanKillProcess(List<string> procNames, string closingMethod = "Combined", bool showModal = true);
    Task<bool> CanKillProcess(string processName, string closingMethod = "Combined", bool showModal = true);
    Task<bool> CloseProcesses(string procName, string closingMethod);
    Task<bool> CloseProcesses(List<string> procNames, string closingMethod);

    /// <summary>
    /// Waits for a program to close, and returns true if not running anymore.
    /// </summary>
    /// <param name="procName">Name of process to lookup</param>
    /// <returns>Whether it was closed before this function returns or not.</returns>
    Task<bool> WaitForClose(string procName);

    Task<bool> WaitForClose(List<string> procNames, string closingMethod);

    /// <summary>
    /// Restart the TcNo Account Switcher as Admin
    /// Launches either the Server or main exe, depending on what's currently running.
    /// </summary>
    void RestartAsAdmin(string args = "");

    /// <summary>
    /// Read all ids from requested platform file
    /// </summary>
    /// <param name="dictPath">Full *.json file path (file safe)</param>
    /// <param name="isBasic"></param>
    Dictionary<string, string> ReadDict(string dictPath, bool isBasic = false);

    void SaveDict(Dictionary<string, string> dict, string path, bool deleteIfEmpty = false);
    string WwwRoot();
    bool DeletedOutdatedFile(string filename);

    /// <summary>
    /// Checks if input file is older than 7 days, then deletes if it is
    /// </summary>
    /// <param name="filename">File path to be checked, and possibly deleted</param>
    /// <param name="daysOld">How many days old the file needs to be to be deleted</param>
    /// <returns>Whether file was deleted or not (Outdated or not)</returns>
    bool DeletedOutdatedFile(string filename, int daysOld);

    /// <summary>
    /// Checks if images is a valid GDI+ image, deleted if not.
    /// </summary>
    /// <param name="filename">File path of image to be checked</param>
    /// <returns>Whether file was deleted, or file was not deleted and was valid</returns>
    bool DeletedInvalidImage(string filename);

    Task JsDestNewline(string jsDest);
    Task DeleteFile(string file, string jsDest);

    /// <summary>
    /// Deletes a single file
    /// </summary>
    /// <param name="f">(Optional) FileInfo of file to delete</param>
    /// <param name="jsDest">Place to send responses (if any)</param>
    Task DeleteFile(FileInfo f, string jsDest);

    Task ClearFolder(string folder);

    /// <summary>
    /// Shorter RecursiveDelete (Sets keep folders to true)
    /// </summary>
    Task ClearFolder(string folder, string jsDest);

    /// <summary>
    /// Recursively delete files in folders (Choose to keep or delete folders too)
    /// </summary>
    /// <param name="baseDir">Folder to start working inwards from (as DirectoryInfo)</param>
    /// <param name="keepFolders">Set to False to delete folders as well as files</param>
    /// <param name="jsDest">Place to send responses (if any)</param>
    Task RecursiveDelete(DirectoryInfo baseDir, bool keepFolders, string jsDest);

    /// <summary>
    /// Deletes registry keys
    /// </summary>
    /// <param name="subKey">Subkey to delete</param>
    /// <param name="val">Value to delete</param>
    /// <param name="jsDest">Place to send responses (if any)</param>
    Task DeleteRegKey(string subKey, string val, string jsDest);

    /// <summary>
    /// Deletes all files of a specific type in a directory.
    /// </summary>
    /// <param name="folder">Folder to search for files in</param>
    /// <param name="extensions">Extensions of files to delete</param>
    /// <param name="so">SearchOption of where to look for files</param>
    /// <param name="jsDest">Place to send responses (if any)</param>
    Task ClearFilesOfType(string folder, string extensions, SearchOption so, string jsDest);

    /// <summary>
    /// Gets a file's MD5 Hash
    /// </summary>
    /// <param name="filePath">Path to file to get hash of</param>
    /// <returns></returns>
    string GetFileMd5(string filePath);

    string ReadOnlyReadAllText(string f);

    /// <summary>
    /// Converts file length to easily read string.
    /// </summary>
    /// <param name="len"></param>
    /// <returns></returns>
    string FileSizeString(double len);

    /// <summary>
    /// Saves input JArray of items to input file path
    /// </summary>
    /// <param name="file">File path to save JSON string to</param>
    /// <param name="joOrder">JArray order of items on page</param>
    void SaveOrder(string file, JArray joOrder);

    JObject LoadSettings(string file);

    /// <summary>
    /// Loads settings from input file (JSON string to JObject)
    /// </summary>
    /// <param name="file">JSON file to be read</param>
    /// <param name="defaultSettings">(Optional) Default JObject, for merging in missing parameters</param>
    /// <returns>JObject created from file</returns>
    JObject LoadSettings(string file, JObject defaultSettings);

    /// <summary>
    /// Read a JSON file from provided path. Returns JObject
    /// </summary>
    Task<JToken> ReadJsonFile(string path);

    /// <summary>
    /// Replaces last occurrence of string in string
    /// </summary>
    /// <param name="input">String to modify</param>
    /// <param name="sOld">String to find (and replace)</param>
    /// <param name="sNew">New string to input</param>
    /// <returns></returns>
    string ReplaceLast(string input, string sOld, string sNew);

    /// <summary>
    /// Escape text to be used as text inside HTML elements, using innerHTML
    /// </summary>
    /// <param name="text">String to escape</param>
    /// <returns>HTML escaped string</returns>
    string EscapeText(string text);

    /// <summary>
    /// Returns an object with a list of all translators, and proofreaders with their languages.
    /// </summary>
    GeneralFuncs.CrowdinResponse CrowdinList();

    Task HandleFirstRender(bool firstRender, string platform);

    /// <summary>
    /// For handling queries in URI
    /// </summary>
    Task<bool> HandleQueries();

    Task<string> ExportAccountList();

    /// <summary>
    /// JS function handler for saving settings from Settings GUI page into [Platform]Settings.json file
    /// </summary>
    /// <param name="file">Platform specific filename (has .json appended later)</param>
    /// <param name="jsonString">JSON String to be saved to file, from GUI</param>
    void GiSaveSettings(string file, string jsonString);

    void GiSaveOrder(string file, string jsonString);

    /// <summary>
    /// JS function handler for returning JObject of settings from [Platform]Settings.json file
    /// </summary>
    /// <param name="file">Platform specific filename (has .json appended later)</param>
    /// <returns>JObject of settings, to be loaded into GUI</returns>
    Task GiLoadSettings(string file);

    /// <summary>
    /// JS function handler for returning string contents of a *.* file
    /// </summary>
    /// <param name="file">Name of file to be read and contents returned in string format</param>
    /// <returns>string of file contents</returns>
    Task GiFileReadAllText(string file);

    /// <summary>
    /// JS function handler for running showModal JS function, with input arguments.
    /// </summary>
    /// <param name="args">Argument string, containing a command to be handled later by modal</param>
    /// <returns></returns>
    Task<bool> ShowModal(string args);

    /// <summary>
    /// JS function handler for showing Toast message.
    /// </summary>
    /// <param name="toastType">success, info, warning, error</param>
    /// <param name="toastMessage">Message to be shown in toast</param>
    /// <param name="toastTitle">(Optional) Title to be shown in toast (Empty doesn't show any title)</param>
    /// <param name="renderTo">(Optional) Part of the document to append the toast to (Empty = Default, document.body)</param>
    /// <param name="duration">(Optional) Duration to show the toast before fading</param>
    /// <returns></returns>
    Task<bool> ShowToast(string toastType, string toastMessage, string toastTitle = "", string renderTo = "body", int duration = 5000);

    /// <summary>
    /// JS function handler for showing Toast message.
    /// Instead of putting in messages (which you still can), use lang vars and they will expand.
    /// </summary>
    Task<bool> ShowToastLangVars(string toastType, string langToastMessage, string langToastTitle = "",
        string renderTo = "body", int duration = 5000);

    /// <summary>
    /// JS function handler for showing Toast message.
    /// Instead of putting in messages (which you still can), use lang vars and they will expand.
    /// </summary>
    Task<bool> ShowToastLangVars(string toastType, LangItem langItem, string langToastTitle = "",
        string renderTo = "body", int duration = 5000);

    /// <summary>
    /// Creates a shortcut to start the Account Switcher, and swap to the account related.
    /// </summary>
    /// <param name="args">(Optional) arguments for shortcut</param>
    Task CreateShortcut(string args = "");

    string PlatformUserModalCopyText();
    string PlatformHintText();
    string GiLocale(string k);
    string GiLocaleObj(string k, object obj);
    string GiCurrentBasicPlatform(string platform);
    string GiCurrentBasicPlatformExe(string platform);
    string GiGetCleanFilePath(string f);

    /// <summary>
    /// Save settings with Ctrl+S Hot key
    /// </summary>
    Task GiCtrlS(string platform);

    Task<bool> GenericLoadAccounts(string name, bool isBasic = false);

    /// <summary>
    /// Gets a list of 'account names' from cache folder provided
    /// </summary>
    /// <param name="folder">Cache folder containing accounts</param>
    /// <param name="accList">List of account strings</param>
    /// <returns>Whether the directory exists and successfully added listed names</returns>
    bool ListAccountsFromFolder(string folder, out List<string> accList);

    /// <summary>
    /// Orders a list of strings by order specific in jsonOrderFile
    /// </summary>
    /// <param name="accList">List of strings to sort</param>
    /// <param name="jsonOrderFile">JSON file containing list order</param>
    /// <returns></returns>
    List<string> OrderAccounts(List<string> accList, string jsonOrderFile);

    /// <summary>
    /// Runs jQueryProcessAccListSize, initContextMenu and initAccListSortable - Final init needed for account switcher to work.
    /// </summary>
    Task FinaliseAccountList();

    /// <summary>
    /// Iterate through account list and insert into platforms account screen
    /// </summary>
    /// <param name="accList">Account list</param>
    /// <param name="platform">Platform name</param>
    /// <param name="isBasic">(Unused for now) To use Basic platform account's ID as accId)</param>
    Task InsertAccounts(IEnumerable accList, string platform, bool isBasic = false);
}