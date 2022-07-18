using System;
using System.Collections.Generic;
using Microsoft.AspNetCore.Components;
using TcNo_Acc_Switcher_Server.Data;

namespace TcNo_Acc_Switcher_Server.Shared.ContextMenu;

public class MenuItem
{
    [Inject] private ILang Lang { get; }
    private string _text;
    public string Text { get => Lang[_text] ?? _text ; set => _text = value; }
    public string Content { get; set; }
    public Action MenuAction;
    public List<MenuItem> Children = new();
}