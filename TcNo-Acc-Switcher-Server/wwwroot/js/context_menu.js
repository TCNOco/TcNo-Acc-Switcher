//// Loader
//var Namespace = Namespace || {};
//Namespace.Deferred = function () {
//    var functions = [];
//    var timer = function () {
//        if (window.jQuery && window.jQuery.ui) {
//            while (functions.length) {
//                functions.shift()(window.jQuery);
//            }
//        } else {
//            window.setTimeout(timer, 250);
//        }
//    };
//    timer();
//    return {
//        execute: function (onJQueryReady) {
//            if (window.jQuery && window.jQuery.ui) {
//                onJQueryReady(window.jQuery);
//            } else {
//                functions.push(onJQueryReady);
//            }
//        }
//    };
//}();

//Namespace.Deferred.execute(addContextMenu);

//function addContextMenu() {

// Selected Element on list, for use in other JS functions
var SelectedElem = "";

function initContextMenu() {
    // Handle currentpage variable.
    currentpage = (window.location.pathname.split("/")[0] !== ""
        ? window.location.pathname.split("/")[0]
        : window.location.pathname.split("/")[1]);

    $(".acc_list").on("click", function () {
        console.log('e');
        $("input:checked").each(function (_, e) {
            $(e).prop("checked", false);
        });
    });

    // Ready accounts for double-click
    $(".acc_list_item").dblclick(function (event) {
        SwapTo(-1, event);
    });

    // Handle Left-clicks:
    $(".acc_list_item").click(function (e) {
        $(e.currentTarget).children('input')[0].click();
        e.stopPropagation();
    });

    //Show contextmenu on Right-Click:
    $(".acc_list_item").contextmenu(function(e) {
        // Select item that was right-clicked.
        $(e.currentTarget).children('input').click();
        //$('[id="' + e.currentTarget.for + '"]').click();
        //console.log($('[id="' + $(e.currentTarget).attr("for") + '"]'));


        // Set currently selected element
        SelectedElem = $(e.currentTarget).children('input')[0];
        // Update status for element
        switch (currentpage) {
            case "Steam":
                updateStatus("Selected: " + $(SelectedElem).attr("Line2"));
                break;
            case "Origin":
                updateStatus("Selected: " + $(SelectedElem).attr("id"));
                break;
            case "Ubisoft":
                updateStatus("Selected: " + $(SelectedElem).attr("Username"));
                break;
            default:
                break;
        }


        //#region POSITIONING OF CONTEXT MENU
        //Get window size:
        var winWidth = $(document).width();
        var winHeight = $(document).height();
        //Get pointer position:
        var posX = e.pageX - 14;
        var posY = e.pageY - 42; // Offset for header bar
        //Get contextmenu size:
        var menuWidth = $(".contextmenu").width();
        var menuHeight = $(".contextmenu").height();
        //Security margin:
        var secMargin = 10;
        //Prevent page overflow:
        if (posX + menuWidth + secMargin >= winWidth &&
          posY + menuHeight + secMargin >= winHeight) {
          //Case 1: right-bottom overflow:
          posLeft = posX - menuWidth - secMargin + "px";
          posTop = posY - menuHeight - secMargin + "px";
        } else if (posX + menuWidth + secMargin >= winWidth) {
          //Case 2: right overflow:
          posLeft = posX - menuWidth - secMargin + "px";
          posTop = posY + secMargin + "px";
        } else if (posY + menuHeight + secMargin >= winHeight) {
          //Case 3: bottom overflow:
          posLeft = posX + secMargin + "px";
          posTop = posY - menuHeight - secMargin + "px";
        } else {
          //Case 4: default values:
          posLeft = posX + secMargin + "px";
          posTop = posY + secMargin + "px";
        };
            //Display contextmenu:
            $(".contextmenu").css({
              "left": posLeft,
              "top": posTop
            }).show();
            //Prevent browser default contextmenu.
            return false;
        });
    
    // Check element fits on page, and move if it doesn't
    // This function moves the element to the left if it doesn't fit.
    var contextMenu = document.getElementsByClassName("contextmenu")[0]; 
    var rightSpace = 0;
    function moveContextMenu(submenuSize) {
        rightSpace = $(contextMenu).position().left + $(contextMenu).width() + submenuSize - window.innerWidth;
        if (rightSpace > 0){
            $(contextMenu).css('left', $(contextMenu).position().left - rightSpace);
        }
    }
    // Define resizeObserver and add it to every submenu on the page.
    var resizeObserver = new ResizeObserver(entries => {
      for (let entry of entries) {
        moveContextMenu(entry.contentRect.width);
      }
    });
    for (let item of document.getElementsByClassName("submenu")) {
        resizeObserver.observe(item);
    }
    //#endregion
    
    //Hide contextmenu:
    $(document).click(function () {
        $(".contextmenu").hide();
    });
};

function SelectedItemChanged() {
    //console.log("click!");
    //console.log(this);
    // Different function groups based on platform
    switch (currentpage) {
        case "Steam":
            updateStatus("Selected: " + $("input[name=accounts]:checked").attr("Line2"));
            break;
        case "Origin":
            updateStatus("Selected: " + $("input[name=accounts]:checked").attr("id"));
            break;
        case "Ubisoft":
            updateStatus("Selected: " + $("input[name=accounts]:checked").attr("Username"));
            break;
        default:
            break;
    }
}