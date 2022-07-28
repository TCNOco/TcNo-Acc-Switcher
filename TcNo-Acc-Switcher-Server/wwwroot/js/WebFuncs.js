var currentVersion = "";

// Returns "Steam" or "Riot" for example, based on the current URL
function getCurrentPage() {
    return (window.location.pathname.split("/")[0] !== "" ?
        window.location.pathname.split("/")[0] :
        window.location.pathname.split("/")[1]);
}

// Clear Cache reload:
var winUrl = window.location.href.split("?");
if (winUrl.length > 1 && winUrl[1].indexOf("cacheReload") !== -1) {
    history.pushState({}, null, window.location.href.replace("cacheReload&", "").replace("cacheReload", ""));
    location.reload(true);
}

// Take a string that is HTML escaped, and return a normal string back.
unEscapeString = (s) => s.replace("&lt;", "<").replace("&gt;", ">").replace("&#34;", "\"").replace("&#39;", "'").replace("&#47;", "/");


// Link handling
OpenLinkInBrowser = async(link) => await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "OpenLinkInBrowser", link);





var appendDelay = 100; // Milliseconds
var recentlyAppend = false;
var pendingQueue = {};

function queuedJQueryAppend(jQuerySelector, strToInsert) {
    if (recentlyAppend) {
        if (jQuerySelector in pendingQueue) pendingQueue[jQuerySelector] += strToInsert;
        else pendingQueue[jQuerySelector] = strToInsert;
    } else {
        recentlyAppend = true;
        setTimeout(flushJQueryAppendQueue, appendDelay);
        $(jQuerySelector).append([strToInsert]);
        // have this as detect and run at some point. For now the only use for this function is the Steam Cleaning list thingy
        $(".clearingRight")[0].scrollTop = $(".clearingRight")[0].scrollHeight;
    }
}

function flushJQueryAppendQueue() {
    for (const [key, value] of Object.entries(pendingQueue)) {
        $(key).append(value);
    }
    pendingQueue = {};
    recentlyAppend = false;
    // have this as detect and run at some point. For now the only use for this function is the Steam Cleaning list thingy
    $(".clearingRight")[0].scrollTop = $(".clearingRight")[0].scrollHeight;
}









// ---------- KEEPING FROM OLD ----------

window.addEventListener("load",
    () => {
        initTooltips();
    });

// Convert title="" into a hover tooltip.
function initTooltips() {
    // I don't know of an easier way to do this.
    $('[data-toggle="tooltip"]').tooltip();
    setTimeout(() => $('[data-toggle="tooltip"]').tooltip(), 1000);
    setTimeout(() => $('[data-toggle="tooltip"]').tooltip(), 2000);
    setTimeout(() => $('[data-toggle="tooltip"]').tooltip(), 4000);
}

// Figures out the best place for a tooltip and returns that location
// el MUST BE A JS VARIABLE
function getBestOffset(el) {
    // Because this can be placed below, and go off the screen.. Figure out where the element is.
    const parentPos = el[0].getBoundingClientRect();
    const parentWidth = el.width();

    const parentLeft = parentPos.left;
    const parentRight = parentLeft + parentWidth;

    // Because this can be placed right or below, and go off the screen.. Figure out where the element is.
    var bestOffset = "bottom";
    // Too close to sides -- Basically 1 account gap
    if (parentLeft < 100) bestOffset = "right";
    else if (screen.width - parentRight < 100) bestOffset = "left";
    return bestOffset;
}

// --------- FROM NEW SYSTEM ----------

// Focus on a specific element, like an input or button
focusOn = async (element) => $(element).focus();

// Remove existing highlighted elements, if any.
function clearAccountTooltips() {
    $(".currentAcc").each((_, e) => {
        var j = $(e);
        j.removeClass("currentAcc");
        j.parent().removeAttr("title").removeAttr("data-original-title").removeAttr("data-placement");
    });
}

// Sets the best data-placement attribute for the requested account
function setBestOffset(element) {
    const parentEl = $(`[for='${element}']`).parent();
    parentEl.attr("data-placement", getBestOffset(parentEl));
}

// Show hover tooltips for account notes.
async function showNoteTooltips() {
    const noteArr = $(".acc_note").toArray();
    if (noteArr.length === 0) return;

    await noteArr.forEach((e) => {
        var j = $(e);
        var note = j.text();
        var parentEl = j.parent().parent();
        parentEl.removeAttr("title").removeAttr("data-original-title").removeAttr("data-placement");
        parentEl.attr("title", note);
        parentEl.attr("data-placement", getBestOffset(parentEl));
    });
    initTooltips();
}

// Scrolls the path picker modal window to the last selected element - For pasting in paths, etc.
var lastPickerText = "";
function pathPickerScrollToElement()
{
    try {
        const curText = $(".selected-path").last().text();
        if (lastPickerText === curText) return;
        lastPickerText = curText;
        $(".pathPicker").animate({
            scrollTop: $(".selected-path").last().offset().top - ($(".pathPicker").offset().top * 1.5)
        }, 500);
    } catch (e) {
        // Do nothing
    }
}

// Start downloading a file, with specified filename.
function saveFile(fileName, urlFile) {
    const a = document.createElement("a");
    a.style = "display: none";
    document.body.appendChild(a);
    a.href = urlFile;
    a.download = fileName;
    a.click();
    a.remove();
}


var windowDimensionsFound = false;
let windowBorderWidth = 7;
let windowHeaderbarHeight = 32;


function repositionFindBorders() {
    if (windowDimensionsFound) return;
    windowBorderWidth = $(".resizeLeft")[0].getBoundingClientRect().width;
    windowHeaderbarHeight = $(".headerbar")[0].getBoundingClientRect().height;
    windowDimensionsFound = true;
}

function repositionTooltip(id) {
    const tooltipSpan = $("#" + id);
    // Only reposition once
    console.log("Moving", id);
    // Get window size and remove modified position (if any)
    repositionFindBorders();
    tooltipSpan.removeClass().css("left", "").css("bottom", "").css("transform", "");
    const bounds = tooltipSpan[0].getBoundingClientRect();
    const parentBounds = tooltipSpan.parent()[0].getBoundingClientRect();

    // The tooltip is in the top-left of its parent.
    const xToCenter = (parentBounds.width - bounds.width) / 2;

    // Space above?
    const spaceT = parentBounds.top - bounds.height > windowHeaderbarHeight;
    // And space when centered?
    const middleParentX = parentBounds.x + (parentBounds.width / 2);
    const middleParentY = parentBounds.y + (parentBounds.height / 2);
    const spaceLCentered = middleParentX - (bounds.width / 2) > windowBorderWidth;
    const spaceRCentered = middleParentX + (bounds.width / 2) < innerWidth - windowBorderWidth;
    if (spaceLCentered && spaceRCentered) {
        // Space on both sides when centered.
        if (spaceT) {
            // Space is available above for the full tooltip, and space on sides.
            // Therefore: Place centered above.
            tooltipSpan.css("transform", `translate(${xToCenter}px, -100%)`);
            return;
        }

        const spaceB = parentBounds.top + parentBounds.height + bounds.height < innerHeight - 60;
        if (spaceB) {
            // Space is available below for the full tooltip, and space on sides.
            // Therefore: Place centered below.
            tooltipSpan.css("transform", `translate(${xToCenter}px, ${parentBounds.height}px)`);
            tooltipSpan.removeClass().addClass("tooltip-wrapper-bottom");
            return;
        }
    }

    // Space not available to have centered above or below.
    // Therefore: Pick a side.
    const middleParentHeight = (parentBounds.height / 2) - (bounds.height / 2);
    const spaceR = parentBounds.x + parentBounds.width + bounds.width < innerWidth - windowBorderWidth;
    if (spaceR) {
        tooltipSpan.css("transform", `translate(${parentBounds.width}px, ${middleParentHeight}px)`);
        tooltipSpan.removeClass().addClass("tooltip-wrapper-right");
        return;
    }

    const spaceL = parentBounds.x - bounds.width > windowBorderWidth;
    if (spaceL) {
        tooltipSpan.css("transform", `translate(${-bounds.width}px, ${middleParentHeight}px)`);
        tooltipSpan.removeClass().addClass("tooltip-wrapper-left");
    }
}


// Basic account switcher: Shortcut dropdown
function sDropdownReposition() {
    const dropDownContainer = $("#shortcutDropdown");
    const btn = $("#shortcutDropdownBtn");
    const dropDownItemsContainer = $(".shortcutDropdownItems")[0];
    const btnPos = btn[0].getBoundingClientRect();
    dropDownContainer.css({ top: btnPos.top - dropDownContainer.height() - btn.height() - 16, left: btnPos.left + 16 - (dropDownContainer.width() / 2) });

    // If overflowing - Widen by scrollbar width to prevent weird overflow gap on side
    if (checkOverflow(dropDownItemsContainer) && dropDownContainer[0].style.minWidth === "") {
        const scrollbarWidth = (dropDownItemsContainer.offsetWidth - dropDownItemsContainer.clientWidth);
        const hasContextMenu = $(".HasContextMenu");
        if (hasContextMenu.length > 0) {
            const computedStyle = window.getComputedStyle($(".HasContextMenu")[0]);
            const computedStyleContainer = window.getComputedStyle($("#shortcutDropdown")[0]);
            const marginX = parseInt(computedStyle.marginLeft) + parseInt(computedStyle.marginRight);
            const marginY = parseInt(computedStyle.marginBottom) + parseInt(computedStyle.marginTop);
            const paddingX = parseInt(computedStyleContainer.paddingLeft) + parseInt(computedStyleContainer.paddingRight);
            const paddingY = parseInt(computedStyleContainer.paddingTop) + parseInt(computedStyleContainer.paddingBottom);
            dropDownContainer.css({
                minWidth: $(dropDownItemsContainer).width() + scrollbarWidth + marginX + paddingX
            });
            $(dropDownItemsContainer).css({
                maxHeight: dropDownContainer.height() - paddingY + marginY
            });
        }
    }
}

// Try init shortcut dropdown & sortable.
function sDropdownInit() {
    // Create sortable list
    try {
        // Check if init
        sortable(".shortcuts, .shortcutDropdownItems", "enable");
    } catch (e) {
        // Else: init.
        sortable(".shortcuts, .shortcutDropdownItems", {
            connectWith: "shortcutJoined",
            forcePlaceholderSize: true,
            placeholderClass: "shortcutPlaceholder",
            items: ":not(#btnOpenShortcutFolder)",
            customDragImage: (draggedElement, elementOffset, event) => {
                // Set the dragged element to the button, not the tooltip.
                return {
                    element: draggedElement.getElementsByTagName("button")[0] ?? draggedElement,
                    posX: event.pageX - elementOffset.left,
                    posY: event.pageY - elementOffset.top
                }
            }
        });

        // Add event handlers
        $(".shortcuts, .shortcutDropdownItems").toArray().forEach(el => {
            // ReSharper disable once Html.EventNotResolved
            el.addEventListener("sortstart", function () {
                $(".shortcuts").addClass("expandShortcuts");
            });
            // ReSharper disable once Html.EventNotResolved
            el.addEventListener("sortstop", function (e) {
                $(".shortcuts").removeClass("expandShortcuts");
                sDropdownReposition();
                $(e.detail.item).show(); // Sometimes items just randomly disappear? what?
                shortcutsChanged = true;
            });
        });
    }
}

// Returns true if element passed in has a scrollbar or is overflowing.
// https://stackoverflow.com/a/143889
function checkOverflow(el) {
    const curOverflow = el.style.overflow;

    if (!curOverflow || curOverflow === "visible")
        el.style.overflow = "hidden";

    const isOverflowing = el.clientWidth < el.scrollWidth
        || el.clientHeight < el.scrollHeight;

    el.style.overflow = curOverflow;

    return isOverflowing;
}

// Shows/Hides shortcut dropdown element, also initializes sortable.
function shortcutDropdownBtnClick() {
    if (!$("#shortcutDropdown").is(":visible")) {
        sDropdownInit();
        $("#shortcutDropdown").show();
        sDropdownReposition();
        $("#shortcutDropdownBtn").addClass("flip");
        // If has no children in main list, add the expandShortcuts CSS to show users they can drag.
        if ($(".shortcuts button").length === 0) {
            $(".shortcuts").addClass("expandShortcuts");
        }
    } else {
        serializeShortcuts();
        $("#shortcutDropdown").hide();
        $("#shortcutDropdownBtn").removeClass("flip");
        $(".shortcuts").removeClass("expandShortcuts");
    }
}

// Saves order of all shortcuts.
let shortcutsChanged = false;
async function serializeShortcuts() {
    var output = {};
    // Serialize highlighted items
    var numHighlightedShortcuts = $(".shortcuts button").children().length;
    $(".shortcuts button").each((i, e) => output[i - numHighlightedShortcuts] = $(e).attr("id"));

    // Serialize dropdown items
    $(".shortcutDropdownItems button").each((i, e) => {
        if ($(e).attr("id") === "btnOpenShortcutFolder") return;
        output[i] = $(e).attr("id");
    });

    if (!shortcutsChanged) return;
    await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "JsSaveShortcuts", output);
    shortcutsChanged = false;
}

// Adds save hotkey to settings page.
async function initSavingHotKey() {
    hotkeys("ctrl+s", async function (event) {
        event.preventDefault();
        DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "JsSaveSettings");
    });
}