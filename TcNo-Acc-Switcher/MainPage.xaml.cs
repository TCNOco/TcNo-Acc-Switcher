using Microsoft.AspNetCore.Components.WebView;
using Windows.ApplicationModel.Core;
using Microsoft.UI.Xaml.Controls;
using TcNo_Acc_Switcher_Globals;
using Microsoft.Web.WebView2.Core;
using WinRT;

namespace TcNo_Acc_Switcher
{
    public partial class MainPage : ContentPage
    {
        // For some VERY annoying reason this exists in BOTH Microsoft.Web.WebView2.Core and Microsoft.WinUI.
        private WebView2 _webView;
        private CoreWebView2 _coreWebView;

        // For draggable regions:
        // https://github.com/MicrosoftEdge/WebView2Feedback/issues/200
        private void BlazorViewOnBlazorWebViewInitialized(object sender, BlazorWebViewInitializedEventArgs e)
        {
            _webView = e.WebView;
            _coreWebView = _webView.CoreWebView2;
            _webView.NavigationCompleted += WebViewOnNavigationCompleted;
        }

        private async void WebViewOnNavigationCompleted(WebView2 sender, CoreWebView2NavigationCompletedEventArgs args)
        {
            await sender.EnsureCoreWebView2Async();
            var eventForwarder = new EventForwarder(WindowActions.HWnd);
            var interop = _coreWebView.As<ICoreWebView2Interop>();
            interop.AddHostObjectToScript("eventForwarder", eventForwarder);
            
            
            //_coreWebView.AddHostObjectToScript("eventForwarder", eventForwarder);
            _webView.NavigationCompleted -= WebViewOnNavigationCompleted;
        }


        public MainPage()
        {
            InitializeComponent();
        }



        private void BlazorWebView_OnUrlLoading(object sender, UrlLoadingEventArgs e)
        {
            Console.Write("blazorView");
            //var coreTitleBar = CoreApplication.GetCurrentView().TitleBar;
            //coreTitleBar.IsVisible = false;
        }
    }
}