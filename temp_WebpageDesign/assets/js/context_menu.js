$(document).ready(function() {
  //Show contextmenu:
  $(".acc").contextmenu(function(e) {
    // Select item that was right-clicked.
    $(e.currentTarget).click();
    //$('[id="' + e.currentTarget.for + '"]').click();
    //console.log($('[id="' + $(e.currentTarget).attr("for") + '"]'));
      
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
    //Hide contextmenu:
    $(document).click(function() {
        $(".contextmenu").hide();
    });
    
    // Check element fits on page, and move if it doesn't
    // This function moves the element to the left if it doesn't fit.
    var contextMenu = document.getElementsByClassName("contextmenu")[0]; 
    var rightSpace = 0;
    function moveContextMenu(submenu_size) {
        rightSpace = $(contextMenu).position().left + $(contextMenu).width() + submenu_size - window.innerWidth;
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
});