namespace TcNo_Acc_Switcher_Server.State.DataTypes
{
    public enum ExtraArg
    {
        None,
        RestartAsAdmin,
        ClearStats,
        ForgetAccount
    }

    public enum StatsSelectorState
    {
        GamesList,
        VarsList
    }
    public enum TextInputGoal
    {
        AppPassword,
        AccString,
        ChangeUsername
    }

    public enum PathPickerElement
    {
        None,
        File,
        Folder
    }

    public enum PathPickerGoal
    {
        FindPlatformExe,
        SetBackground,
        SetUserdata,
        SetAccountImage
    }
}
