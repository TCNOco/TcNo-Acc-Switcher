namespace TcNo_Acc_Switcher_Server.State.Classes.GameStats;

public class CollectInstruction
{
    public string XPath { get; set; }
    public string Select { get; set; }
    public string DisplayAs { get; set; } = "%x%";
    public string ToggleText { get; set; } = "";

    // Optional
    public string SelectAttribute { get; set; } = ""; // If Select = "attribute", set the attribute to get here.
    public string SpecialType { get; set; } = ""; // Possible types: ImageDownload.
    public string NoDisplayIf { get; set; } = ""; // The DisplayAs text will not display if equal to the default value of this.
    public string Icon { get; set; } = ""; // Icon HTML markup
}