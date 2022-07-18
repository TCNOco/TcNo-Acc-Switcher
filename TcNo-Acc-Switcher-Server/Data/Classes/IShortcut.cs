using System.Collections.Generic;
using System.Drawing;
using System.IO;
using System.Threading.Tasks;

namespace TcNo_Acc_Switcher_Server.Data.Classes;

public interface IShortcut
{
    string Exe { get; set; }
    string WorkingDir { get; set; }
    string IconDir { get; set; }
    string ShortcutPath { get; set; }
    string Desc { get; set; }
    string Args { get; set; }
    public string Desktop { get; set; }
    public string StartMenu { get; set; }
    string ParentDirectory(string dir);
    bool ShortcutExist();
    string ShortcutDir();
    string GetSelfPath();

    /// <summary>
    /// Delete shortcut if it exists
    /// </summary>
    /// <param name="delFolder">(Optional) Whether to delete parent folder if it's empty</param>
    void DeleteShortcut(bool delFolder);

    /// <summary>
    /// Toggles whether a shortcut exists (Creates/Deletes depending on state)
    /// </summary>
    /// <param name="shouldExist">Whether the shortcut SHOULD exist</param>
    /// <param name="shouldFolderExist">Whether the shortcut ALREADY Exists</param>
    void ToggleShortcut(bool shouldExist, bool shouldFolderExist = true);

    /// <summary>
    /// Write shortcut to file if doesn't already exist
    /// </summary>
    void TryWrite();

    /// <summary>
    /// Fills in necessary info to create a shortcut to the TcNo Account Switcher
    /// </summary>
    /// <param name="location"></param>
    /// <returns></returns>
    Shortcut Shortcut_Switcher(string location);

    /// <summary>
    /// Sets up a Steam Tray shortcut
    /// </summary>
    /// <param name="location">Place to put shortcut</param>
    /// <returns></returns>
    Shortcut Shortcut_Tray(string location);

    /// <summary>
    /// Creates an icon file with multiple sizes, and combines the BG and FG images.
    /// </summary>
    /// <param name="bgImg">Background image, platform</param>
    /// <param name="fgImg">Foreground image, user image</param>
    /// <param name="iconName">Filename, unique so stored without being overwritten</param>
    Task CreateCombinedIcon(string bgImg, string fgImg, string iconName);

    /// <summary>
    /// Sets up a platform specific shortcut
    /// </summary>
    /// <param name="location">Place to put shortcut</param>
    /// <param name="platformName">(Optional) Name of the platform this shortcut is for (eg. steam)</param>
    /// <param name="args">(Optional) Arguments to add, default "steam" to open Steam page of switcher</param>
    /// <param name="descAdd">(Optional) Additional description to add to "TcNo Account Switcher - Steam</param>
    /// <param name="platformNameIsFullName">Whether the platformName is the fill name, or just to be appended</param>
    /// <returns></returns>
    Shortcut Shortcut_Platform(string location, string platformName = "Steam", string args = "steam", string descAdd = "", bool platformNameIsFullName = false);

    bool CheckShortcuts(string platform);
    void DesktopShortcut_Toggle(string platform, bool desktopShortcut);

    /// <summary>
    /// Checks if the TcNo Account Switcher Tray application has a Task to start with Windows
    /// </summary>
    bool StartWithWindows_Enabled();

    /// <summary>
    /// Toggles whether the TcNo Account Switcher Tray application starts with Windows or not
    /// </summary>
    /// <param name="shouldExist">Whether it should start with Windows, or not</param>
    void StartWithWindows_Toggle(bool shouldExist);

    /// <summary>
    /// Saves the specified <see cref="Bitmap"/> objects as a single
    /// icon into the output stream.
    /// </summary>
    /// <param name="images">The bitmaps to save as an icon.</param>
    /// <param name="stream">The output stream.</param>
    void SavePngsAsIcon(IEnumerable<Bitmap> images, Stream stream);

    /// <summary>
    /// Creates multiple images that are added to a single .ico file.
    /// Images are a combination of the platform's icon and the user accounts icon, for a good shortcut icon.
    /// </summary>
    /// <param name="sBgImg">Background image (platform's image)</param>
    /// <param name="sFgImg">User's profile image, for the foreground</param>
    /// <param name="icoOutput">Output filename</param>
    void CreateIcon(string sBgImg, string sFgImg, ref string icoOutput);

    /// <summary>
    /// Creates a combination of the platform's icon and the user accounts icon, for a good shortcut icon.
    /// </summary>
    /// <param name="sBgImg">Background image (platform's image)</param>
    /// <param name="sFgImg">User's profile image, for the foreground</param>
    /// <param name="output">Output filename</param>
    /// <param name="imgSize">Requested dimensions for the image</param>
    void CreateImage(MemoryStream sBgImg, string sFgImg, MemoryStream output, Size imgSize);

    Task CreatePlatformShortcut();
}