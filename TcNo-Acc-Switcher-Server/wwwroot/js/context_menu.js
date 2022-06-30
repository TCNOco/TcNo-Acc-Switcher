// Selected Element on list, for use in other JS functions
var selectedElem = "";

// Returns "Steam" or "Riot" for example, based on the current URL
function getCurrentPage() {
    return (window.location.pathname.split("/")[0] !== "" ?
        window.location.pathname.split("/")[0] :
        window.location.pathname.split("/")[1]);
}

async function initContextMenu() {
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
            selectedElem = $(e.currentTarget).children("input")[0];
        });
    }

    const selectedText = await GetLangSub("Status_SelectedAccount", { name: "XXX" });
    // Show shortcut contextmenu on Right-Click
    $(`.HasContextMenu`).contextmenu((e) => {
        // Set currently selected element
        selectedElem = $(e.currentTarget);

        //Get window size:
        const winWidth = $(document).width();
        const winHeight = $(document).height();
        //Get pointer position:
        const posX = e.pageX - 14;
        const posY = e.pageY - 42; // Offset for header bar
        //Get contextmenu size:
        const menuWidth = $("#Shortcuts").width();
        const menuHeight = $("#Shortcuts").height();

        // Header offset + 10:
        const hOffset = 42;
        // Prevent page overflow:
        const xOverflow = posX + menuWidth + hOffset - winWidth;
        const yOverflow = posY + menuHeight + hOffset - winHeight;
        var posLeft = posX + (xOverflow > 0 ? - menuWidth : 10) + "px";
        var posTop = posY + (yOverflow > 0 ? - yOverflow : 10) + "px";

        //Display contextmenu:
        $("#Shortcuts").css({
            "left": posLeft,
            "top": posTop
        }).show();
        //Prevent browser default contextmenu.
        return false;
    });

    //Show contextmenu on Right-Click:
    $(`.${group}_list_item`).contextmenu((e) => {
        if (group === "acc") {
            // Select item that was right-clicked.
            $(e.currentTarget).children("input").click();

            // Set currently selected element
            selectedElem = $(e.currentTarget).children("input")[0];

            // Update status for element
            let statusText = "";
            switch (getCurrentPage()) {
                case "Steam":
	                statusText = $(selectedElem).attr("Line2");
                break;
            default:
                break;
            }
            updateStatus(selectedText.replace("XXX", statusText));

        } else if (group === "platform") {
            // Set currently selected element
            selectedElem = $(e.currentTarget).prop("id");
        }

        //Get window size:
        const winWidth = $(document).width();
        const winHeight = $(document).height();
        //Get pointer position:
        const posX = e.pageX - 14;
        const posY = e.pageY - 42; // Offset for header bar
        //Get contextmenu size:
        const menuWidth = $("#AccOrPlatList").width();
        const menuHeight = $("#AccOrPlatList").height();

        // Header offset + 10:
        const hOffset = 42;
        // Prevent page overflow:
        const xOverflow = posX + menuWidth + hOffset - winWidth;
        const yOverflow = posY + menuHeight + hOffset - winHeight;
        var posLeft = posX + (xOverflow > 0 ? - menuWidth : 10) + "px";
        var posTop = posY + (yOverflow > 0 ? - yOverflow : 10) + "px";

        //Display contextmenu:
        $("#AccOrPlatList").css({
            "left": posLeft,
            "top": posTop
        }).show();
        //Prevent browser default contextmenu.
        return false;
    });

    //Show contextmenu on Right-Click (platform shortcut):
    $(`#btnStartPlat`).contextmenu((e) => {
        if (group === "acc") {
            // Select item that was right-clicked.
            $(e.currentTarget).children("input").click();

            // Set currently selected element
            selectedElem = $(e.currentTarget);

            // Update status for element
            let statusText = "";
            switch (getCurrentPage()) {
                case "Steam":
	                statusText = $(selectedElem).attr("Line2");
                break;
            default:
                break;
            }
            updateStatus(selectedText.replace("XXX", statusText));

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
        const menuWidth = $("#Platform").width();
        const menuHeight = $("#Platform").height();

        // Header offset + 10:
        const hOffset = 42;
        // Prevent page overflow:
        const xOverflow = posX + menuWidth + hOffset - winWidth;
        const yOverflow = posY + menuHeight + hOffset - winHeight;
        var posLeft = posX + (xOverflow > 0 ? - menuWidth : 10) + "px";
        var posTop = posY + (yOverflow > 0 ? - yOverflow : 10) + "px";

        //Display contextmenu:
        $("#Platform").css({
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

    function addSearchAndPaging(ele, itemsPerPage) {
        let pageCount = (visibleOnly = true) => {
            const childCount = visibleOnly ? ele.querySelectorAll(':scope > li:not(.filteredItem)').length
                : ele.querySelectorAll(':scope > li').length;
            return Math.ceil( childCount / itemsPerPage);
        };
        let currentPage = 1;
        if (pageCount(false) < 2) return;

        let paginationContainer = document.createElement("div");
        paginationContainer.classList.add("paginationContainer");
        let paginationDiv = document.createElement("div");
        paginationDiv.classList.add("pagination");
        paginationDiv.onclick = ((e) => {
            e.preventDefault();
            e.stopPropagation();
        });

        let pageText = document.createElement("span");
        pageText.innerText = `${currentPage} / ${pageCount()}`;

        let searchInputItem = document.createElement("li");
        searchInputItem.classList.add("contextSearch");
        searchInputItem.innerHTML = "<input type='text' placeholder='Search'></input>";
        searchInputItem.onclick = (e => {
            e.stopPropagation();
        });

        let updateButtons = () => {
            const currentlyVisible = ele.querySelectorAll(':scope > li:not(:first-child):not(.filteredItem)').length;
            if (currentPage === 1 || currentlyVisible <= 6)
                backButton.style.visibility = "hidden";
            else
                backButton.style.visibility = "visible";
            if (currentPage >= pageCount() || currentlyVisible <= 6)
                forwardButton.style.visibility = "hidden";
            else
                forwardButton.style.visibility = "visible";
        };
        let updateVisibleElements = () => {
            ele.querySelectorAll(':scope > li:not(:first-child):not(.filteredItem)').forEach((child, index) => {
                if (Math.ceil((index + 1)/ itemsPerPage) === currentPage) {
                    child.classList.remove('pagedOutItem');
                }
                else {
                    child.classList.add('pagedOutItem');
                }
            });
            const currentlyVisible = ele.querySelectorAll(':scope > li:not(:first-child):not(.filteredItem)').length;
            pageText.innerText = currentlyVisible > 6 ? `${currentPage} / ${pageCount()}` : '';
            updateButtons();
        }
        let navigate = (forward, event) => {
            event.preventDefault();
            event.stopPropagation();
            if (forward) currentPage = Math.min(currentPage + 1, pageCount());
            else currentPage = Math.max(1, currentPage - 1);
            pageText.innerText = `${currentPage} / ${pageCount().toString()}`;
            updateVisibleElements();
        };

        let backButton = document.createElement("a");
        backButton.innerHTML = '<i class="fas fa-arrow-left"></i>';
        backButton.addEventListener("click", navigate.bind(null, false), false);


        let forwardButton = document.createElement("a");
        forwardButton.innerHTML = '<i class="fas fa-arrow-right"></i>';
        forwardButton.addEventListener("click", navigate.bind(null, true), false);

        searchInputItem.querySelector('input').addEventListener('input', (event => {
            currentPage = 1;
            searchInputItem.parentElement.querySelectorAll(':scope > li:not(:first-child) > a').forEach(item => {
                if (item.innerText.toLowerCase().includes(event.target.value.toLowerCase()))
                    item.parentElement.classList.remove('filteredItem');
                else item.parentElement.classList.add('filteredItem');
            });
            updateVisibleElements();
        }));

        ele.prepend(searchInputItem);
        ele.appendChild(paginationContainer);
        paginationContainer.appendChild(paginationDiv);
        paginationDiv.appendChild(backButton);
        paginationDiv.appendChild(pageText);
        paginationDiv.appendChild(forwardButton);
        updateVisibleElements();
    }
    document.querySelectorAll('ul.submenu').forEach(ele => {
        addSearchAndPaging(ele, 6);
    });
};

async function selectedItemChanged() {
    // Different function groups based on platform
    const selectedText = await GetLangSub("Status_SelectedAccount", { name: $("input[name=accounts]:checked").attr("DisplayName") });
    updateStatus(selectedText);
}