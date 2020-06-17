using System;
using System.Globalization;
using System.Windows.Data;

namespace TcNo_Acc_Switcher_Steam
{
    public class IntToStringConverter : IValueConverter
    {
        public object Convert(object value, Type targetType, object parameter, CultureInfo culture) => ((int)value).ToString();

        public object ConvertBack(object value, Type targetType, object parameter, CultureInfo culture) => int.Parse((string)value);
    }
}