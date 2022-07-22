﻿using System;
using System.Collections.Generic;

namespace TcNo_Acc_Switcher_Server.Shared.ContextMenu;

public class MenuItem
{
    private static readonly Lang Lang = Lang.Instance;
    private string _text;
    public string Text { get => Lang[_text] ?? _text ; set => _text = value; }
    public string Content { get; set; }
    public Action MenuAction;
    public List<MenuItem> Children = new();
}