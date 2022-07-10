using System;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.Linq;

namespace TcNo_Acc_Switcher_Server.Shared;

public static class MenuBuilder
{
    public static ObservableCollection<MenuItem> Build(IEnumerable<Tuple<string, object>> items)
    {
        var menu = new ObservableCollection<MenuItem>();
        foreach (var item in items)
        {
            MenuItem menuItem = new()
            {
                Text = item.Item1
            };
            switch (item.Item2)
            {
                case string content:
                {
                    menuItem.Content = content;
                    break;
                }
                case IEnumerable<Tuple<string, object>> subItems:
                    menuItem.Children = Build(subItems).ToList();
                    break;
            }
            menu.Add(menuItem);
        }
        return menu;
    }
}