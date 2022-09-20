// ReSharper disable Html.EventNotResolved
if (sortable == undefined) {
	window.notification.new({
		type: "error",
        title: "",
		message: "A critical component could not be loaded (sorter). Please restart the application!",
		renderTo: "toastarea",
		duration: 10000
	});
	sortable = null;
}

function jQueryAppend(jQuerySelector, strToInsert) {
	$(jQuerySelector).append(strToInsert);
}

// Removes arguments like "?toast_type, &toast_title, &toast_message" from the URL.
function removeUrlArgs(argString) {
    const toRemove = argString.split(",");
	let url = window.location.href;
	if (url.indexOf("?") !== -1) {
		const parts = url.split("?");
		url = parts[0];
		const args = parts[1];
		let outArgs = "?";
		if (args.indexOf("&") !== -1) {
			args.split("&").forEach((i) => {
				if (i.indexOf("=") !== -1) {
					const key = i.split("=")[0];
					const val = i.split("=")[1];
					if (!toRemove.includes(key))
						outArgs += key + "=" + val + "&";
				} else {
					if (!toRemove.includes(i))
						outArgs += i + "&";
				}
			});
		}
		url += outArgs.slice(0, -1); // Remove last '&' or first '?'
	}
    history.pushState({}, null, url);
}

async function initPlatformListSortable() {
    // Create sortable list
    sortable(".platform_list", {
        forcePlaceholderSize: true,
        placeholderClass: "placeHolderPlat",
        items: ":not(toastarea)"
    });

    // On drag end, save list of items.
    sortable(".platform_list")[0].addEventListener("sortupdate", () => {
        var order = [];
        $(".platform_list > div").each((i, e) => { order.push(e.getAttribute("safeName")); });

        DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiSaveOrder", JSON.stringify(order));
    });

    // On drag end
    sortable(".platform_list")[0].addEventListener("sortstop", (e) => {
        $(e.detail.item).show(); // Sometimes items just randomly disappear? what?
    });
}

async function initAccListSortableDelayed() {
    setTimeout(() => {
        initAccListSortable();
    }, 1000);
}

async function initAccListSortable() {
    accountListCenterCheck();

	if (document.getElementsByClassName("acc_list").length === 0) return;
    // Create sortable list
    sortable(".acc_list", {
        forcePlaceholderSize: true,
        placeholderClass: "placeHolderAcc",
        items: ":not(toastarea)",
        customDragImage: (draggedElement, elementOffset, event) => {
            // Set the dragged element to the button, not the tooltip.
            return {
                element: draggedElement.getElementsByTagName("div")[0] ?? draggedElement,
                posX: event.pageX - elementOffset.left,
                posY: event.pageY - elementOffset.top
            }
        }
    });
    // On drag start, un-select all items.
    sortable(".acc_list")[0].addEventListener("sortstart", () => {
        $("input:checked").each((_, e) => {
            $(e).prop("checked", false);
        });
    });

    // On drag end, save list of items.
    sortable(".acc_list")[0].addEventListener("sortupdate", (e) => {
        const order = [];
        e.detail.destination.items.forEach((i) => {
            if (!$(i).is("div")) return; // Ignore <toastarea class="toastarea" />
            order.push(i.getElementsByTagName("input")[0].getAttribute("id"));
        });

        DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiSaveOrder", JSON.stringify(order));
    });

    // On drag end
    sortable(".acc_list")[0].addEventListener("sortstop", (e) => {
        $(e.detail.item).show(); // Sometimes items just randomly disappear? what?
    });
}

// If number of rows in accounts grid is > 1, justify items center to prevent odd spacing on right.
function accountListCenterCheck() {
    if (window.getComputedStyle($(".acc_list")[0]).getPropertyValue("grid-template-rows").split(" ").length > 1)
        $(".acc_list").css("justify-content", "center");
    else
        $(".acc_list").css("justify-content", "");
}

// Timeout so window resize is only run after 100ms passes -- Stops constant running while resizing
var windowResizedTimeout;
window.onresize = function () {
    clearTimeout(windowResizedTimeout);
    windowResizedTimeout = setTimeout(function () {
        resizedWindow();
    }, 100);
};

// Run when the window is finished being resized.
function resizedWindow() {
    accountListCenterCheck();
}




function initEditor() {
    const editor = ace.edit("editor");
    editor.session.setMode("ace/mode/batchfile");
}


// --------- FROM NEW SYSTEM ----------

$(function () {
    /*
     * Prevents default browser navigation (Often causes breaks in code by somehow keeping state)
     * Can't seem to do this via the WebView2 component directly as key presses just don't reach the app...
     * So, instead the mouse back button is handled here.
     * Don't know where I could handle a keyboard back button... Because I can't find a JS key for it.
     *
     * tldr: pressing mouse back, or keyboard back often somewhat reliably causes the error in-app, and should be handled differently.
     */

    // 2022-07-17: This seems to be something that can ONLY be handled with JS - and not Blazor...
    $(document).bind("mouseup", (e) => {
        if (e.which === 4 || e.which === 5) { // Backward & Forward mouse button
            e.preventDefault();
        }
    });
});