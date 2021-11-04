using CefSharp;
using CefSharp.Enums;
using System;
using System.Collections.Generic;
using System.Drawing;

namespace TcNo_Acc_Switcher_Client_CEF
{
    public class DragDropHandler : IDragHandler
    {
        public Region DraggableRegion = new Region();
        public event Action<Region> RegionsChanged;

        public bool OnDragEnter(IWebBrowser browserControl, IBrowser browser, IDragData dragData, DragOperationsMask mask)
        {
            return false;
        }

        public void OnDraggableRegionsChanged(IWebBrowser browserControl, IBrowser browser, IFrame frame, IList<DraggableRegion> regions)
        {
            if (browser.IsPopup) return;

            DraggableRegion = null;
            if (regions.Count > 0)
            {
                foreach (var region in regions)
                {
                    var rect = new Rectangle(region.X, region.Y, region.Width, region.Height);

                    if (DraggableRegion == null)
                        DraggableRegion = new Region(rect);
                    else
                    {
                        if (region.Draggable)
                            DraggableRegion.Union(rect);
                        else
                            DraggableRegion.Exclude(rect);
                    }
                }
            }

            RegionsChanged?.Invoke(DraggableRegion);
        }

        public void Dispose()
        {
            RegionsChanged = null;
        }
    }
}
