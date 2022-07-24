using System;
using System.Threading.Tasks;
using TcNo_Acc_Switcher_Server.State.DataTypes;

namespace TcNo_Acc_Switcher_Server.State;

public interface IModals
{
    event Action OnChange;
    bool IsShown { get; set; }
    string Type { get; set; }
    string Title { get; set; }
    ExtraArg ExtraArgs { get; set; }
    StatsSelectorState CurrentStatsSelectorState { get; set; }
    PathPickerRequest PathPicker { get; set; }
    TextInputRequest TextInput { get; set; }
    void NotifyDataChanged();
    event Action PathPickerOnChange;
    void PathPickerNotifyDataChanged();
    event Action TextInputOnChange;
    void TextInputNotifyDataChanged();
    event Action GameStatsModalOnChange;
    void GameStatsModalOnChangeChanged();
    void ShowModal(string type, ExtraArg arg = ExtraArg.None);
    void ShowGameStatsSelectorModal();
    void ShowUpdatePlatformFolderModal();

    /// <summary>
    /// Change the selected platform's EXE folder to another and save changes
    /// </summary>
    void UpdatePlatformFolder();

    /// <summary>
    /// Show the PathPicker modal to find an image to import and use as the app background
    /// </summary>
    void ShowSetBackgroundModal();

    /// <summary>
    /// Import image to use as the app background
    /// </summary>
    void SetBackground();

    /// <summary>
    /// Show the PathPicker modal to move all Userdata files to a new folder, and set it as default.
    /// </summary>
    void ShowChangeUserdataFolderModal();

    /// <summary>
    /// Moves all Userdata files to a new folder, and set it as default.
    /// </summary>
    Task ChangeUserdataFolder();

    /// <summary>
    /// Show the Username change modal
    /// </summary>
    void ShowChangeAccImageModal();

    /// <summary>
    /// Update an accounts username
    /// </summary>
    Task ChangeAccImage();

    /// <summary>
    /// Show the Set App Password modal
    /// </summary>
    void ShowSetAppPasswordModal();

    /// <summary>
    /// Sets the App Password, to stop simple eyes from snooping
    /// </summary>
    Task SetAppPassword();

    /// <summary>
    /// Show the Set App Password modal
    /// </summary>
    void ShowSetAccountStringModal();

    /// <summary>
    /// Show the Set App Password modal
    /// </summary>
    void ShowChangeUsernameModal();

    Task SetAccountString();
    void TemplatedChangeUsername(string accId, string newName, bool reload = true);
}