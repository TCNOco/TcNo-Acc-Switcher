// Selected Element on list, for use in other JS functions
var selectedElem = "";

// Returns "Steam" or "Riot" for example, based on the current URL
function getCurrentPage() {
    return (window.location.pathname.split("/")[0] !== "" ?
        window.location.pathname.split("/")[0] :
        window.location.pathname.split("/")[1]);
}

var observerAccOrPlatList, observerAccOShortcuts, observerShortcuts;

// Initialize context menus on navigation.
function onNavigateRefreshContextMenu() {
    console.log("onNavigate");
    // Disconnect existing observers, if any:
    if (observerAccOrPlatList != null) observerAccOrPlatList.disconnect();
    if (observerAccOShortcuts != null) observerAccOShortcuts.disconnect();
    if (observerShortcuts != null) observerShortcuts.disconnect();

    console.log("Reconnecting");
    // Reconnect
    observerAccOrPlatList = prepareObserver("#AccOrPlatList");
    observerAccOShortcuts = prepareObserver("#Shortcuts");
    observerShortcuts = prepareObserver("#Platform");
    console.log("Done!");
}

// Prepares and returns a new mutation observer for the requested element.
function prepareObserver(element) {
    console.log("Preparing observer for: ", element);
    const mut = new MutationObserver(function (mutations) {
        mutations.forEach(function (mutationRecord) {
            onContextShowReposition(element);
        });
    });
    const el = $(element)[0];
    if (el == null) return null;
    mut.observe(el, { attributes: true, attributeFilter: ["data-shown"] });
    return mut;
}

// On context mutate (show/hide) -> Reposition.
function onContextShowReposition(el) {
    const jq = $(el);
    if (jq.attr("data-shown") === "false") return;
    console.log("Shown, and repositioned!", el);
    //console.warn(mutationRecord);
    //console.trace();

}

function positionAndShowMenu(event, contextMenuId) {
    const jQueryContextMenu = $(contextMenuId);
    //Get window size:
    const winWidth = $(document).width();
    const winHeight = $(document).height();

    let posX = 0, posY = 0;
    if (typeof event !== "string") {
        //Get pointer position:
        posX = event.pageX - 14;
        posY = event.pageY - 42; // Offset for header bar
    } else {
        const el = $(event).parent();
        // Was called by a scripted or keyboard event:
        posX = el.offset().left + (el.width() / 2);
        posY = el.offset().top + (el.height() / 2);
    }

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
    });
    jQueryContextMenu.attr("data-shown", true);

    // Resize observer
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


    //Prevent browser default contextmenu.
    return false;
}

//Hide contextmenu
function hideContextMenus() {
    document.querySelectorAll(".contextmenu").forEach(menu => {
        const jq = $(menu);
        if (menu == null || jq.attr("data-shown") === "false") return;
        jq.attr("data-shown", false);
    });
}

document.querySelector("body").addEventListener("click", (e) => {
    const excluded = Array.from(document.querySelectorAll(
        ".contextmenu, .contextmenu *:not(li,a:not(.paginationButton)), .contextIgnoreClick"));
    if (!excluded.includes(e.target)) {
        hideContextMenus();
    }
});