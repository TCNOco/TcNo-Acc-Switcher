using System;
using Microsoft.AspNetCore.Components;
using Microsoft.JSInterop;
using TcNo_Acc_Switcher_Server.State.DataTypes;

namespace TcNo_Acc_Switcher_Server.State
{
    public class Modals
    {
        [Inject] JSRuntime JsRuntime { get; set; }

        public event Action OnChange;
        public void NotifyDataChanged() => OnChange?.Invoke();

        private bool _isShown;
        public bool IsShown
        {
            get => _isShown;
            set
            {
                _isShown = value;
                JsRuntime.InvokeVoidAsync(value ? "showModal" : "hideModal");
                NotifyDataChanged();
            }
        }
        private string _type;
        public string Type
        {
            get => _type;
            set
            {
                _type = value;
                NotifyDataChanged();
            }
        }

        private string _title;
        public string Title
        {
            get => _title;
            set
            {
                _title = value;
                NotifyDataChanged();
            }
        }

        public static ExtraArg ExtraArgs { get; set; }
        public StatsSelectorState CurrentStatsSelectorState { get; set; }


        #region PathPicker
        public PathPickerRequest PathPicker { get; set; }

        // These MUST be separate from the class above to refresh the element properly.
        public event Action PathPickerOnChange;
        public void PathPickerNotifyDataChanged() => PathPickerOnChange?.Invoke();
        #endregion

        #region TextInputModal
        public static TextInputRequest TextInput { get; set; }

        // These MUST be separate from the class above to refresh the element properly.
        public event Action TextInputOnChange;
        public void TextInputNotifyDataChanged() => TextInputOnChange?.Invoke();
        #endregion

        #region GameStatsModal
        public event Action GameStatsModalOnChange;
        public void GameStatsModalOnChangeChanged() => GameStatsModalOnChange?.Invoke();
        #endregion

        public void ShowModal(string type, ExtraArg arg = ExtraArg.None)
        {
            Type = type;
            ExtraArgs = arg;
            IsShown = true;
        }

        public void ShowGameStatsSelectorModal()
        {
            CurrentStatsSelectorState = StatsSelectorState.GamesList;
            ShowModal("gameStatsSelector");
            GameStatsModalOnChangeChanged();
        }
    }

    public enum ExtraArg
    {
        None,
        RestartAsAdmin,
        ClearStats,
        ForgetAccount
    }

    public enum StatsSelectorState
    {
        GamesList,
        VarsList
    }
}