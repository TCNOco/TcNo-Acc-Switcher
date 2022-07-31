// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2022 TechNobo (Wesley Pyburn)
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

using System.Collections.Generic;
using System.ComponentModel;
using System.Runtime.CompilerServices;
using Microsoft.AspNetCore.Components;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.State.DataTypes;
using TcNo_Acc_Switcher_Server.State.Interfaces;

namespace TcNo_Acc_Switcher_Server.State.Classes;

public class Account : INotifyPropertyChanged
{
    private bool _isChecked;
    private bool _isCurrent;
    private string _titleText = "";
    private string _platform = "";
    private string _imagePath = "";
    private string _displayName = "";
    private string _loginUsername = "";
    private string _accountId = "";
    private string _line0 = "";
    private string _line2 = "";
    private string _line3 = "";
    private string _note = "";
    private Dictionary<string, Dictionary<string, StatValueAndIcon>> _userStats;
    private ClassCollection _classes = new();
    public event PropertyChangedEventHandler? PropertyChanged;

    protected void OnPropertyChanged(string propertyName) => PropertyChanged?.Invoke(this, new PropertyChangedEventArgs(propertyName));

    protected bool SetField<T>(ref T field, T value, [CallerMemberName] string propertyName = "")
    {
        if (EqualityComparer<T>.Default.Equals(field, value)) return false;
        field = value;
        OnPropertyChanged(propertyName);
        return true;
    }

    public bool IsChecked
    {
        get => _isChecked;
        set
        {
            if (_isChecked != value) SetField(ref _isChecked, value);
        }
    }

    public bool IsCurrent
    {
        get => _isCurrent;
        set
        {
            if (_isCurrent != value) SetField(ref _isCurrent, value);
        }
    }

    public string TitleText
    {
        get => _titleText;
        set
        {
            if (_titleText != value) SetField(ref _titleText, value);
        }
    }

    public string Platform
    {
        get => _platform;
        set => SetField(ref _platform, value);
    }

    public string ImagePath
    {
        get => _imagePath;
        set => SetField(ref _imagePath, value);
    }

    public string DisplayName
    {
        get => _displayName;
        set
        {
            SafeDisplayName = GeneralFuncs.EscapeText(value);
            SetField(ref _displayName, value);
        }
    }

    public string LoginUsername
    {
        get => _loginUsername;
        set => SetField(ref _loginUsername, value);
    }

    public string SafeDisplayName { get; set; }

    public string AccountId
    {
        get => _accountId;
        set => SetField(ref _accountId, value);
    }

    public string Line0
    {
        get => _line0;
        set => SetField(ref _line0, value);
    }

    public string Line2
    {
        get => _line2;
        set => SetField(ref _line2, value);
    }

    public string Line3
    {
        get => _line3;
        set => SetField(ref _line3, value);
    }

    public string Note
    {
        get => _note;
        set
        {
            if (_note != value) SetField(ref _note, value);
        }
    }

    public Dictionary<string, Dictionary<string, StatValueAndIcon>> UserStats
    {
        get => _userStats;
        set => SetField(ref _userStats, value);
    }

    public void SetUserStats(IGameStats gameStats)
    {
        UserStats = gameStats.GetUserStatsAllGamesMarkup(AccountId);
        OnPropertyChanged(nameof(UserStats));
    }

    public ClassCollection Classes
    {
        get => _classes;
        set => SetField(ref _classes, value);
    }

    // Optional
    public class ClassCollection
    {
        public string Label { get; set; } = "";
        public string Image { get; set; } = "";
        public string Line0 { get; set; } = "";
        public string Line2 { get; set; } = "";
        public string Line3 { get; set; } = "";
    }
}