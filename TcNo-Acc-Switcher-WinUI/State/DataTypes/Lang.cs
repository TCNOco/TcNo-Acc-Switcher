namespace TcNo_Acc_Switcher.State.DataTypes
{
    public class LangSub
    {
        public string LangKey { get; set; }
        public object Variable { get; set; }

        public LangSub(string key, object var)
        {
            LangKey = key;
            Variable = var;
        }
    }
}
