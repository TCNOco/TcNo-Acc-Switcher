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

namespace TcNo_Acc_Switcher.State.DataTypes;

public class TextInputRequest : INotifyPropertyChanged
{
    public event PropertyChangedEventHandler? PropertyChanged;
    protected void OnPropertyChanged(string propertyName) => PropertyChanged?.Invoke(this, new PropertyChangedEventArgs(propertyName));
    protected bool SetField<T>(ref T field, T value, [CallerMemberName] string propertyName = "")
    {
        if (EqualityComparer<T>.Default.Equals(field, value)) return false;
        field = value;
        OnPropertyChanged(propertyName);
        return true;
    }

    public string LastString = "";
    public TextInputGoal Goal
    {
        get => _goal;
        set => SetField(ref _goal, value);
    }
    private TextInputGoal _goal;

    public string ModalHeader;
    public MarkupString ModalText;
    public MarkupString ModalSubheading = new();
    public string ModalButtonText;
    public MarkupString ExtraButtons = new();

    public TextInputRequest() {}
    public TextInputRequest(TextInputGoal goal)
    {
        // Clear existing data, if any.
        LastString = "";
        Goal = goal;
    }
}