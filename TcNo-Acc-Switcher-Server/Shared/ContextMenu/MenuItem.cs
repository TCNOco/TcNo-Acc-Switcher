using System;
using System.Collections.Generic;
using Microsoft.AspNetCore.Components;
using TcNo_Acc_Switcher_Server.State.Interfaces;

namespace TcNo_Acc_Switcher_Server.Shared.ContextMenu;

public class MenuItem
{
    private string _text;
    public string Text { get => _text ?? _text ; set => _text = value; }
    public string Content { get; set; }
    public Action MenuAction;
    public List<MenuItem> Children = new();
}