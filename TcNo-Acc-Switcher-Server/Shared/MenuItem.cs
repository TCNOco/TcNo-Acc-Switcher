using System.Collections.Generic;
using TcNo_Acc_Switcher_Server.Data;

namespace TcNo_Acc_Switcher_Server.Shared;

public class MenuItem
{
    private static readonly Lang Lang = Lang.Instance;
    private string _text;
    public string Text { get => Lang[_text] ?? _text ; set => _text = value; }
    public string Content { get; set; }
    public List<MenuItem> Children = new();
}