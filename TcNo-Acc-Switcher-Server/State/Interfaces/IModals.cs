using System;
using System.Threading.Tasks;
using TcNo_Acc_Switcher_Server.State.DataTypes;

namespace TcNo_Acc_Switcher_Server.State.Interfaces;

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
    /// Show the PathPicker modal to find an image to import and use as the app background
    /// </summary>
    void ShowSetBackgroundModal();

    /// <summary>
    /// Show the Username change modal
    /// </summary>
    void ShowChangeAccImageModal();

    /// <summary>
    /// Show the Set App Password modal
    /// </summary>
    void ShowChangeUsernameModal();
}