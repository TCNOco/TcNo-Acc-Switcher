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

using System;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.Linq;
using TcNo_Acc_Switcher_Server.State.Interfaces;

namespace TcNo_Acc_Switcher_Server.Shared.ContextMenu;

public class MenuBuilder
{
    private readonly ILang _lang;
    private readonly ObservableCollection<MenuItem> _menu;
    public ObservableCollection<MenuItem> Result() => _menu;
    public MenuBuilder()
    {
        _menu = new ObservableCollection<MenuItem>();
    }

    /// <summary>
    /// The new, preferred MenuBuilder (with lang support)
    /// </summary>
    public MenuBuilder(ILang lang)
    {
        _lang = lang;
        _menu = new ObservableCollection<MenuItem>();
    }
    public MenuBuilder(ILang lang, Tuple<string, object> item):this()
    {
        _lang = lang;
        AddItem(item);
    }
    public MenuBuilder(ILang lang, IEnumerable<Tuple<string, object>> items):this()
    {
        _lang = lang;
        AddItems(items);
    }

    public void AddItems(IEnumerable<Tuple<string, object>> items)
    {
        foreach (var item in items)
        {
            AddItem(item);
        }
    }

    public void AddMenuItem(MenuItem item)
    {
        _menu.Add(item);
    }
    public void AddItem(Tuple<string, object> item)
    {
        if (item is null) return;
        MenuItem menuItem = new()
        {
            Text = _lang?[item.Item1] ?? item.Item1
        };
        switch (item.Item2)
        {
            case string content:
            {
                menuItem.Content = content;
                break;
            }
            case IEnumerable<Tuple<string, object>> subItems:
            {
                var builder = new MenuBuilder(_lang);
                foreach(var subItem in subItems) {
                    builder.AddItem(subItem);
                }
                menuItem.Children = builder.Result().ToList();
                break;
            }
            // Example usage: new ("Test Action", () => System.Environment.Exit(1) ),
            case Action action:
            {
                menuItem.MenuAction = action;
                break;
            }
            case MenuItem existingMenuItem:
            {
                menuItem = existingMenuItem;
                break;
            }
            default:
            {
                throw new ArgumentException("Invalid menu item type");
            }
        }
        _menu.Add(menuItem);
    }
}