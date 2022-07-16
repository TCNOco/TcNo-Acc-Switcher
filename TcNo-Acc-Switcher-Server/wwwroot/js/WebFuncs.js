var currentVersion = "";

// Returns "Steam" or "Riot" for example, based on the current URL
function getCurrentPage() {
    return (window.location.pathname.split("/")[0] !== "" ?
        window.location.pathname.split("/")[0] :
        window.location.pathname.split("/")[1]);
}

async function getCurrentPageFullname() {
    // If a name for text is required, rather than code (Steam instead of steam or basic)
    var platform = getCurrentPage();
    if (platform === "Basic") {
        platform = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiCurrentBasicPlatform", platform);
    }
    return platform;
}


// Clear Cache reload:
var winUrl = window.location.href.split("?");
if (winUrl.length > 1 && winUrl[1].indexOf("cacheReload") !== -1) {
    history.pushState({}, null, window.location.href.replace("cacheReload&", "").replace("cacheReload", ""));
    location.reload(true);
}

GetLang = async(k) => await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiLocale", k);
GetLangSub = async(key, obj) => await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiLocaleObj", key, obj);


// Take a string that is HTML escaped, and return a normal string back.
unEscapeString = (s) => s.replace("&lt;", "<").replace("&gt;", ">").replace("&#34;", "\"").replace("&#39;", "'").replace("&#47;", "/");


// General function: Get selected account
var selected;
function getSelected() {
	// This may be unnecessary.
	selected = $(".acc:checked");
	if (selected === "" || selected[0] === null || typeof selected[0] === "undefined") {
		return false;
	}
	return true;
}

// Swapping accounts
async function changeImage(e) {
    if (e !== undefined && e !== null) e.preventDefault();
    if (!getSelected()) return;

    const path = $(".acc:checked").next("label").children("img")[0].getAttribute("src").split("?")[0];

    const modalTitleBackground = await GetLang("Modal_Title_Userdata"),
        modalHeading = await GetLang("Modal_SetImageHeader"),
        modalSetButton = await GetLang("Modal_SetImage");

    $("#modalTitle").text(modalTitleBackground);
    $("#modal_contents").empty();
    $("#modal_contents").append(`<div>
		        <p class="modal-text">${modalHeading}</p>
	        </div>
	        <div class="inputAndButton">
		        <input type="text" id="FolderLocation" autocomplete="off" style="width: 100%;padding: 8px;" onkeydown="javascript: if(event.keyCode == 13) document.getElementById('set_background').click();"'>
	        </div>
	        <div class="settingsCol inputAndButton">
                <button class="modalOK" type="button" id="set_background" onclick="Modal_FinalizeImage('${encodeURI(path)}')"><span>${modalSetButton}</span></button>
	        </div>
            <div class="pathPicker">
                ${await getLogicalDrives()}
            </div>`);

    pathPickerRequestedFile = "AnyFile";
    const input = document.getElementById("FolderLocation");
    $(".pathPicker").on("click", pathPickerClick);
    $(".modalBG").fadeIn(() => {
        try {
            if (input === undefined) return;
            input.focus();
            input.select();
        }
        catch (err) {

        }
    });
}

// Open Game Stats menu 1: Enable/Disable stats for specific games
async function ShowGameStatsSetup(e = null) {
    if (e !== undefined && e !== null) e.preventDefault();
    if (!getSelected()) return;

    const accountId = selected.attr("id");

    const modalHeading = await GetLangSub("Modal_GameStats_Header", { accountName: getDisplayName() }),
        modalTitle= await GetLang("Modal_Title_GameStats"),
        edit = await GetLang("Edit"),
        refresh = await GetLang("Refresh");

    const currentPage = await getCurrentPageFullname();
    const safeGameNames = [];

    const enabledGames =
        await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GetEnabledGames", currentPage, accountId);
    const disabledGames = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GetDisabledGames", currentPage, accountId);

    $("#modalTitle").text(modalTitle);
    $("#modal_contents").empty();
    let html = "";
    html += `<div class="gameStatsWindow">
		        <p>${modalHeading}</p>
                <div class="modalScrollSection">`;

    for (const x in enabledGames) {
        const game = enabledGames[x];
        const safeGame = game.replace(/[/\\?%*:|"<>\s]/g, "");
        html += `<div class="rowSetting">
                    <div class="form-check mb-2">
                        <input class="form-check-input" type="checkbox" id="${safeGame}" checked><label class="form-check-label" for="${safeGame}"></label><label for="${safeGame}">${game}<br></label></div>
                    <div>
                        <button type="button" onclick="showGameStatsVars('${game}')"><span>${edit}</span></button>
                        <button type="button" onclick="refreshAccount('${game}', '${accountId}')"><span>${refresh}</span></button>
                    </div>
                </div>`;
        safeGameNames.push(safeGame);
    }
    for (const x in disabledGames) {
        const game = disabledGames[x];
        const safeGame = game.replace(/[/\\?%*:|"<>\s]/g, "");
        html += `   <div class="form-check mb-2">
                        <input class="form-check-input" type="checkbox" id="${safeGame}"><label class="form-check-label" for="${safeGame}"></label><label for="${safeGame}">${game}<br></label>
                    </div>`;
        safeGameNames.push(safeGame);
    }

                //@foreach (var item in AData.EnabledPlatformSorted())
                //{
                //
                //}
    html += "</div></div></div>";
    $("#modal_contents").append(html);

    for (const x in safeGameNames) {
        const game = safeGameNames[x];
        $(`#${game}`).change(async function () {
            await toggleGameStats(game, this.checked);
        });
    }

    $(".modalBG").fadeIn(() => {
        try {
            if (input === undefined) return;
            input.focus();
            input.select();
        }
        catch (err) {

        }
    });
    // Later:
    // On enabling game, required variables are collected from user, and game stats activated for that game.
    // User stats also collected and displayed.
    // On disabling game, stats are cleared for said game.
}

async function toggleGameStats(safeGame, isChecked) {
    if (!getSelected()) return;
    const game = $(`label[for='${safeGame}']:last`).text();
    const accountId = selected.attr("id");
    console.log(game, isChecked);

    if (!isChecked) {
        // Unchecked: Remove entry and continue.
        await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `DisableGame`, game, accountId);
        ShowGameStatsSetup();
        return;
    }

    // Checked: Get required variables and present to user.
    showGameStatsVars(game);
}

// Open the variable setting menu for game stats for account.
// This is the Manage button, when variables are already set.
async function showGameStatsVars(game) {
    if (!getSelected()) return;
    const accountId = selected.attr("id");

    // Checked: Get required variables and present to user.
    const required = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `GetRequiredVars`, game);
    const existing = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `GetExistingVars`, game, accountId);
    const hidden = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `GetHiddenMetrics`, game, accountId);
    const globallyHidden = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `GetGloballyHiddenMetrics`, game);
    showGameVarCollectionModel(game, required, existing, hidden, globallyHidden);
}

async function showGameVarCollectionModel(game, requiredVars, existingVars = {}, hidden = {}, globallyHidden = []) {
    if (!getSelected()) return;
    const accountId = selected.attr("id");
    const currentPage = await getCurrentPageFullname();

    const modalTitle = await GetLangSub("Modal_Title_GameVars", { game: game }),
        modalHeading = await GetLangSub("Modal_GameVars_Header", { game: game, username: getDisplayName(), platform: currentPage }),
        submit = await GetLang("Submit"),
        disabledGlobally = await GetLang("Tooltip_DisabledGlobally"),
        metricsToShow = await GetLang("Stats_MetricsToShow");


    $("#modalTitle").text(modalTitle);
    $("#modal_contents").empty();
    let html = "";
    html += `<div class="gameStatsWindow">
                <p>${modalHeading}</p>
                <div class="modalScrollSection centeredContainer">
                    <div class="centeredSection">`;

    let checkboxesMarkup = "";

    for (let [key, value] of Object.entries(requiredVars)) {
        console.log(key, value);
        let placeholder = "";
        if (value.includes("[") && value.includes("]")) {
            const parts = value.split("[");
            value = parts[0].trim();
            placeholder = parts[1].trim().replace("]", "");
        }

        let existingValue = key in existingVars ? existingVars[key] : "";
        if (value === "%ACCOUNTID%") {
            value = await GetLang("Stats_AccountId");
            if (existingValue === "")
                existingValue = accountId;
        }

        html +=
            `<div class="rowSetting"><span>${value}</span><input type="text" id="acc${key}" spellcheck="false" placeholder="${placeholder}" value="${existingValue}"></div>`;
    }

    for (let [key, value] of Object.entries(hidden)) {
        const metricHidden = value["item1"], checkboxText = value["item2"];
        let disabled = false;
        if (globallyHidden.includes(key)) disabled = true;
        checkboxesMarkup += `<div class="form-check mb-2" ${disabled ? "data-toggle=\"tooltip\" title=\"" + disabledGlobally + "\"" : ""}><input class="form-check-input" type="checkbox" id="${key}" ${(!metricHidden ? "checked" : "")} ${disabled ? "disabled" : ""}><label class="form-check-label" for="${key}"></label><label for="${key}">${checkboxText}<br></label></div>`;
    }

    html += `       </div>
                </div>
                <div>
                    <p>${metricsToShow}</p>
                    ${checkboxesMarkup}
                </div>
                <div class="settingsCol inputAndButton">
                    <button class="modalOK" type="button" id="set_password" onclick="Modal_FinaliseGameVars('${game}', '${accountId}')"><span>${submit}</span></button>
                </div>
            </div>`;
    $("#modal_contents").append(html);
    $("#modalBtnBack").show();
    $("#modalBtnBack").on("click",
        () => {
            $("#modalBtnBack").hide();
            ShowGameStatsSetup();
        });

    $(".modalBG").fadeIn(() => {
        try {
            initTooltips();
            if (input === undefined) return;
            input.focus();
            input.select();
        }
        catch (err) {

        }
    });
}

async function Modal_FinaliseGameVars(game, accountId) {
    console.log(game, accountId);
    const currentPage = await getCurrentPageFullname();

    // Get list of variable keys
    const requiredVars = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `GetRequiredVars`, game);
    const possibleHidden = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `GetHiddenMetrics`, game, accountId);
    const globallyHidden = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `GetGloballyHiddenMetrics`, game);
    console.log(requiredVars, typeof (requiredVars));

    // Get value for each key and create dictionary
    const returnDict = {};
    for (const [key, value] of Object.entries(requiredVars)) {
        console.log(key, value, $(`#acc${key}`).val());
        returnDict[key] = $(`#acc${key}`).val();
    }

    // Get list of hidden metrics
    const hidden = [];
    for (const [key, _] of Object.entries(possibleHidden)) {
        const checkbox = $(`#${key}`);
        if (checkbox.is(":not(:checked)")) {
            hidden.push(key);
        }
    }

    // Add user statistics for game, with collected variables
    $(".modalBG").fadeOut();
    const success = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `SetGameVars`, currentPage, game, accountId, returnDict, hidden);
    if (success) location.reload();
}

async function refreshAccount(game, accountId) {
    const currentPage = await getCurrentPageFullname();
    $(".modalBG").fadeOut();
    await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `RefreshAccount`, accountId, game, currentPage);
    location.reload();
}

Modal_FinalizeImage = async(dest) => await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `ImportNewImage`, JSON.stringify({ dest: dest, path: $("#FolderLocation").val() }));

hidePlatform = async() => await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "HidePlatform", selectedElem);
createPlatformShortcut = async() => await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiCreatePlatformShortcut", selectedElem);

var exportingAccounts = false;

async function exportAllAccounts() {
    if (exportingAccounts) {
	    const toastAlreadyProcessing = await GetLang("Toast_AlreadyProcessing"),
		    error = await GetLang("Error");

	    window.notification.new({
		    type: "error",
            title: error,
            message: toastAlreadyProcessing,
		    renderTo: "toastarea",
            duration: 5000
	    });
        return;
    }
    exportingAccounts = true;
    const r = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiExportAccountList", selectedElem);
    const filename = r.split("/");
    saveFile(filename[filename.length - 1], r);
    exportingAccounts = false;
}

function saveFile(fileName, urlFile) {
	const a = document.createElement("a");
	a.style = "display: none";
	document.body.appendChild(a);
	a.href = urlFile;
	a.download = fileName;
	a.click();
	a.remove();
}

// Link handling
OpenLinkInBrowser = async(link) => await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "OpenLinkInBrowser", link);

let pathPickerRequestedFile = "";

// Info Window
async function showModalOld(modaltype) {
    let input, platform;
    if (modaltype.startsWith("setAppPassword")) {
        // USAGE: "changeUsername"
        const modalChangeUsername = await GetLang("Modal_SetPassword"),
            modalSetPasswordButton = await GetLang("Modal_SetPassword_Button"),
            modalTitleSetPassword = await GetLang("Modal_Title_SetPassword"),
            modalSetPasswordInfo = await GetLangSub("Modal_SetPassword_Info", { link: "https://github.com/TcNobo/TcNo-Acc-Switcher/wiki/FAQ---More-Info#can-i-put-this-program-on-a-usb-portable" });

        $("#modalTitle").text(modalTitleSetPassword);
        $("#modal_contents").empty();

        $("#modal_contents").append(`<h3>${modalChangeUsername}</h3>
	        <div class="inputAndButton">
		        <input type="text" id="SwitcherPassword" style="width: 100%;padding: 8px;" autocomplete="off" onkeydown="javascript: if(event.keyCode == 13) document.getElementById('set_password').click();">
	        </div>
            <div>
                <span class="modal-text">${modalSetPasswordInfo}</span>
            </div>
	        <div class="settingsCol inputAndButton">
		        <button class="modalOK" type="button" id="set_password" onclick="Modal_FinaliseSwitcherPassword()"><span>${
            modalSetPasswordButton}</span></button>
	        </div>`);
        input = document.getElementById("NewAccountName");
    } else if (modaltype === "accString") {
        platform = getCurrentPage();
        const extraButtons = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "PlatformUserModalExtraButtons");

        Modal_RequestedLocated(false);
        // Sub in info if this is a basic page
        var redirectLink = platform;
        platform = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiCurrentBasicPlatform", platform);

        const modalTitleAddNew = await GetLangSub("Modal_Title_AddNew", { platform: platform }),
            modalAddNew = await GetLangSub("Modal_AddNew", { platform: platform }),
            modalAddCurrentAccount = await GetLangSub("Modal_AddCurrentAccount", { platform: platform });

        $("#modalTitle").text(modalTitleAddNew);
        $("#modal_contents").empty();
        $("#modal_contents").append(`<div>
		        <span class="modal-text">${modalAddNew}</span>
	        </div>
	        <div class="inputAndButton">
		        <input type="text" id="CurrentAccountName" autocomplete="off" style="width: 100%;padding: 8px;" onkeydown="javascript: if(event.keyCode == 13) document.getElementById('set_account_name').click();" onkeypress='return /[^<>:\\.\\"\\/\\\\|?*]/i.test(event.key)'>
	        </div>
	        <div class="settingsCol inputAndButton">
				${extraButtons}
		        <button class="modalOK" type="button" id="set_account_name" onclick="Modal_FinaliseAccString('${
            redirectLink}')"><span>${modalAddCurrentAccount}</span></button>
	        </div>`);
        input = document.getElementById("CurrentAccountName");
    } else if (modaltype === "SetBackground") {
        const modalTitleBackground = await GetLang("Modal_Title_Background"),
            modalHeading = await GetLang("Modal_SetBackground"),
            modalSetButton = await GetLang("Modal_SetBackground_Button");

        $("#modalTitle").text(modalTitleBackground);
        $("#modal_contents").empty();
        $("#modal_contents").append(`<div>
		        <p class="modal-text">${modalHeading}</p>
	        </div>
	        <div class="inputAndButton">
		        <input type="text" id="FolderLocation" oninput="updateIndicator('')" autocomplete="off" style="width: 100%;padding: 8px;" onkeydown="javascript: if(event.keyCode == 13) document.getElementById('set_background').click();"'>
	        </div>
	        <div class="settingsCol inputAndButton">
		        <button class="modalOK" type="button" id="set_background" onclick="Modal_FinaliseBackground()"><span>${
            modalSetButton}</span></button>
	        </div>
            <div class="pathPicker">
                ${await getLogicalDrives()}
            </div>`);

        pathPickerRequestedFile = "AnyFile";
        $(".pathPicker").on("click", pathPickerClick);
        input = document.getElementById("FolderLocation");
    } else if (modaltype === "SetUserdata") {
        const modalTitleBackground = await GetLang("Modal_Title_Userdata"),
            modalHeading = await GetLang("Modal_SetUserdata"),
            modalSetButton = await GetLang("Modal_SetUserdata_Button");

        $("#modalTitle").text(modalTitleBackground);
        $("#modal_contents").empty();
        $("#modal_contents").append(`<div>
		        <p class="modal-text">${modalHeading}</p>
	        </div>
	        <div class="inputAndButton">
		        <input type="text" id="FolderLocation" oninput="updateIndicator('')" autocomplete="off" style="width: 100%;padding: 8px;" onkeydown="javascript: if(event.keyCode == 13) document.getElementById('set_background').click();"'>
	        </div>
	        <div class="settingsCol inputAndButton">
                <button class="modalOK" type="button" id="set_background" onclick="Modal_FinaliseUserDataFolder()"><span>${
            modalSetButton}</span></button>
	        </div>
            <div class="pathPicker">
                ${await getLogicalDrives()}
            </div>`);

        pathPickerRequestedFile = "AnyFolder";
        $(".pathPicker").on("click", pathPickerClick);
        input = document.getElementById("FolderLocation");
    } else {

        const notice = await GetLang("Notice");

        $("#modalTitle").text(notice);
        $("#modal_contents").empty();
        $("#modal_contents").append(`<div class="infoWindow"><div class="fullWidthContent">${modaltype}</div></div>`);
    }
    $(".modalBG").fadeIn(() => {
        try {
            if (input === undefined) return;
            input.focus();
            if (input.nodeName !== "TEXTAREA") input.select();
        }
        catch (err) {

        }
    });
}










async function getLogicalDrives() {
    var folderContent = "";
    const logicalDrives = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GetLogicalDrives");
    folderContent = "<div>";
    logicalDrives.Folders.forEach((f) => { folderContent += "<span class=\"folder\" path=\"" + f + "\">" + f + "</span>"; });
    folderContent += "</div>";

    return folderContent;
}

function updateIndicator(e) {
    console.log("Update Indicator REQUESTED");
    // Update Found/Not Found preview
    var foundRequested;
    if (pathPickerRequestedFile === "AnyFolder" && e !== "" && $(e.target).hasClass("folder")) {
        foundRequested = true;
    } else if (pathPickerRequestedFile === "AnyFile" && e !== "" && !$(e.target).hasClass("folder")) {
        foundRequested = true;
    } else {
        // Everything else
        pathPickerRequestedFile = pathPickerRequestedFile.replace("*", "");
        foundRequested = $("#FolderLocation").val().toLowerCase().includes(pathPickerRequestedFile.toLowerCase());
    }
    Modal_RequestedLocated(foundRequested);
}
async function pathPickerClick(e) {
    const result = $(e.target).attr("path");
    if (result === undefined) return;
    //console.log(result);
    $("#FolderLocation").val(result);
    updateIndicator(e); // Because the above doesn't trigger the event
    const currentSpanPath = $(e.target).attr("path");
    var folderContent = "";

    // Because this is reset: see if has .exe inside it.
    if ($(e.target).hasClass("folder") && pathPickerRequestedFile.endsWith(".exe")) {
        //console.log($(e.target).parent().html());
        Modal_RequestedLocated($(e.target).parent().html().toLowerCase().includes(pathPickerRequestedFile.toLowerCase()));
    }
    // If is not currently open: Continue
    if ($(e.target).hasClass("c")) return;

    $(".pathPicker .c").each((_, s) => {
        var path = $(s).attr("path");
        if (!result.includes(path)) {
            $(s).parent().replaceWith("<span class=\"folder\" path=\"" + path + "\">" + (path.at(-1) !== "\\" ? path.split("\\").at(-1) : path) + "</span>");
        }
    });

    // Reset all selected-path highlights.
    $(".pathPicker .selected-path").removeClass("selected-path");
    $(e.target).addClass("selected-path");

    // Expand folder
    if ($(e.target).hasClass("folder") && !$(e.target).hasClass("c")) {
        let getFunc = "GetFoldersAndFiles";
        if (pathPickerRequestedFile === "AnyFolder") getFunc = "GetFolders";

        const fileSystemResult = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", getFunc, result);
        folderContent = "<div path=\"" + currentSpanPath + "\"><span class=\"folder c head selected-path\" path=\"" + currentSpanPath + "\">" + (currentSpanPath.at(-1) !== "\\" ? currentSpanPath.split("\\").at(-1) : currentSpanPath) + "</span>";
        fileSystemResult.Folders.forEach((f) => {
            folderContent += "<span class=\"folder\" path=\"" + f + "\">" + (f.at(-1) !== "\\" ? f.split("\\").at(-1) : f) + "</span>";
        });
        fileSystemResult.Files.forEach((f) => {
            folderContent += "<span " + (f.includes(pathPickerRequestedFile) ? "class=\"suggested\" " : "") + "path=\"" + f + "\">" + (f.at(-1) !== "\\" ? f.split("\\").at(-1) : f) + "</span>";
        });
        folderContent += "</div>";

        //console.log(folderContent);
        $(e.target).replaceWith(folderContent);

        // After expanding, see if has .exe inside it.
        if ($(e.target).hasClass("folder") && pathPickerRequestedFile.endsWith(".exe")) {
            Modal_RequestedLocated(folderContent.toLowerCase().includes(pathPickerRequestedFile.toLowerCase()));
        }
    }
}


// For finding files with modal:
function Modal_RequestedLocated(found) {
    try {
        $(".folder_indicator").removeClass("notfound found");
        $(".folder_indicator_bg").removeClass("notfound found");
        if (found === true) {
            $(".folder_indicator").addClass("found");
            $(".folder_indicator_bg").addClass("found");
        } else {
            $(".folder_indicator").addClass("notfound");
            $(".folder_indicator_bg").addClass("notfound");
        }
    } catch (_) {

    }
}

//function Modal_Finalise(platform, platformSettingsPath) {
//    DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiUpdatePath", platformSettingsPath, $("#FolderLocation").val());
//    $(".modalBG").fadeOut();
//    window.location.assign(platformSettingsPath.split("Settings")[0]);
//}

async function Modal_Finalise(platform, platformSettingsPath) {
    if (window.location.href.includes("PreviewCss")) {
        // Do nothing for CSS preview page.
        return;
    }

    await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiUpdatePath", platformSettingsPath, $("#FolderLocation").val());
    $(".modalBG").fadeOut();

    location.reload();
}

async function Modal_FinaliseAccString(platform) {
    if (window.location.href.includes("PreviewCss")) {
        // Do nothing for CSS preview page.
        return;
    }

    // Supported: BASIC
    const raw = $("#CurrentAccountName").val();
    let name = raw;

    // Clean string if not a command string.
    if (raw.indexOf(":{") === -1) {
        name = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiGetCleanFilePath", raw);
    }

    await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", platform + "AddCurrent", name);
    $(".modalBG").fadeOut();
    $("#acc_list").click();
}

async function Modal_FinaliseBackground() {
    const pathOrUrl = $("#FolderLocation").val();
    await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "SetBackground", pathOrUrl);
    $(".modalBG").fadeOut();
}

async function Modal_FinaliseSwitcherPassword() {
    const switcherPassword = $("#SwitcherPassword").val();
    await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "SetSwitcherPassword", switcherPassword);
    $(".modalBG").fadeOut();
}

async function Modal_FinaliseUserDataFolder() {
    if (window.location.href.includes("PreviewCss")) {
        // Do nothing for CSS preview page.
        return;
    }

    const pathOrUrl = $("#FolderLocation").val();
    await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "SetUserData", pathOrUrl);
    $(".modalBG").fadeOut();
}

async function Modal_SaveNotes(accId) {
    await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", `Set${getCurrentPage()}Notes`, accId, $("#accNotes").val());
    location.reload(true);
}

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

async function usernameModalCopyText() {
    const toastTitle = await GetLang("Toast_Copied");
    const platform = getCurrentPage();

    const toastHintText = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "PlatformHintText", platform);
    const code = await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "PlatformUserModalCopyText");

    await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "CopyText", code);
    window.notification.new({
	    type: "success",
        title: toastTitle,
        message: toastHintText,
	    renderTo: "toastarea",
	    duration: 5000
    });
}


// Basic account switcher: Shortcut dropdown
var sDropdownInitialized = false;
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

function sDropdownInit() {
    if (sDropdownInitialized) return;
    sDropdownInitialized = true;
    // Create sortable list
    sortable(".shortcuts, .shortcutDropdownItems", {
        connectWith: "shortcutJoined",
        forcePlaceholderSize: true,
        placeholderClass: "shortcutPlaceholder",
        items: ":not(#btnOpenShortcutFolder)"
    });

    $(".shortcuts, .shortcutDropdownItems").toArray().forEach(el => {
// ReSharper disable once Html.EventNotResolved
        el.addEventListener("sortstart", function () {
            $(".shortcuts").addClass("expandShortcuts");
        });
// ReSharper disable once Html.EventNotResolved
        el.addEventListener("sortstop", function () {
            $(".shortcuts").removeClass("expandShortcuts");
            sDropdownReposition();
            serializeShortcuts();
        });
    });
}

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
        $("#shortcutDropdown").hide();
        $("#shortcutDropdownBtn").removeClass("flip");
        $(".shortcuts").removeClass("expandShortcuts");
    }
}

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

    if (getCurrentPage() === "Steam")
        await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "SaveShortcutOrderSteam", output);
    else
        await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "SaveShortcutOrder", output);
}

// Context menu buttons
async function shortcut(action) {
    const reqId = $(selectedElem).prop("id");
    console.log(reqId);
    if (getCurrentPage() === "Steam")
        await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "HandleShortcutActionSteam", reqId, action);
    else
        await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "HandleShortcutAction", reqId, action);
    if (action === "hide") $(selectedElem).remove();
}

updateBarClick = async () => await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "UpdateNow");

async function initSavingHotKey() {
    hotkeys("ctrl+s", async function (event) {
        event.preventDefault();
        await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "GiCtrlS", getCurrentPage());
    });
}

getDisplayName = () => $(selectedElem).siblings("label").find(".displayName").text();

async function initCopyHotKey() {
    const toastCopied = await GetLang("Toast_Copied");
    hotkeys("ctrl+c,ctrl+shift+c,alt+c", async function (event, handler) {
        // Doesn't prevent default!
        switch (handler.key) {
        case "ctrl+shift+c":
        case "alt+c":
            await copyToClipboard($(selectedElem).prop("id"));
            break;
        case "ctrl+c":
                await copyToClipboard(getDisplayName());
            break;
        }

        window.notification.new({
            type: "info",
            title: "",
            message: toastCopied,
            renderTo: "toastarea",
            duration: 2000
        });
    });
}












// ---------- KEEPING FROM OLD ----------

copyToClipboard = async (str) => await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "CopyToClipboard", str);

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

// Show and Hide modal popup window -- This allows animations.
showModal = async () => $(".modalBG").fadeIn();
hideModal = async () => $(".modalBG").fadeOut();

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
