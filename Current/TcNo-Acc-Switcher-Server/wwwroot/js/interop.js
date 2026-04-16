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

$(function () {
    /*
     * Prevents default browser navigation (Often causes breaks in code by somehow keeping state)
     * Can't seem to do this via the WebView2 component directly as key presses just don't reach the app...
     * So, instead the mouse back button is handled here.
     * Don't know where I could handle a keyboard back button... Because I can't find a JS key for it.
     *
     * tldr: pressing mouse back, or keyboard back often somewhat reliably causes the error in-app, and should be handled differently.
     */
    $(document).bind("mouseup", (e) => {
        if (e.which === 4 || e.which === 5) { // Backward & Forward mouse button
            e.preventDefault();
        }

        if (e.which === 4) { // Backward mouse button
            btnBack_Click();
            return;
        }
    });
});

function jQueryAppend(jQuerySelector, strToInsert) {
	$(jQuerySelector).append(strToInsert);
}

function jQueryProcessAccListSize() {
    let maxHeight = 0;
    $(".acc_list_item label").each((_, e) => { maxHeight = Math.max(maxHeight, e.offsetHeight); });
    if (document.getElementById("acc_list"))
	    document.getElementById("acc_list").setAttribute("style", `grid-template-rows: repeat(auto-fill, ${maxHeight}px)`);
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

function updateStatus(status) {
    $("#CurrentStatus").val(status);
}

async function initPlatformListSortable() {
    // Create sortable list
    sortable(".platform_list", {
        forcePlaceholderSize: true,
        placeholderClass: "placeHolderPlat",
        hoverClass: "accountHover",
        items: ":not(toastarea)"
    });

    // On drag end, save list of items.
    sortable(".platform_list")[0].addEventListener("sortupdate", () => {
        var order = [];
        $(".platform_list > div").each((i, e) => { order.push(e.id); });

        DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiSaveOrder", `Settings\\platformOrder.json`, JSON.stringify(order));
    });
}

async function initAccListSortable() {
	if (document.getElementsByClassName("acc_list").length === 0) return;
    // Create sortable list
    sortable(".acc_list", {
        forcePlaceholderSize: true,
        placeholderClass: "placeHolderAcc",
        hoverClass: "accountHover",
        items: ":not(toastarea)"
    });
    // On drag start, un-select all items.
    sortable(".acc_list")[0].addEventListener("sortstart", () => {
        $("input:checked").each((_, e) => {
            $(e).prop("checked", false);
        });
    });

    var platformName = getCurrentPage();
    await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiCurrentBasicPlatform", platformName).then((r) => {
        platformName = r;
    });

    // On drag end, save list of items.
    sortable(".acc_list")[0].addEventListener("sortupdate", (e) => {
        const order = [];
        e.detail.destination.items.forEach((i) => {
            if (!$(i).is("div")) return; // Ignore <toastarea class="toastarea" />
            order.push(i.getElementsByTagName("input")[0].getAttribute("id"));
        });

        DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiSaveOrder", `LoginCache\\${platformName}\\order.json`, JSON.stringify(order));
    });
}

function steamAdvancedClearingAddLine(text) {
    queuedJQueryAppend("#lines", `<p>${text}</p>`);
}


function initEditor() {
    const editor = ace.edit("editor");
    editor.session.setMode("ace/mode/batchfile");
}

async function showUpdateBar() {
    $(".programMain").prepend(`<div class="updateBar"><span>${await GetLang("Update")}</span><i class="fas fa-times-circle" id="closeUpdateBar"></i></div>`);
    $(document).on("click", ".updateBar", function (event) {
        $(".updateBar").fadeOut();
        if (event.target.id !== "closeUpdateBar") {
            updateBarClick();
        }
    });
}