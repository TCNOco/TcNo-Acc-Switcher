using System.Threading;
using System.Threading.Tasks;

namespace TcNo_Acc_Switcher_Server.Shared.Toast
{
    public class Toast
    {
        public ToastType Type { get; set; }
        public string Title { get; set; }
        public string Message { get; set; }
        public int DuplicateCount { get; set; }
        public Task RemoveSelf { get; set; }
        public CancellationTokenSource CancellationSource { get; set; } = new CancellationTokenSource();
        public CancellationToken Cancellation { get; set; }

        public Toast(ToastType type, string title = "", string message = "")
        {
            Type = type;
            Title = title;
            Message = message;
            DuplicateCount = 0;
        }
    }

    public enum ToastType
    {
        Success,
        Info,
        Warning,
        Error,
    }
}
