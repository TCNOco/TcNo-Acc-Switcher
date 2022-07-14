// Selected Element on list, for use in other JS functions
var selectedElem = "";

// Returns "Steam" or "Riot" for example, based on the current URL
function getCurrentPage() {
    return (window.location.pathname.split("/")[0] !== "" ?
        window.location.pathname.split("/")[0] :
        window.location.pathname.split("/")[1]);
}

function positionAndShowMenu(event, contextMenuId) {
    const jQueryContextMenu = $(contextMenuId);
    //Get window size:
    const winWidth = $(document).width();
    const winHeight = $(document).height();
    //Get pointer position:
    const posX = event.pageX - 14;
    const posY = event.pageY - 42; // Offset for header bar
    //Get contextmenu size:
    const menuWidth = jQueryContextMenu.width();
    const menuHeight = jQueryContextMenu.height();
    // Header offset + 10:
    const hOffset = 42;
    // Prevent page overflow:
    const xOverflow = posX + menuWidth + hOffset - winWidth;
    const yOverflow = posY + menuHeight + hOffset - winHeight;
    const posLeft = posX + (xOverflow > 0 ? - menuWidth : 10) + "px";
    const posTop = posY + (yOverflow > 0 ? - yOverflow : 10) + "px";

    //Display contextmenu:
    jQueryContextMenu.css({
        "left": posLeft,
        "top": posTop
    }).show();

    //Prevent browser default contextmenu.
    return false;
}

async function initContextMenu() {
    let group = "acc";
    if (getCurrentPage() === "") {
        group = "platform";
    }
    if (getCurrentPage() === "Settings") {
        group = "preview";
    }

    // Show shortcut contextmenu on Right-Click
    $(`.HasContextMenu`).contextmenu((e) => positionAndShowMenu(e, "#Shortcuts"));

    //Show platform/account contextmenu on Right-Click:
    //$(`.${group}_list_item`).contextmenu((e) => positionAndShowMenu(e, "#AccOrPlatList"));

    //Show contextmenu on Right-Click (platform shortcut):
    $(`#btnStartPlat`).contextmenu((e) => positionAndShowMenu(e, "#Platform"));

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
};

//Hide contextmenu
function hideContextMenus() {
    document.querySelectorAll(".contextmenu").forEach(menu => {
        if (menu == null) return;
        menu.style["display"] = "none";
    });
}
document.querySelector("body").addEventListener("click", (e) => {
    const excluded = Array.from(document.querySelectorAll(
        ".contextmenu, .contextmenu *:not(li,a:not(.paginationButton))"));
    if (!excluded.includes(e.target)) {
        hideContextMenus();
    }
});