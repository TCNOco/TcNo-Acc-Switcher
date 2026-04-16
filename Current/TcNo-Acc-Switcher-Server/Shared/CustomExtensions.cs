using System.Collections.Generic;
using System.Collections.ObjectModel;

namespace TcNo_Acc_Switcher_Server.Shared;

public static class ObservableCollectionExtensions {
    public static void AddRange<T>(this ObservableCollection<T> collection, IEnumerable<T> items)
    {
        foreach (var item in items)
        {
            collection.Add(item);
        }
    }
}