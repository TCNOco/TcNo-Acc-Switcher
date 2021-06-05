jQueryAppend = (jQuerySelector, strToInsert) => {
    $(jQuerySelector).append(strToInsert);
};
jQueryProcessAccListSize = () => {
    let maxHeight = 0;
    $(".acc_list_item label").each((_, e) => { maxHeight = Math.max(maxHeight, e.offsetHeight); });
    document.getElementById("acc_list").setAttribute("style", `grid-template-rows: repeat(auto-fill, ${maxHeight}px)`);
};

updateStatus = (status) => {
    $("#CurrentStatus").val(status);
};
initAccListSortable = () => {
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
    // On drag end, save list of items.
    sortable(".acc_list")[0].addEventListener("sortupdate", (e) => {
        let order = [];
        e.detail.destination.items.forEach((e) => {
            if (!$(e).is("div")) return; // Ignore <toastarea>
            order.push(e.getElementsByTagName("input")[0].getAttribute("id"));
        });
        DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiSaveOrder", `LoginCache\\${getCurrentPage()}\\order.json`, JSON.stringify(order));
    });
};

steamAdvancedClearingAddLine = (text) => {
    queuedJQueryAppend("#lines", "<p>" + text + "</p>");
};