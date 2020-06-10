using System;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.ComponentModel;
using System.Diagnostics;
using System.Drawing;
using System.IO;
using System.Linq;
using System.Text;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Data;
using System.Windows.Documents;
using System.Windows.Input;
using System.Windows.Media;
using System.Windows.Media.Imaging;
using System.Windows.Shapes;
using TcNo_Acc_Switcher;
using Color = System.Windows.Media.Color;

namespace TcNo_Acc_Switcher
{
    /// <summary>
    /// Interaction logic for PlatformPicker.xaml
    /// </summary>
    public partial class MainWindow : Window
    {
        public MainWindow()
        {
            // Bypass this platform picker launcher for now.
            // This keeps shortcuts working, and can be repalced with an update.
            Process.Start("TcNo Account Switcher Steam.exe");
            Environment.Exit(1);

            InitializeComponent();



            //List<Platform> Platforms = new List<Platform>();
            //Platforms.Add(new Platform() { Icon = BitmapToImageSource(Properties.Resources.Origin), Name = "Steam" }); 

            //// DISABLE LISTBOX
            //foreach (Platform pl in Platforms)
            //{
            //    MainViewmodel.Platforms.Add(pl);
            //}
            //ListPlatforms.Items.Add(Platforms[0]);
        }

        //public class Platform
        //{
        //    public string Name { get; set; }
        //    public BitmapImage Icon { get; set; }
        //}
        //BitmapImage BitmapToImageSource(Bitmap bitmap)
        //{
        //    using (MemoryStream memory = new MemoryStream())
        //    {
        //        bitmap.Save(memory, System.Drawing.Imaging.ImageFormat.Bmp);
        //        memory.Position = 0;
        //        BitmapImage bitmapimage = new BitmapImage();
        //        bitmapimage.BeginInit();
        //        bitmapimage.StreamSource = memory;
        //        bitmapimage.CacheOption = BitmapCacheOption.OnLoad;
        //        bitmapimage.EndInit();

        //        return bitmapimage;
        //    }
        //}

        // Window interaction
        private void btnExit(object sender, RoutedEventArgs e)
        {
            this.Close();
        }

        private void btnMinimize(object sender, RoutedEventArgs e)
        {
            this.WindowState = WindowState.Minimized;
        }

        private void dragWindow(object sender, MouseButtonEventArgs e)
        {
            if (e.LeftButton == MouseButtonState.Pressed)
            {
                this.DragMove();
            }
        }

        private void Window_Closing(object sender, CancelEventArgs e)
        {

        }
        private void Window_Closed(object sender, EventArgs e)
        {
            Environment.Exit(1);
        }

        private void PlatformSelect(object sender, SelectionChangedEventArgs e)
        {
            var item = (ListBox)sender;
            var su = (ListBoxItem)item.SelectedItem;
            su.Background = new SolidColorBrush(Color.FromArgb(255, 21, 21, 30));
        }
        private void ListBoxItem_MouseDoubleClick(object sender, MouseButtonEventArgs e)
        {
            MessageBox.Show("Program selected");
        }

        private void ListPlatforms_OnPreviewMouseUp(object sender, MouseButtonEventArgs e)
        {
            foreach (ListBoxItem listPlatformsItem in ListPlatforms.Items)
            {
                listPlatformsItem.Background = new SolidColorBrush() { Color = Color.FromArgb(255, 40, 41, 58) };
            }
        }

        private void PickerSteam(object sender, MouseButtonEventArgs e)
        {
            // Check if Steam is in the default location
            // Otherwise
            // Check if Steam is running, and get the .exe location
            // Otherwise
            // Ask the user where Steam is
            this.Hide();
        }

        private void PickerOrigin(object sender, MouseButtonEventArgs e)
        {
            // Check if Origin is in the default location
            // Otherwise
            // Check if Origin is running, and get the .exe location
            // Otherwise
            // Ask the user where Steam is
            this.Hide();
        }
    }
}
