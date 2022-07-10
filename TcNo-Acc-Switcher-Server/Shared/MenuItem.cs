using System.Collections.Generic;

namespace TcNo_Acc_Switcher_Server.Shared;

public class MenuItem
{
    public string Text { get; set; }
    public string Content { get; set; }
    public List<MenuItem> Children = new();
}