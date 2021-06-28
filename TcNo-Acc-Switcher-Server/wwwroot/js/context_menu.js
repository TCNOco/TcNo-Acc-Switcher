// Selected Element on list, for use in other JS functions
var selectedElem = "";

function initContextMenu() {
    let group = "acc";
    if (getCurrentPage() === "") {
        group = "platform";
    }
    $(`.${group}_list`).on("click", () => {
        $("input:checked").each((_, e) => {
            $(e).prop("checked", false);
        });
    });

    if (group === "acc") {
        // Ready accounts for double-click
        $(".acc_list_item").dblclick((event) => {
            swapTo(-1, event);
        });

        // Handle Left-clicks:
        $(".acc_list_item").click((e) => {
            $(e.currentTarget).children("input")[0].click();
            e.stopPropagation();
        });
    }
    
    //Show contextmenu on Right-Click:
    $(`.${group}_list_item`).contextmenu((e) => {
        if (group === "acc") {
            // Select item that was right-clicked.
            $(e.currentTarget).children("input").click();

            // Set currently selected element
            selectedElem = $(e.currentTarget).children("input")[0];

            // Update status for element
            switch (getCurrentPage()) {
            case "Steam":
                updateStatus(`Selected: ${$(selectedElem).attr("Line2")}`);
                break;
            case "Origin":
                updateStatus(`Selected: ${$(selectedElem).attr("id")}`);
                break;
            case "Ubisoft":
                updateStatus(`Selected: ${$(selectedElem).attr("Username")}`);
                break;
            default:
                break;
            }
        } else if (group === "platform") {
            // Set currently selected element
            selectedElem = $(e.currentTarget).prop("id").substr(8);
        }
        
        //Get window size:
        const winWidth = $(document).width();
        const winHeight = $(document).height();
        //Get pointer position:
        const posX = e.pageX - 14;
        const posY = e.pageY - 42; // Offset for header bar
        //Get contextmenu size:
        const menuWidth = $(".contextmenu").width();
        const menuHeight = $(".contextmenu").height();

        // Header offset + 10:
        const hOffset = 42;
        // Prevent page overflow:
        const xOverflow = posX + menuWidth + hOffset - winWidth;
        const yOverflow = posY + menuHeight + hOffset - winHeight;
        var posLeft = posX + (xOverflow > 0 ? - menuWidth : 10) + "px";
        var posTop = posY + (yOverflow > 0 ? - yOverflow : 10) + "px";

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
    var rightSpace = 0, bottomSpace = 0;

    function moveContextMenu(submenu) {
        //rightSpace = $(contextMenu).position().left + $(contextMenu).width() + submenuSize - window.innerWidth;
        rightSpace = $(submenu).offset().left + $(submenu).width() - window.innerWidth;
        bottomSpace = $(submenu).offset().top + $(submenu).height() - window.innerHeight;

        if (rightSpace > 0) {
            $(contextMenu).css("left", $(contextMenu).position().left - rightSpace - 40);
        }
        if (bottomSpace > 0) {
            $(contextMenu).css("top", $(contextMenu).position().top - bottomSpace - 10);
        }
    }

    // Define resizeObserver and add it to every submenu on the page.
    const resizeObserver = new ResizeObserver((entries) => {
        for (let entry of entries) {
            moveContextMenu(entry.target);
        }
    });
    for (let item of document.getElementsByClassName("submenu1")) {
	    resizeObserver.observe(item);
	} for (let item of document.getElementsByClassName("submenu2")) {
	    resizeObserver.observe(item);
    }

    //Hide contextmenu:
    $(document).click(() => {
        $(".contextmenu").hide();
    });
};

function selectedItemChanged() {
    // Different function groups based on platform
    updateStatus(`Selected: ${$("input[name=accounts]:checked").attr("DisplayName")}`);
}