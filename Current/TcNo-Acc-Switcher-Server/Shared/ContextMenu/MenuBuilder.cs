using System;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.Linq;

namespace TcNo_Acc_Switcher_Server.Shared.ContextMenu;

public class MenuBuilder
{
    private readonly ObservableCollection<MenuItem> _menu;
    public ObservableCollection<MenuItem> Result() => _menu;
    public MenuBuilder()
    {
        _menu = new ObservableCollection<MenuItem>();
    }

    public MenuBuilder(Tuple<string, object> item):this()
    {
        AddItem(item);
    }
    public MenuBuilder(IEnumerable<Tuple<string, object>> items):this()
    {
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
            {
                var builder = new MenuBuilder();
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