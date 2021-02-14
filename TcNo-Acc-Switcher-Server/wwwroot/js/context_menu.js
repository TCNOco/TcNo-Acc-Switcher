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
    // Ready accounts for double-click
    $(".acc").dblclick(function () {
        SwapTo();
    });

    //Show contextmenu on Right-Click:
    $(".acc").contextmenu(function(e) {
        // Select item that was right-clicked.
        $(e.currentTarget).click();
        //$('[id="' + e.currentTarget.for + '"]').click();
        //console.log($('[id="' + $(e.currentTarget).attr("for") + '"]'));


        // Set currently selected element
        SelectedElem = $('[id="' + $(e.currentTarget).attr("for") + '"]')[0];
        // Update status for element
        $("#CurrentStatus").val("Selected: " + $(SelectedElem).attr("Line2"));


        //#region POSITIONING OF CONTEXT MENU
        //Get window size:
        var winWidth = $(document).width();
        var winHeight = $(document).height();
        //Get pointer position:
        var posX = e.pageX;
        var posY = e.pageY;
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
    $("#CurrentStatus").val("Selected: " + $("input[name=accounts]:checked").attr("Line2"));
}