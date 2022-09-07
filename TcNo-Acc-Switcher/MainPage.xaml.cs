using Microsoft.AspNetCore.Components.WebView;
using Windows.ApplicationModel.Core;

namespace TcNo_Acc_Switcher
{
    public partial class MainPage : ContentPage
    {
        public MainPage()
        {
            InitializeComponent();
        }

        private void BlazorWebView_OnUrlLoading(object sender, UrlLoadingEventArgs e)
        {
            Console.Write("TEST");
            //var coreTitleBar = CoreApplication.GetCurrentView().TitleBar;
            //coreTitleBar.IsVisible = false;
        }
    }
}